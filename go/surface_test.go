package examples

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// The notes API exists twice here, from a Go contract (notes-go) and from an oRPC one (notes-ts).
// Each project's check holds its committed specs to its contract (spec:check); this test holds the
// two sets of specs to each other. It is the repo's, not a project's: a project has one contract.

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

// The Go contract (notes-go/api/contract.go) and the oRPC contract (notes-ts/src/contract.ts) describe the same API:
// every operation the SDKs see must be the same in both specs, and so must the channels.
func TestTheNotesExamplesHaveTheSameSurface(t *testing.T) {
	read := func(name string) map[string]any {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	want, got := surface(t, read("notes-ts/fern/openapi.json")), surface(t, read("notes-go/fern/openapi.json"))
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
	orpc, goSpec := read("notes-ts/fern/asyncapi.json"), read("notes-go/fern/asyncapi.json")
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

// Each notes example keeps its own migrations/, as every project does, but both servers serve the
// same API from the same table: the two folders hold the same files, byte for byte.
func TestTheNotesExamplesHaveTheSameSchema(t *testing.T) {
	files := func(dir string) map[string]string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, entry := range entries {
			raw, err := os.ReadFile(dir + "/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			out[entry.Name()] = string(raw)
		}
		return out
	}
	goFiles, tsFiles := files("notes-go/migrations"), files("notes-ts/migrations")
	if len(goFiles) == 0 {
		t.Fatal("notes-go/migrations: no files")
	}
	for name, content := range goFiles {
		if other, ok := tsFiles[name]; !ok {
			t.Errorf("migrations/%s is in notes-go only", name)
		} else if other != content {
			t.Errorf("migrations/%s differs between notes-go and notes-ts", name)
		}
	}
	for name := range tsFiles {
		if _, ok := goFiles[name]; !ok {
			t.Errorf("migrations/%s is in notes-ts only", name)
		}
	}
}

func at(doc any, path ...string) any {
	for _, key := range path {
		m, ok := doc.(map[string]any)
		if !ok {
			return nil
		}
		doc = m[key]
	}
	return doc
}

// Fern sends only the credentials fern/generators.yml restates (auth-schemes, auth: any), so each
// notes example's restates its spec's bearer and Access schemes, with the same headers and
// variables (an OIDC token goes as the bearer token), and the two examples declare the same schemes
// and security.
func TestFernRestatesTheSchemes(t *testing.T) {
	security := map[string]string{}
	for _, project := range []string{"notes-go", "notes-ts"} {
		raw, err := os.ReadFile(project + "/fern/openapi.json")
		if err != nil {
			t.Fatal(err)
		}
		yml, err := os.ReadFile(project + "/fern/generators.yml")
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		schemes := asMap(at(doc, "components", "securitySchemes"))
		if len(schemes) != 4 || schemes["oidc"] == nil {
			t.Errorf("%s: %d security schemes, want bearer, Access's two and oidc", project, len(schemes))
		}
		for name, raw := range schemes {
			if name == "oidc" {
				continue
			}
			scheme := raw.(map[string]any)
			want := []string{"    " + name + ":\n", "      - " + name + "\n"}
			if scheme["type"] == "apiKey" {
				fern := asMap(scheme["x-fern-header"])
				want = append(want, "header: "+scheme["name"].(string)+"\n", "name: "+fern["name"].(string)+"\n", "env: "+fern["env"].(string)+"\n")
			}
			for _, w := range want {
				if !strings.Contains(string(yml), w) {
					t.Errorf("%s/fern/generators.yml: the scheme %s has no %q", project, name, strings.TrimSpace(w))
				}
			}
		}
		if !strings.Contains(string(yml), "  auth:\n    any:\n") {
			t.Errorf("%s/fern/generators.yml: auth is not any", project)
		}
		names := make([]string, 0, len(schemes))
		for name := range schemes {
			names = append(names, name)
		}
		sort.Strings(names)
		create, _ := json.Marshal(at(doc, "paths", "/api/notes", "post", "security"))
		security[project] = strings.Join(names, " ") + " " + string(create)
	}
	if security["notes-go"] != security["notes-ts"] {
		t.Errorf("the schemes and createNote's security differ:\n  Go: %s\n  TS: %s", security["notes-go"], security["notes-ts"])
	}
}
