// Package twins is charter twins <reference> <other>: the same API in two projects (a TypeScript one and its Go twin)
// must answer the same. Both run under workerd, as they are deployed (each built first), each on a free port with
// local state of its own. Both get the same requests, in the same order, and every answer that differs is printed (its
// status, the headers a client reads, its body); it fails if any does. The requests come from the reference's own
// specs, so they need no upkeep: for each operation, the right request and each kind of wrong one (no credentials, too
// little scope, each parameter missing, of the wrong type or out of bounds, a body that is not JSON, of a media type the
// operation does not take, with a property it does not have, a method the path does not have); for each channel, a
// request that is no upgrade; and what every API serves (docs/self/standard.md): /.well-known/, MCP, both specs, an
// unknown path. Then what is saved in each project, which no request shows: the specs in fern/ (the files the SDKs are
// made from) and the database's migrations/.
package twins

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/standard"
	"github.com/joeblew999/charter/project"
)

// Main runs charter twins.
func Main(args []string) {
	fs := flag.NewFlagSet("charter twins", flag.ExitOnError)
	fs.Parse(args)
	log.SetFlags(0)
	if fs.NArg() != 2 {
		log.Fatal("✗ twins needs two projects, the reference first: charter twins examples/conformance-ts examples/conformance-go")
	}
	if err := Run(fs.Arg(0), fs.Arg(1)); err != nil {
		log.Fatal("✗ ", err)
	}
}

// answer is what a client reads of a response.
type answer struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

// read are the headers a client reads.
var read = []string{"content-type", "allow", "www-authenticate", "retry-after", "location", "cache-control"}

// A probe is one request, the same to both.
type probe struct {
	name, method, path string
	auth               string // none, read or write: the token it carries
	headers            map[string]string
	body               func() (io.Reader, string) // the body, and the content type it takes when the probe names none
}

// schema is a JSON Schema, as the spec has it.
type schema = map[string]any

type parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Schema   schema `json:"schema"`
}

type operation struct {
	OperationID string                 `json:"operationId"`
	Security    *[]map[string][]string `json:"security"`
	Parameters  []parameter            `json:"parameters"`
	RequestBody *struct {
		Content json.RawMessage `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Content map[string]json.RawMessage `json:"content"`
	} `json:"responses"`
	Idempotent bool `json:"x-fern-idempotent"`
}

// spec is the reference's OpenAPI spec, what probes are made from.
type spec struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Security   []map[string][]string                 `json:"security"`
	Components struct {
		Schemas map[string]schema `json:"schemas"`
	} `json:"components"`
	IdempotencyHeaders []struct {
		Header string `json:"header"`
	} `json:"x-fern-idempotency-headers"`
	order []string // its paths, in its order
}

func (s *spec) resolved(sc schema) schema {
	if ref, ok := sc["$ref"].(string); ok {
		return s.resolved(s.Components.Schemas[ref[strings.LastIndex(ref, "/")+1:]])
	}
	return sc
}

// typeOf is a schema's type, of a nullable one the other.
func typeOf(sc schema) string {
	switch t := sc["type"].(type) {
	case string:
		return t
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok && s != "null" {
				return s
			}
		}
	}
	return ""
}

func number(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// valueOf is a valid value for a schema: its example, its default, or the least the schema allows.
func (s *spec) valueOf(sc schema) any {
	sc = s.resolved(sc)
	if sc == nil {
		return "x"
	}
	if ex, ok := sc["examples"].([]any); ok && len(ex) > 0 {
		return ex[0]
	}
	if d, ok := sc["default"]; ok {
		return d
	}
	if e, ok := sc["enum"].([]any); ok && len(e) > 0 {
		return e[0]
	}
	switch typeOf(sc) {
	case "integer", "number":
		if m, ok := sc["minimum"]; ok {
			return m
		}
		return 1.0
	case "boolean":
		return true
	case "array":
		items, _ := sc["items"].(schema)
		return []any{s.valueOf(items)}
	case "object":
		out := map[string]any{}
		props, _ := sc["properties"].(map[string]any)
		required, _ := sc["required"].([]any)
		for _, r := range required {
			name, _ := r.(string)
			p, _ := props[name].(schema)
			out[name] = s.valueOf(p)
		}
		return out
	}
	n := 1
	if m, ok := number(sc["minLength"]); ok && int(m) > n {
		n = int(m)
	}
	return strings.Repeat("x", n)
}

// wrongType is a value of the wrong type for a schema.
func wrongType(sc schema) any {
	if typeOf(sc) == "string" {
		return 5.0
	}
	return "x"
}

// text is a value as JavaScript's String() writes it, in a query or a form.
func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return "null"
	}
	return fmt.Sprint(v)
}

// jsonText is v as JSON.stringify writes it.
func jsonText(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// values are a request's parameters, in order; a value unset is left out.
type values []struct {
	name  string
	value any
	unset bool
}

// with is vs with name set to v (or unset), in its place or at the end.
func (vs values) with(name string, v any, unset bool) values {
	out := slices.Clone(vs)
	for i := range out {
		if out[i].name == name {
			out[i].value, out[i].unset = v, unset
			return out
		}
	}
	return append(out, struct {
		name  string
		value any
		unset bool
	}{name, v, unset})
}

func (vs values) get(name string) (any, bool) {
	for _, v := range vs {
		if v.name == name && !v.unset && v.value != nil {
			return v.value, true
		}
	}
	return nil, false
}

func stringBody(s string) func() (io.Reader, string) {
	return func() (io.Reader, string) { return strings.NewReader(s), "text/plain;charset=UTF-8" }
}

// probesOf are the probes of one operation, from what the spec says of it.
func (s *spec) probesOf(path, method string, op operation, item map[string]json.RawMessage) []probe {
	label := op.OperationID
	if label == "" {
		label = strings.ToUpper(method) + " " + path
	}
	security := s.Security
	if op.Security != nil {
		security = *op.Security
	}
	secured, needsWrite := false, false
	for _, r := range security {
		secured = secured || len(r) > 0
		for _, scopes := range r {
			needsWrite = needsWrite || slices.Contains(scopes, "write")
		}
	}
	auth := "none"
	if secured {
		auth = "write"
	}
	_, stream := op.Responses["200"].Content["text/event-stream"]
	// A stream ends when its integer parameters are at their least (seconds=1).
	value := func(p parameter) any {
		if m, ok := p.Schema["minimum"]; stream && typeOf(p.Schema) == "integer" && ok {
			return m
		}
		return s.valueOf(p.Schema)
	}
	query := func(vs values) string {
		filled := path
		for _, p := range op.Parameters {
			if p.In == "path" {
				v, ok := vs.get(p.Name)
				if !ok {
					v = value(p)
				}
				filled = strings.ReplaceAll(filled, "{"+p.Name+"}", url.PathEscape(text(v)))
			}
		}
		var search []string
		for _, v := range vs {
			if !v.unset {
				search = append(search, url.QueryEscape(v.name)+"="+url.QueryEscape(text(v.value)))
			}
		}
		if len(search) > 0 {
			return filled + "?" + strings.Join(search, "&")
		}
		return filled
	}
	var good values
	for _, p := range op.Parameters {
		if p.In == "query" && (p.Required || stream) {
			good = good.with(p.Name, value(p), false)
		}
	}
	var media string
	var others []string
	var bodySchema schema
	var goodBody any
	if op.RequestBody != nil {
		others = standard.Keys(op.RequestBody.Content)
		if len(others) > 0 {
			media = others[0]
			var content map[string]struct {
				Schema schema `json:"schema"`
			}
			json.Unmarshal(op.RequestBody.Content, &content)
			bodySchema = s.resolved(content[media].Schema)
			goodBody = s.valueOf(bodySchema)
		}
	}
	encode := func(body any) func() (io.Reader, string) {
		return func() (io.Reader, string) {
			fields, _ := body.(map[string]any)
			switch media {
			case "application/x-www-form-urlencoded":
				form := url.Values{}
				for _, k := range slices.Sorted(maps.Keys(fields)) {
					form.Set(k, text(fields[k]))
				}
				return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded;charset=UTF-8"
			case "multipart/form-data":
				var b bytes.Buffer
				w := multipart.NewWriter(&b)
				props, _ := bodySchema["properties"].(map[string]any)
				for _, k := range slices.Sorted(maps.Keys(fields)) {
					p, _ := props[k].(schema)
					if v, isString := fields[k].(string); isString && s.resolved(p)["contentMediaType"] != nil {
						h := make(map[string][]string)
						h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name=%q; filename="file.txt"`, k)}
						h["Content-Type"] = []string{"text/plain"}
						part, _ := w.CreatePart(h)
						part.Write([]byte(v))
					} else {
						w.WriteField(k, text(fields[k]))
					}
				}
				w.Close()
				return &b, w.FormDataContentType()
			}
			return strings.NewReader(jsonText(body)), "text/plain;charset=UTF-8"
		}
	}
	typed := func(t string) map[string]string {
		if t != "" && t != "multipart/form-data" {
			return map[string]string{"content-type": t}
		}
		return map[string]string{}
	}
	// The idempotency key the document names: the same key, so the same id, on both.
	key := func(n string) map[string]string {
		if op.Idempotent && len(s.IdempotencyHeaders) > 0 {
			return map[string]string{s.IdempotencyHeaders[0].Header: "twins-" + label + "-" + n}
		}
		return map[string]string{}
	}
	merge := func(ms ...map[string]string) map[string]string {
		out := map[string]string{}
		for _, m := range ms {
			maps.Copy(out, m)
		}
		return out
	}
	call := func(name string, vs values, change func(*probe)) probe {
		p := probe{name: label + ": " + name, method: strings.ToUpper(method), path: query(vs), auth: auth}
		if media != "" {
			p.headers, p.body = merge(typed(media), key(name)), encode(goodBody)
		}
		if change != nil {
			change(&p)
		}
		return p
	}
	probes := []probe{call("as the spec has it", good, nil)}
	if secured {
		probes = append(probes, call("no credentials", good, func(p *probe) { p.auth = "none" }))
	}
	if needsWrite {
		probes = append(probes, call("the read token", good, func(p *probe) { p.auth = "read" }))
	}
	for _, p := range op.Parameters {
		if p.In != "query" && p.In != "path" {
			continue
		}
		sc := s.resolved(p.Schema)
		if p.Required {
			probes = append(probes, call("no "+p.Name, good.with(p.Name, nil, true), nil))
		}
		probes = append(probes, call(p.Name+" of the wrong type", good.with(p.Name, "x", typeOf(sc) == "string"), nil))
		if m, ok := number(sc["minimum"]); ok {
			probes = append(probes, call(p.Name+" under its minimum", good.with(p.Name, m-1, false), nil))
		}
		if m, ok := number(sc["maximum"]); ok {
			probes = append(probes, call(p.Name+" over its maximum", good.with(p.Name, m+1, false), nil))
		}
		if pattern, _ := sc["pattern"].(string); pattern != "" {
			probes = append(probes, call(p.Name+" not matching its pattern", good.with(p.Name, "!", false), nil))
		}
	}
	if fields, ok := goodBody.(map[string]any); media != "" && ok {
		probes = append(probes,
			call("an empty body", good, func(p *probe) { p.headers, p.body = typed(media), stringBody("") }),
			call("a property it does not have", good, func(p *probe) {
				p.headers, p.body = merge(typed(media), key("unknown")), encode(with(fields, "twins", 1.0))
			}))
		required, _ := bodySchema["required"].([]any)
		props, _ := bodySchema["properties"].(map[string]any)
		for _, r := range required {
			name, _ := r.(string)
			rest := maps.Clone(fields)
			delete(rest, name)
			probes = append(probes, call("no "+name, good, func(p *probe) { p.headers, p.body = typed(media), encode(rest) }))
			if media == "application/json" {
				prop, _ := props[name].(schema)
				wrong := with(fields, name, wrongType(s.resolved(prop)))
				probes = append(probes, call(name+" of the wrong type", good, func(p *probe) { p.headers, p.body = typed(media), encode(wrong) }))
			}
		}
		if media == "application/json" {
			probes = append(probes, call("JSON that does not parse", good, func(p *probe) { p.headers, p.body = typed(media), stringBody("{") }))
		}
		for _, t := range []string{"application/json", "text/plain"} {
			if !slices.Contains(others, t) {
				probes = append(probes, call("a body of "+t, good, func(p *probe) {
					p.headers, p.body = map[string]string{"content-type": t}, stringBody(jsonText(goodBody))
				}))
			}
		}
	}
	for _, other := range []string{"delete", "put", "patch"} {
		if _, has := item[other]; !has {
			probes = append(probes, probe{name: label + ": " + strings.ToUpper(other) + ", a method the path does not have", method: strings.ToUpper(other), path: query(good), auth: auth})
			break
		}
	}
	bare, _, _ := strings.Cut(query(nil), "?")
	return append(probes, probe{name: label + ": a trailing slash", method: strings.ToUpper(method), path: bare + "/", auth: auth})
}

func with(m map[string]any, k string, v any) map[string]any {
	out := maps.Clone(m)
	out[k] = v
	return out
}

func mcp(name, method string, params map[string]any, auth, raw string) probe {
	headers, body := standard.Request(method, params, "twins", raw)
	return probe{name: "MCP: " + name, method: "POST", path: "/api/mcp", auth: auth, headers: headers, body: stringBody(body)}
}

// probesFor is every probe of an API, from its specs: what every API serves, each operation's, each channel's.
func (s *spec) probesFor(channels, tools []string, auth string) []probe {
	probes := []probe{
		{name: "protected-resource metadata", path: "/.well-known/oauth-protected-resource"},
		{name: "authorization-server metadata", path: "/.well-known/oauth-authorization-server"},
		{name: "OpenID Connect discovery", path: "/.well-known/openid-configuration"},
		{name: "an unknown /.well-known/ document", path: "/.well-known/twins"},
		{name: "an unknown path under /api", path: "/api/twins"},
		{name: "/api itself", path: "/api"},
		{name: "the root", path: "/"},
		{name: "the OpenAPI spec", path: "/api/openapi.json"},
		{name: "the AsyncAPI spec", path: "/api/asyncapi.json"},
		mcp("tools/list", "tools/list", map[string]any{}, auth, ""),
		mcp("a tool that is not one", "tools/call", map[string]any{"name": "twins", "arguments": map[string]any{}}, auth, ""),
		mcp("an unknown method", "twins/twins", map[string]any{}, auth, ""),
		mcp("JSON that does not parse", "tools/list", map[string]any{}, auth, `{"jsonrpc":"2.0","id":1,"method":`),
		{name: "MCP: GET", path: "/api/mcp", auth: auth},
	}
	for _, tool := range tools {
		args := map[string]any{"name": tool, "arguments": map[string]any{}}
		probes = append(probes, mcp(tool+" with no arguments", "tools/call", args, auth, ""), mcp(tool+" without credentials", "tools/call", args, "none", ""))
	}
	for _, path := range s.order {
		item := s.Paths[path]
		for _, m := range standard.Methods {
			raw, ok := item[m]
			if !ok {
				continue
			}
			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				continue
			}
			probes = append(probes, s.probesOf(path, m, op, item)...)
		}
	}
	for _, address := range channels {
		probes = append(probes,
			probe{name: "channel " + address + ": no upgrade", path: address, auth: auth},
			probe{name: "channel " + address + ": no credentials", path: address},
			probe{name: "channel " + address + ": a POST", method: "POST", path: address, auth: auth, headers: map[string]string{"content-type": "application/json"}, body: stringBody("{}")})
	}
	return probes
}

var timestamp = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(\.\d+)?Z?\b`)

// normalize is an answer with what differs only because the servers differ taken out: their origin, their name (the
// Worker's, and as the SDKs' variables have it), a time.
func normalize(a answer, origin, name string) answer {
	u, _ := url.Parse(origin)
	host := u.Host
	screaming := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	ws := strings.Replace(origin, "http", "ws", 1)
	var scrub func(v any) any
	scrub = func(v any) any {
		switch x := v.(type) {
		case string:
			x = strings.ReplaceAll(x, name, "{name}")
			x = strings.ReplaceAll(x, screaming, "{NAME}")
			x = strings.ReplaceAll(x, origin, "{origin}")
			x = strings.ReplaceAll(x, ws, "{ws-origin}")
			x = strings.ReplaceAll(x, host, "{host}")
			return timestamp.ReplaceAllString(x, "{time}")
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = scrub(e)
			}
			return out
		case map[string]any:
			out := map[string]any{}
			for k, e := range x {
				if _, isString := e.(string); isString && k == "access_token" {
					out[k] = "{token}"
				} else {
					out[k] = scrub(e)
				}
			}
			return out
		}
		return v
	}
	headers := map[string]string{}
	for k, v := range a.Headers {
		headers[k] = scrub(v).(string)
	}
	return answer{a.Status, headers, scrub(a.Body)}
}

var client = &http.Client{
	Timeout:       20 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func ask(origin string, p probe, tokens map[string]string) (answer, error) {
	method := p.method
	if method == "" {
		method = "GET"
	}
	var body io.Reader
	var fallback string
	if p.body != nil {
		body, fallback = p.body()
	}
	req, err := http.NewRequest(method, origin+p.path, body)
	if err != nil {
		return answer{}, err
	}
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", fallback)
	}
	if token := tokens[p.auth]; token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return answer{}, fmt.Errorf("%s %s: %w", method, p.path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return answer{}, fmt.Errorf("%s %s: %w", method, p.path, err)
	}
	var parsed any = string(b)
	var v any
	if json.Unmarshal(b, &v) == nil {
		parsed = v
	}
	headers := map[string]string{}
	for _, name := range read {
		if vs := resp.Header.Values(name); len(vs) > 0 {
			headers[name] = strings.Join(vs, ", ")
		}
	}
	return answer{resp.StatusCode, headers, parsed}, nil
}

// missing stands for a key one side has and the other has not.
type missing struct{}

// differences is every place two values differ, as a path and the two values.
func differences(a, b any, path string) []string {
	if _, ok := a.(missing); !ok {
		if _, ok := b.(missing); !ok && jsonText(a) == jsonText(b) {
			return nil
		}
	}
	am, aObject := a.(map[string]any)
	bm, bObject := b.(map[string]any)
	if aObject && bObject {
		var out []string
		for _, k := range slices.Sorted(maps.Keys(union(am, bm))) {
			out = append(out, differences(field(am, k), field(bm, k), path+"/"+k)...)
		}
		return out
	}
	aa, aArray := a.([]any)
	ba, bArray := b.([]any)
	if aArray && bArray {
		var out []string
		for i := range max(len(aa), len(ba)) {
			var x, y any = missing{}, missing{}
			if i < len(aa) {
				x = aa[i]
			}
			if i < len(ba) {
				y = ba[i]
			}
			out = append(out, differences(x, y, path+"/"+strconv.Itoa(i))...)
		}
		return out
	}
	show := func(v any) string {
		if _, ok := v.(missing); ok {
			return "(none)"
		}
		s := jsonText(v)
		if len(s) > 300 {
			s = s[:300]
		}
		return s
	}
	if path == "" {
		path = "/"
	}
	return []string{path + ": " + show(a) + "  |  other: " + show(b)}
}

func union(a, b map[string]any) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

func field(m map[string]any, k string) any {
	if v, ok := m[k]; ok {
		return v
	}
	return missing{}
}

// toAny is an answer as JSON has it, to compare.
func toAny(a answer) any {
	var v any
	json.Unmarshal([]byte(jsonText(a)), &v)
	return v
}

// savedDifferences is what the two projects keep in files and no request shows: the specs the SDKs are made from
// (fern/, their servers apart: each names its own deployment) and the database's migrations, which must be the same
// files.
func savedDifferences(reference, refName, other, otherName string) ([]string, error) {
	var found []string
	for _, file := range []string{"fern/openapi.json", "fern/asyncapi.json"} {
		specOf := func(dir, name string) (any, error) {
			b, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				return nil, err
			}
			var v map[string]any
			if err := json.Unmarshal(b, &v); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Join(dir, file), err)
			}
			delete(v, "servers")
			return normalize(answer{Status: 200, Body: v}, "http://saved.invalid", name).Body, nil
		}
		a, err := specOf(reference, refName)
		if err != nil {
			return nil, err
		}
		b, err := specOf(other, otherName)
		if err != nil {
			return nil, err
		}
		for _, line := range differences(a, b, "") {
			found = append(found, file+": "+line)
		}
	}
	migrations := func(dir string) map[string]any {
		out := map[string]any{}
		files, _ := os.ReadDir(filepath.Join(dir, "migrations"))
		for _, f := range files {
			b, _ := os.ReadFile(filepath.Join(dir, "migrations", f.Name()))
			out[f.Name()] = string(b)
		}
		return out
	}
	for _, line := range differences(migrations(reference), migrations(other), "") {
		found = append(found, "migrations/"+line)
	}
	return found, nil
}

func self() string {
	bin, err := os.Executable()
	if err != nil {
		return "charter"
	}
	return bin
}

func fetchJSON(u string, into any) error {
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

// Run is charter twins on two projects' folders, the reference first.
func Run(reference, other string) error {
	ref, err := project.Load(reference)
	if err != nil {
		return err
	}
	oth, err := project.Load(other)
	if err != nil {
		return err
	}
	secrets := map[string]string{}
	maps.Copy(secrets, oth.Local.Vars)
	maps.Copy(secrets, ref.Local.Vars)
	state, err := os.MkdirTemp("", "charter-twins-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(state)
	ports, err := run.FreePorts(2)
	if err != nil {
		return err
	}
	origins := []string{"http://localhost:" + strconv.Itoa(ports[0]), "http://localhost:" + strconv.Itoa(ports[1])}
	// Each Worker built as check builds it, then each on local workerd with state of its own.
	for _, p := range []*project.Project{ref, oth} {
		if err := run.Run(p.Root, nil, self(), "build"); err != nil {
			return err
		}
	}
	server := func(p *project.Project, port int, name string) run.Server {
		env := maps.Clone(secrets)
		env["PORT"] = strconv.Itoa(port)
		env["CHARTER_LOCAL_STATE"] = filepath.Join(state, name)
		return run.Server{Command: []string{self(), "serve"}, Dir: p.Root, Env: env, Ready: "http://localhost:" + strconv.Itoa(port) + "/api/openapi.json"}
	}
	return run.With(server(ref, ports[0], "reference"), func() error {
		return run.With(server(oth, ports[1], "other"), func() error { return compare(reference, other, ref, oth, origins, secrets) })
	})
}

// compare sends every probe to both, the reference (refDir, as named) at origins[0], and says what differs.
func compare(refDir, othDir string, ref, oth *project.Project, origins []string, secrets map[string]string) error {
	resp, err := client.Get(origins[0] + "/api/openapi.json")
	if err != nil {
		return err
	}
	specJSON, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	var s spec
	if err := json.Unmarshal(specJSON, &s); err != nil {
		return fmt.Errorf("the reference's OpenAPI spec: %w", err)
	}
	var raw struct {
		Paths json.RawMessage `json:"paths"`
	}
	json.Unmarshal(specJSON, &raw)
	s.order = standard.Keys(raw.Paths)
	var async struct {
		Channels json.RawMessage `json:"channels"`
	}
	if err := fetchJSON(origins[0]+"/api/asyncapi.json", &async); err != nil {
		return fmt.Errorf("the reference's AsyncAPI spec: %w", err)
	}
	var channelsByName map[string]struct {
		Address string `json:"address"`
	}
	json.Unmarshal(async.Channels, &channelsByName)
	var channels []string
	for _, name := range standard.Keys(async.Channels) {
		channels = append(channels, channelsByName[name].Address)
	}
	tokens := map[string]string{"read": secrets["READ_TOKEN"], "write": secrets["WRITE_TOKEN"]}
	listed, err := ask(origins[0], mcp("tools/list", "tools/list", map[string]any{}, "write", ""), tokens)
	if err != nil {
		return err
	}
	var tools []string
	if body, ok := listed.Body.(map[string]any); ok {
		result, _ := body["result"].(map[string]any)
		list, _ := result["tools"].([]any)
		for _, t := range list {
			tool, _ := t.(map[string]any)
			if name, _ := tool["name"].(string); name != "" {
				tools = append(tools, name)
			}
		}
	}
	auth := "none"
	if secrets["WRITE_TOKEN"] != "" {
		auth = "write"
	}
	probes := s.probesFor(channels, tools, auth)
	differ := 0
	for _, p := range probes {
		a, err := ask(origins[0], p, tokens)
		if err != nil {
			return err
		}
		b, err := ask(origins[1], p, tokens)
		if err != nil {
			return err
		}
		found := differences(toAny(normalize(a, origins[0], ref.Name)), toAny(normalize(b, origins[1], oth.Name)), "")
		if len(found) == 0 {
			continue
		}
		differ++
		method := p.method
		if method == "" {
			method = "GET"
		}
		fmt.Printf("\nDIFFERS  %s  (%s %s)\n  %s %s\n", p.name, method, p.path, refDir, strings.Join(found, "\n  "+refDir+" "))
	}
	if differ > 0 {
		fmt.Printf("\n%d of %d requests answered differently by %s than by %s, the reference\n", differ, len(probes), othDir, refDir)
	} else {
		fmt.Printf("\nnone of %d requests answered differently by %s than by %s, the reference\n", len(probes), othDir, refDir)
	}
	saved, err := savedDifferences(ref.Root, ref.Name, oth.Root, oth.Name)
	if err != nil {
		return err
	}
	for _, line := range saved {
		fmt.Println("DIFFERS  " + line)
	}
	if differ > 0 || len(saved) > 0 {
		return fmt.Errorf("%s differs from %s: %d of %d requests, %d saved files", othDir, refDir, differ, len(probes), len(saved))
	}
	return nil
}
