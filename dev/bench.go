package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

func init() {
	commands["bench"] = command{"[-n 20] [-spec <file or url>] [-write] [-cpu] [-worker <name>] <url>",
		"time every operation of an API from its OpenAPI spec: wall time, and with -cpu the CPU time Cloudflare measured", bench}
	anywhere["bench"] = true
}

// A call is one operation made callable from the spec's own examples.
type call struct {
	method, path, trigger string
	body                  []byte
}

// bench times an API's operations as a client sees them, and with -cpu also reads what they cost
// on Cloudflare. It works on any API: the operations come from the OpenAPI spec (the one the API
// serves at /api/openapi.json, or -spec), and their inputs from the spec's examples.
//
//   - GET operations whose required inputs have examples are called; streams (text/event-stream)
//     are not. -write adds POST, PUT, PATCH and DELETE: they change data.
//   - Wall time is the median and the slowest of -n requests, after 3 warm-up ones.
//   - CPU time is what Workers bills and limits. It comes from Workers Logs through Cloudflare's
//     API, so it needs CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID (the environment, or fnox)
//     and observability enabled on the Worker; the events take up to a minute or two to arrive.
func bench(args []string) error {
	n, spec, write, cpu, worker := 20, "", false, false, ""
	rest := flags("bench", args, func(f *flag.FlagSet) {
		f.IntVar(&n, "n", n, "requests per operation")
		f.StringVar(&spec, "spec", "", "the OpenAPI spec, a file or a URL (default: <url>/api/openapi.json)")
		f.BoolVar(&write, "write", false, "also call operations that change data (POST, PUT, PATCH, DELETE)")
		f.BoolVar(&cpu, "cpu", false, "also report CPU time from Cloudflare (a deployed Worker)")
		f.StringVar(&worker, "worker", "", "the Worker's name, for -cpu (default: the first label of the URL's host)")
	})
	if len(rest) != 1 || n < 1 {
		return errors.New("bench needs <url>, e.g. https://my-api.example.workers.dev or http://localhost:5174")
	}
	base := strings.TrimSuffix(rest[0], "/")
	if spec == "" {
		spec = base + "/api/openapi.json"
	}
	calls, skipped, err := callsFromSpec(spec, write)
	if err != nil {
		return err
	}
	calls = append(calls, call{method: "GET", path: "/__bench/not-found", trigger: "GET /__bench/not-found"})

	started := time.Now()
	type timing struct {
		status          int
		median, slowest time.Duration
	}
	wall := map[string]timing{}
	fmt.Printf("%s: %d operations, %d requests each\n", base, len(calls), n)
	for _, c := range calls {
		var times []time.Duration
		status := 0
		for i := -3; i < n; i++ {
			req, err := http.NewRequest(c.method, base+c.path, bytes.NewReader(c.body))
			if err != nil {
				return err
			}
			if c.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			start := time.Now()
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			status = res.StatusCode
			if i >= 0 {
				times = append(times, time.Since(start))
			}
		}
		sort.Slice(times, func(a, b int) bool { return times[a] < times[b] })
		wall[c.trigger] = timing{status, times[len(times)/2], times[len(times)-1]}
	}

	var cpuOf map[string][2]float64
	if cpu {
		if worker == "" {
			if u, err := url.Parse(base); err == nil {
				worker, _, _ = strings.Cut(u.Hostname(), ".")
			}
		}
		cpuOf, err = workerCPU(worker, started, len(calls)*n)
		if err != nil {
			fmt.Println("no CPU time:", err)
		}
	}
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	fmt.Printf("\n%-44s %6s %12s %12s", "operation", "status", "wall median", "wall slowest")
	if cpuOf != nil {
		fmt.Printf(" %11s %9s", "CPU median", "CPU p99")
	}
	fmt.Println()
	for _, c := range calls {
		t := wall[c.trigger]
		fmt.Printf("%-44s %6d %9.1f ms %9.1f ms", c.trigger, t.status, ms(t.median), ms(t.slowest))
		if v, ok := cpuOf[c.trigger]; ok {
			fmt.Printf(" %8.1f ms %6.1f ms", v[0], v[1])
		}
		fmt.Println()
	}
	for _, why := range skipped {
		fmt.Println("skipped:", why)
	}
	if cpuOf != nil {
		fmt.Printf("\nCPU time is Cloudflare's, for Worker %q (Workers Free allows 10 ms per request).\n", worker)
	}
	return nil
}

// callsFromSpec makes every operation it can call from the spec's examples, and says which it left out.
func callsFromSpec(source string, write bool) (calls []call, skipped []string, err error) {
	var raw []byte
	if strings.Contains(source, "://") {
		res, err := http.Get(source)
		if err != nil {
			return nil, nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("%s: HTTP %d (pass the spec with -spec <file or url>)", source, res.StatusCode)
		}
		raw, err = io.ReadAll(res.Body)
		if err != nil {
			return nil, nil, err
		}
	} else if raw, err = os.ReadFile(source); err != nil {
		return nil, nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, fmt.Errorf("%s is not an OpenAPI document in JSON: %w", source, err)
	}
	paths, _ := doc["paths"].(map[string]any)
	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		item, _ := paths[name].(map[string]any)
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			trigger := strings.ToUpper(method) + " " + name
			if method != "get" && !write {
				skipped = append(skipped, trigger+": changes data (-write to include it)")
				continue
			}
			if _, stream := dig(op, "responses", "200", "content", "text/event-stream").(map[string]any); stream {
				skipped = append(skipped, trigger+": a stream")
				continue
			}
			path, query, missing := name, url.Values{}, ""
			for _, p := range items(op["parameters"]) {
				param, _ := p.(map[string]any)
				in, pname := fmt.Sprint(param["in"]), fmt.Sprint(param["name"])
				required, _ := param["required"].(bool)
				value := example(doc, param)
				switch {
				case in == "path" && value == nil:
					missing = "path parameter " + pname
				case in == "path":
					path = strings.ReplaceAll(path, "{"+pname+"}", url.PathEscape(fmt.Sprint(value)))
				case in == "query" && required && value == nil:
					missing = "query parameter " + pname
				case in == "query" && required:
					query.Set(pname, fmt.Sprint(value))
				}
			}
			var body []byte
			if schema, ok := dig(op, "requestBody", "content", "application/json", "schema").(map[string]any); ok && missing == "" {
				value := example(doc, map[string]any{"schema": schema})
				if value == nil {
					missing = "request body"
				} else {
					body, _ = json.Marshal(value)
				}
			}
			if missing != "" {
				skipped = append(skipped, trigger+": no example for its "+missing)
				continue
			}
			if len(query) > 0 {
				path += "?" + query.Encode()
			}
			calls = append(calls, call{method: strings.ToUpper(method), path: path, trigger: trigger, body: body})
		}
	}
	if len(calls) == 0 {
		return nil, skipped, fmt.Errorf("%s has no operation that can be called from its examples", source)
	}
	return calls, skipped, nil
}

// example is a value for a parameter or a schema: its own example, or one built from its
// properties' examples (an object's required properties), following $ref.
func example(doc map[string]any, holder map[string]any) any {
	if v, ok := holder["example"]; ok {
		return v
	}
	schema, _ := holder["schema"].(map[string]any)
	for depth := 0; schema != nil && depth < 8; depth++ {
		ref, ok := schema["$ref"].(string)
		if !ok {
			break
		}
		schema, _ = dig(doc, strings.Split(strings.TrimPrefix(ref, "#/"), "/")...).(map[string]any)
	}
	if schema == nil {
		return nil
	}
	if examples := items(schema["examples"]); len(examples) > 0 {
		return examples[0]
	}
	for _, key := range []string{"example", "default"} {
		if v, ok := schema[key]; ok {
			return v
		}
	}
	if values := items(schema["enum"]); len(values) > 0 {
		return values[0]
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		object := map[string]any{}
		for _, name := range items(schema["required"]) {
			property, _ := properties[fmt.Sprint(name)].(map[string]any)
			value := example(doc, map[string]any{"schema": property})
			if value == nil {
				return nil
			}
			object[fmt.Sprint(name)] = value
		}
		return object
	}
	return nil
}

func dig(v any, keys ...string) any {
	for _, key := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[key]
	}
	return v
}

func items(v any) []any { l, _ := v.([]any); return l }

// workerCPU asks Cloudflare (Workers Logs) for the CPU time per operation of a Worker since a
// moment: median and p99, in milliseconds. It waits for the events to arrive, up to two minutes.
func workerCPU(worker string, since time.Time, expect int) (map[string][2]float64, error) {
	token, account := secret("CLOUDFLARE_API_TOKEN"), secret("CLOUDFLARE_ACCOUNT_ID")
	if token == "" || account == "" {
		return nil, errors.New("CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID are not set (in the environment, or in fnox)")
	}
	calculation := func(operator, alias string) map[string]any {
		return map[string]any{"operator": operator, "key": "$workers.cpuTimeMs", "keyType": "number", "alias": alias}
	}
	query := map[string]any{
		"queryId": "dev-bench", "view": "calculations", "ignoreSeries": true, "dry": false,
		"parameters": map[string]any{
			"datasets":     []string{"cloudflare-workers"},
			"filters":      []any{map[string]any{"key": "$metadata.service", "operation": "eq", "type": "string", "value": worker}},
			"calculations": []any{map[string]any{"operator": "count", "alias": "n"}, calculation("median", "p50"), calculation("p99", "p99")},
			"groupBys":     []any{map[string]any{"type": "string", "value": "$metadata.trigger"}},
			"limit":        200,
		},
	}
	fmt.Printf("waiting for Cloudflare's CPU figures for %q (up to two minutes)...\n", worker)
	for deadline := time.Now().Add(2 * time.Minute); ; time.Sleep(10 * time.Second) {
		query["timeframe"] = map[string]int64{"from": since.Add(-2 * time.Second).UnixMilli(), "to": time.Now().UnixMilli()}
		body, _ := json.Marshal(query)
		req, _ := http.NewRequest("POST", "https://api.cloudflare.com/client/v4/accounts/"+account+"/workers/observability/telemetry/query", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		var reply struct {
			Success bool
			Errors  []struct{ Message string }
			Result  struct {
				Calculations []struct {
					Alias      string
					Aggregates []struct {
						GroupKey string
						Value    float64
					}
				}
			}
		}
		err = json.NewDecoder(res.Body).Decode(&reply)
		res.Body.Close()
		if err != nil || !reply.Success {
			return nil, fmt.Errorf("Cloudflare's observability API refused (HTTP %d %v): the token needs to read Workers observability, and the Worker needs observability enabled", res.StatusCode, reply.Errors)
		}
		out, seen := map[string][2]float64{}, 0.0
		for _, c := range reply.Result.Calculations {
			for _, a := range c.Aggregates {
				v := out[a.GroupKey]
				switch c.Alias {
				case "n":
					seen += a.Value
				case "p50":
					v[0] = a.Value
				case "p99":
					v[1] = a.Value
				}
				out[a.GroupKey] = v
			}
		}
		if int(seen) >= expect || time.Now().After(deadline) {
			if len(out) == 0 {
				return nil, fmt.Errorf("no events for Worker %q: is its name right (-worker), and is observability enabled?", worker)
			}
			return out, nil
		}
	}
}

// secret is an environment variable, or the same name from fnox when the environment has none.
func secret(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	out, err := exec.Command("fnox", "get", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
