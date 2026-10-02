package api

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// Only in this repo, where the same API also exists as an oRPC contract: `dev new` leaves this file
// out of a new project.

// surface is what Fern builds an SDK's methods from: per operation its id, its parameters (name,
// place, constraints) and its x-fern-* extensions.
func surface(t *testing.T, doc map[string]any) map[string]string {
	t.Helper()
	out := map[string]string{}
	paths, _ := doc["paths"].(map[string]any)
	for path, item := range paths {
		for method, raw := range item.(map[string]any) {
			op, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			entry := map[string]any{"operationId": op["operationId"], "tags": op["tags"], "summary": op["summary"]}
			for key, value := range op {
				if strings.HasPrefix(key, "x-fern-") {
					entry[key] = value
				}
			}
			var params []string
			for _, p := range asList(op["parameters"]) {
				param := p.(map[string]any)
				schema, _ := param["schema"].(map[string]any)
				constraints, _ := json.Marshal(map[string]any{"type": schema["type"], "min": schema["minimum"], "max": schema["maximum"], "default": schema["default"], "pattern": schema["pattern"]})
				params = append(params, param["in"].(string)+" "+param["name"].(string)+" "+string(constraints))
			}
			sort.Strings(params)
			entry["parameters"] = params
			var media []string
			for name := range asMap(at(op, "responses", "200", "content")) {
				media = append(media, name)
			}
			entry["200"] = media
			b, _ := json.Marshal(entry)
			out[strings.ToUpper(method)+" "+path] = string(b)
		}
	}
	return out
}

func asList(v any) []any         { l, _ := v.([]any); return l }
func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

// In this repo the Go contract and the oRPC contract (api/ts/src/contract.ts) describe the same API:
// every operation the SDKs see must be the same in both specs, and so must the channels.
func TestSameSurfaceAsTheORPCContract(t *testing.T) {
	read := func(name string) map[string]any {
		raw, err := os.ReadFile("../../../sdk/fern/apis/api/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	want, got := surface(t, read("openapi.json")), surface(t, spec(t, OpenAPI))
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s\n  oRPC: %s\n  Go:   %s", key, w, got[key])
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("%s is in the Go contract only", key)
		}
	}
	orpc, goSpec := read("asyncapi.json"), spec(t, AsyncAPI)
	for _, path := range [][]string{
		{"channels", "liveNotes", "address"},
		{"channels", "liveNotes", "summary"},
		{"channels", "liveNotes", "bindings", "ws", "query", "properties", "after"},
		{"operations", "receiveNote"},
	} {
		// Examples are left out, here as in surface: the Go contract gives them (`example` tags) and
		// the oRPC one doesn't, and they change no SDK method.
		if property, ok := at(goSpec, path...).(map[string]any); ok {
			delete(property, "examples")
		}
		w, _ := json.Marshal(at(orpc, path...))
		g, _ := json.Marshal(at(goSpec, path...))
		if string(w) != string(g) {
			t.Errorf("asyncapi %s\n  oRPC: %s\n  Go:   %s", strings.Join(path, "."), w, g)
		}
	}
}
