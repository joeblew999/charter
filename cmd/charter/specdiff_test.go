package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The specs of the notes example, as `mise run spec` writes them, each changed the way a contract
// changes: is each change found, and called breaking or not?

// at walks a decoded spec: at(doc, "paths", "/api/notes", "post").
func at(v any, keys ...string) map[string]any {
	for _, key := range keys {
		v = object(v)[key]
	}
	return object(v)
}

func notesSpec(t *testing.T, file string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../examples/notes-go/fern/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

type specCase struct {
	name     string
	before   func(map[string]any) // optional: a change to both, so that after can differ from it
	after    func(map[string]any)
	want     string // in a change's place and text; "" means no change at all
	breaking bool
}

func runSpecCases(t *testing.T, file string, cases []specCase) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before, after := notesSpec(t, file), notesSpec(t, file)
			if c.before != nil {
				c.before(before)
				c.before(after)
			}
			if c.after != nil {
				c.after(after)
			}
			a, _ := json.Marshal(before)
			b, _ := json.Marshal(after)
			changes, err := diffSpecs(a, b)
			if err != nil {
				t.Fatal(err)
			}
			if c.want == "" {
				if len(changes) > 0 {
					t.Errorf("want no change, got %+v", changes)
				}
				return
			}
			for _, change := range changes {
				if strings.Contains(change.Where+": "+change.What, c.want) {
					if change.Breaking != c.breaking {
						t.Errorf("%s: breaking %v, want %v", c.want, change.Breaking, c.breaking)
					}
					return
				}
			}
			t.Errorf("no change %q in %+v", c.want, changes)
		})
	}
}

func TestSpecDiffOpenAPI(t *testing.T) {
	note := func(doc map[string]any) map[string]any { return at(doc, "components", "schemas", "Note") }
	limit := func(doc map[string]any) map[string]any {
		return object(anyList(at(doc, "paths", "/api/notes", "get")["parameters"])[1])
	}
	runSpecCases(t, "openapi.json", []specCase{
		{name: "the same", want: ""},
		{name: "an operation removed", after: func(d map[string]any) { delete(at(d, "paths", "/api/notes"), "post") },
			want: "POST /api/notes: operation notes.create removed", breaking: true},
		{name: "an operation moved", after: func(d map[string]any) {
			paths := at(d, "paths")
			paths["/api/v2/notes"] = paths["/api/notes"]
			delete(paths, "/api/notes")
		}, want: "removed, now GET /api/v2/notes", breaking: true},
		{name: "an operation added", after: func(d map[string]any) {
			at(d, "paths", "/api/notes")["delete"] = map[string]any{"operationId": "clearNotes", "responses": map[string]any{}}
		}, want: "DELETE /api/notes: operation clearNotes added"},
		{name: "an SDK method renamed", after: func(d map[string]any) { at(d, "paths", "/api/notes", "get")["x-fern-sdk-method-name"] = "all" },
			want: "SDK method notes.all, was notes.list", breaking: true},
		{name: "a response field removed", after: func(d map[string]any) { delete(at(note(d), "properties"), "body") },
			want: "POST /api/notes: field response.body removed", breaking: true},
		{name: "a response field added", after: func(d map[string]any) { at(note(d), "properties")["pinned"] = map[string]any{"type": "boolean"} },
			want: "POST /api/notes: field response.pinned added"},
		{name: "a response field no longer always there", after: func(d map[string]any) { note(d)["required"] = []any{"id", "created_at"} },
			want: "field response.body no longer always there", breaking: true},
		{name: "a field's type changed", after: func(d map[string]any) { at(note(d), "properties", "id")["type"] = "string" },
			want: "id: type string, was integer", breaking: true},
		{name: "a field in a list's items", after: func(d map[string]any) { delete(at(note(d), "properties"), "created_at") },
			want: "GET /api/notes: field response.data[].created_at removed", breaking: true},
		{name: "a required request field added", after: func(d map[string]any) {
			body := at(d, "components", "schemas", "CreateInputBody")
			at(body, "properties")["title"] = map[string]any{"type": "string"}
			body["required"] = []any{"body", "title"}
		}, want: "POST /api/notes: required field request.title added", breaking: true},
		{name: "an optional request field added", after: func(d map[string]any) {
			at(d, "components", "schemas", "CreateInputBody", "properties")["title"] = map[string]any{"type": "string"}
		}, want: "field request.title added"},
		{name: "a parameter made required", after: func(d map[string]any) { limit(d)["required"] = true },
			want: "parameter query limit now required", breaking: true},
		{name: "a parameter removed", after: func(d map[string]any) {
			op := at(d, "paths", "/api/notes", "get")
			op["parameters"] = anyList(op["parameters"])[:1]
		}, want: "parameter query limit removed", breaking: true},
		{name: "an optional parameter added", after: func(d map[string]any) {
			op := at(d, "paths", "/api/notes", "get")
			op["parameters"] = append(anyList(op["parameters"]), map[string]any{"in": "query", "name": "q", "schema": map[string]any{"type": "string"}})
		}, want: "parameter query q added"},
		{name: "a parameter takes null too", after: func(d map[string]any) { at(limit(d), "schema")["type"] = []any{"integer", "null"} },
			want: "query.limit: type integer or null, was integer"},
		{name: "a parameter narrowed to an enum", after: func(d map[string]any) { at(limit(d), "schema")["enum"] = []any{10, 20} },
			want: "query.limit: now limited to 10, 20", breaking: true},
		{name: "an enum value removed", before: func(d map[string]any) { at(limit(d), "schema")["enum"] = []any{10, 20} },
			after: func(d map[string]any) { at(limit(d), "schema")["enum"] = []any{10} }, want: "enum value 20 removed", breaking: true},
		{name: "an enum value added", before: func(d map[string]any) { at(limit(d), "schema")["enum"] = []any{10, 20} },
			after: func(d map[string]any) { at(limit(d), "schema")["enum"] = []any{10, 20, 50} }, want: "enum value 50 added"},
		{name: "a success response removed", after: func(d map[string]any) { delete(at(d, "paths", "/api/hello", "get", "responses"), "200") },
			want: "GET /api/hello: response 200 removed", breaking: true},
		{name: "a scope added", after: func(d map[string]any) {
			at(d, "paths", "/api/notes", "post")["security"] = []any{map[string]any{"bearer": []any{"write", "admin"}}}
		}, want: "POST /api/notes: security tightened: bearer[write,admin], was bearer[write]", breaking: true},
		{name: "an open operation closed", after: func(d map[string]any) {
			at(d, "paths", "/api/notes", "get")["security"] = []any{map[string]any{"bearer": []any{"read"}}}
		}, want: "GET /api/notes: security tightened: bearer[read], was anyone", breaking: true},
		{name: "another way in added", after: func(d map[string]any) {
			op := at(d, "paths", "/api/notes", "post")
			op["security"] = append(anyList(op["security"]), map[string]any{"partner": []any{"write"}})
		}, want: "security loosened"},
		{name: "an upper limit lowered", after: func(d map[string]any) { at(limit(d), "schema")["maximum"] = 50 },
			want: "query.limit: maximum 50, was 100", breaking: true},
		{name: "an upper limit raised", after: func(d map[string]any) { at(limit(d), "schema")["maximum"] = 500 },
			want: "query.limit: maximum 500, was 100"},
		{name: "a lower limit raised on a request field", after: func(d map[string]any) {
			at(d, "components", "schemas", "CreateInputBody", "properties", "body")["minLength"] = 3
		}, want: "request.body: minLength 3, was 1", breaking: true},
		{name: "a new limit on a request field", after: func(d map[string]any) {
			at(d, "components", "schemas", "CreateInputBody", "properties", "body")["maxLength"] = 280
		}, want: "request.body: now maxLength 280", breaking: true},
		{name: "a new limit on a response field", after: func(d map[string]any) { at(note(d), "properties", "body")["maxLength"] = 280 },
			want: "response.body: now maxLength 280"},
		{name: "a pattern on a request field", after: func(d map[string]any) {
			at(d, "components", "schemas", "CreateInputBody", "properties", "body")["pattern"] = "^[a-z]+$"
		}, want: "request.body: now pattern ^[a-z]+$", breaking: true},
		{name: "a variant removed from what is sent", before: func(d map[string]any) {
			at(d, "components", "schemas", "CreateInputBody", "properties")["kind"] = map[string]any{"oneOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "integer"}}}
		}, after: func(d map[string]any) {
			at(d, "components", "schemas", "CreateInputBody", "properties", "kind")["oneOf"] = []any{map[string]any{"type": "string"}}
		}, want: "request.kind: oneOf #2 removed", breaking: true},
		{name: "a variant added to what is received", before: func(d map[string]any) {
			at(note(d), "properties")["owner"] = map[string]any{"anyOf": []any{map[string]any{"$ref": "#/components/schemas/HelloOutputBody"}}}
		}, after: func(d map[string]any) {
			at(note(d), "properties", "owner")["anyOf"] = []any{map[string]any{"$ref": "#/components/schemas/HelloOutputBody"}, map[string]any{"$ref": "#/components/schemas/ErrorDetail"}}
		}, want: "response.owner: anyOf ErrorDetail added", breaking: true},
		{name: "inside a variant", before: func(d map[string]any) {
			at(note(d), "properties")["owner"] = map[string]any{"anyOf": []any{map[string]any{"$ref": "#/components/schemas/HelloOutputBody"}}}
		}, after: func(d map[string]any) { delete(at(d, "components", "schemas", "HelloOutputBody", "properties"), "message") },
			want: "field response.owner(HelloOutputBody).message removed", breaking: true},
		{name: "a token no longer needed", after: func(d map[string]any) { delete(at(d, "paths", "/api/notes", "post"), "security") },
			want: "security loosened: anyone, was bearer[write]"},
	})
}

func TestSpecDiffAsyncAPI(t *testing.T) {
	channel := func(d map[string]any) map[string]any { return at(d, "channels", "liveNotes") }
	runSpecCases(t, "asyncapi.json", []specCase{
		{name: "the same", want: ""},
		{name: "a channel removed", after: func(d map[string]any) { delete(at(d, "channels"), "liveNotes") },
			want: "channel liveNotes: removed", breaking: true},
		{name: "an address changed", after: func(d map[string]any) { channel(d)["address"] = "/api/live" },
			want: "address /api/live, was /api/notes/live", breaking: true},
		{name: "a field of a received message removed", after: func(d map[string]any) { delete(at(d, "components", "schemas", "Note", "properties"), "body") },
			want: "channel liveNotes: field Note.body removed", breaking: true},
		{name: "a field of a received message added", after: func(d map[string]any) {
			at(d, "components", "schemas", "Note", "properties")["pinned"] = map[string]any{"type": "boolean"}
		}, want: "field Note.pinned added"},
		{name: "a query parameter made required", after: func(d map[string]any) { at(channel(d), "bindings", "ws", "query")["required"] = []any{"after"} },
			want: "field ws.query.after now required", breaking: true},
		{name: "a message removed", after: func(d map[string]any) { delete(at(channel(d), "messages"), "Note") },
			want: "message Note removed", breaking: true},
		{name: "an operation's direction changed", after: func(d map[string]any) { at(d, "operations", "receiveNote")["action"] = "send" },
			want: "operation receiveNote: action send, was receive", breaking: true},
		{name: "an operation removed", after: func(d map[string]any) { delete(at(d, "operations"), "receiveNote") },
			want: "operation receiveNote: removed", breaking: true},
		{name: "a channel closed to anonymous subscribers", after: func(d map[string]any) {
			at(d, "components")["securitySchemes"] = map[string]any{"bearer": map[string]any{"type": "http", "scheme": "bearer"}}
			at(d, "operations", "receiveNote")["security"] = []any{map[string]any{"$ref": "#/components/securitySchemes/bearer"}}
		}, want: "channel liveNotes: security tightened: bearer[], was anyone", breaking: true},
		{name: "a channel opened", before: func(d map[string]any) {
			at(d, "components")["securitySchemes"] = map[string]any{"bearer": map[string]any{"type": "http", "scheme": "bearer"}}
			at(d, "operations", "receiveNote")["security"] = []any{map[string]any{"$ref": "#/components/securitySchemes/bearer"}}
		}, after: func(d map[string]any) { delete(at(d, "operations", "receiveNote"), "security") },
			want: "channel liveNotes: security loosened: anyone, was bearer[]"},
	})
}

func TestSpecDiffRefusesMixedSpecs(t *testing.T) {
	if _, err := diffSpecs([]byte(`{"openapi":"3.1.0"}`), []byte(`{"asyncapi":"3.0.0"}`)); err == nil {
		t.Error("an OpenAPI spec compared with an AsyncAPI one")
	}
}

func TestMajorRelease(t *testing.T) {
	for _, c := range []struct {
		from, to string
		major    bool
	}{
		{"v0.1.0", "v0.2.0", true}, {"v0.1.0", "v0.1.1", false}, {"v1.4.2", "v2.0.0", true},
		{"v1.4.2", "v1.5.0", false}, {"v0.9.3", "v1.0.0", true}, {"v1.0.0", "v0.9.0", false},
	} {
		if got := majorRelease(c.from, c.to); got != c.major {
			t.Errorf("%s to %s: major %v, want %v", c.from, c.to, got, c.major)
		}
	}
}
