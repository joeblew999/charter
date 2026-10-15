package api

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// The oRPC conformance's Fern folder: its specs are generated from ts/conformance/src/contract.ts
// (npm run spec, there). What the Go contract must say too.
const reference = "../../../ts/conformance/fern/"

func read(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func parse(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func spec(t *testing.T, generate func(string) ([]byte, error)) map[string]any {
	t.Helper()
	raw, err := generate("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	return parse(t, raw)
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

func asList(v any) []any         { l, _ := v.([]any); return l }
func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

// JavaScript's safe-integer bounds: how a Zod contract says "an integer", not a limit.
const safeInteger = float64(1<<53 - 1)

// shape is a schema as an SDK's types see it: its type, its properties with theirs, which are
// required, an array's items, a file. References are followed, so a schema has the same shape named
// or inline. Left out on purpose, because no SDK sees them: the schema's name where one side
// inlines it (Huma names every body, e.g. CreateInputBody), descriptions, `additionalProperties:
// false`, integer formats (Huma writes int32, which Fern types as a plain integer) and the
// safe-integer bounds.
func shape(doc map[string]any, schema any) any {
	s := asMap(schema)
	if ref, ok := s["$ref"].(string); ok {
		return shape(doc, at(doc, strings.Split(strings.TrimPrefix(ref, "#/"), "/")...))
	}
	out := map[string]any{}
	for _, key := range []string{"type", "minimum", "maximum", "default", "pattern", "enum"} {
		if value, ok := s[key]; ok && value != safeInteger && value != -safeInteger {
			out[key] = value
		}
	}
	if s["format"] == "binary" {
		out["format"] = "binary"
	}
	if properties := asMap(s["properties"]); len(properties) > 0 {
		shapes := map[string]any{}
		for name, property := range properties {
			shapes[name] = shape(doc, property)
		}
		out["properties"] = shapes
	}
	if required := asList(s["required"]); len(required) > 0 {
		names := make([]string, len(required))
		for i, name := range required {
			names[i] = name.(string)
		}
		sort.Strings(names)
		out["required"] = names
	}
	if items, ok := s["items"]; ok {
		out["items"] = shape(doc, items)
	}
	return out
}

// content is a request or response body per media type.
func content(doc map[string]any, body any) map[string]any {
	out := map[string]any{}
	for media, value := range asMap(at(body, "content")) {
		out[media] = shape(doc, at(value, "schema"))
	}
	return out
}

// operation is what Fern builds an SDK method from: its id, tags and summary, its x-fern-*
// extensions, its security, its parameters, its request body and its 200 response per media type.
// Left out: other responses. Huma declares its error model (a `default` response,
// application/problem+json) on every operation, so the SDKs made from the Go specs have a typed
// error where the oRPC spec declares none.
func operation(doc map[string]any, op map[string]any) string {
	entry := map[string]any{"operationId": op["operationId"], "tags": op["tags"], "summary": op["summary"], "security": op["security"]}
	for key, value := range op {
		if strings.HasPrefix(key, "x-fern-") {
			entry[key] = value
		}
	}
	var parameters []string
	for _, p := range asList(op["parameters"]) {
		parameter := asMap(p)
		constraints, _ := json.Marshal(shape(doc, parameter["schema"]))
		parameters = append(parameters, parameter["in"].(string)+" "+parameter["name"].(string)+" required="+boolean(parameter["required"])+" "+string(constraints))
	}
	sort.Strings(parameters)
	entry["parameters"] = parameters
	if body := asMap(op["requestBody"]); body != nil {
		entry["requestBody"] = map[string]any{"required": boolean(body["required"]), "content": content(doc, body)}
	}
	entry["200"] = content(doc, at(op, "responses", "200"))
	out, _ := json.Marshal(entry)
	return string(out)
}

func boolean(v any) string {
	if b, _ := v.(bool); b {
		return "true"
	}
	return "false"
}

// surface is everything in an OpenAPI document that reaches an SDK, as comparable text per item.
func surface(doc map[string]any) map[string]string {
	out := map[string]string{}
	for _, group := range []string{"paths", "webhooks"} {
		for path, item := range asMap(doc[group]) {
			for method, op := range asMap(item) {
				if op, ok := op.(map[string]any); ok {
					out[group+": "+strings.ToUpper(method)+" "+path] = operation(doc, op)
				}
			}
		}
	}
	for _, key := range []string{"security", "x-fern-idempotency-headers", "x-fern-webhook-signature"} {
		value, _ := json.Marshal(doc[key])
		out["document: "+key] = string(value)
	}
	schemes, _ := json.Marshal(at(doc, "components", "securitySchemes"))
	out["document: securitySchemes"] = string(schemes)
	return out
}

// channels is everything in an AsyncAPI document that reaches an SDK: each channel's address, the
// query it is opened with and its messages, each operation, and each message's payload.
func channels(doc map[string]any) map[string]string {
	out := map[string]string{}
	for name, channel := range asMap(doc["channels"]) {
		var messages []string
		for message := range asMap(at(channel, "messages")) {
			messages = append(messages, message)
		}
		sort.Strings(messages)
		entry, _ := json.Marshal(map[string]any{"address": at(channel, "address"), "messages": messages, "query": shape(doc, at(channel, "bindings", "ws", "query"))})
		out["channel: "+name] = string(entry)
	}
	for name, op := range asMap(doc["operations"]) {
		entry, _ := json.Marshal(op)
		out["operation: "+name] = string(entry)
	}
	for name, message := range asMap(at(doc, "components", "messages")) {
		entry, _ := json.Marshal(shape(doc, at(message, "payload")))
		out["message: "+name] = string(entry)
	}
	return out
}

// The Go contract and the oRPC conformance contract describe the same API: every operation, webhook,
// document-level setting and channel an SDK sees must be the same in both generated specs, with no
// difference tolerated (the oRPC one is the reference). What the comparison leaves out is said on
// shape and operation above; what the servers answer is compared by twins.ts.
func TestSameSurfaceAsTheORPCConformance(t *testing.T) {
	openapi, asyncapi := parse(t, read(t, reference+"openapi.json")), parse(t, read(t, reference+"asyncapi.json"))
	compare := func(what string, want, got map[string]string) {
		for key, w := range want {
			if got[key] != w {
				t.Errorf("%s %s\n  oRPC: %s\n  Go:   %s", what, key, w, got[key])
			}
		}
		for key := range got {
			if _, ok := want[key]; !ok {
				t.Errorf("%s %s is in the Go contract only", what, key)
			}
		}
	}
	compare("openapi", surface(openapi), surface(spec(t, API(Env{}).OpenAPI)))
	compare("asyncapi", channels(asyncapi), channels(spec(t, API(Env{}).AsyncAPI)))
}

// Fern applies an overlay to each API's OpenAPI spec (generators.yml), and it names the two notes
// methods (notes.list, notes.create). Both conformance projects must use the same one.
func TestSameOverlayAsTheORPCConformance(t *testing.T) {
	actions := func(path string) string {
		var lines []string
		for _, line := range strings.Split(string(read(t, path)), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				lines = append(lines, line)
			}
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	want, got := actions(reference+"overlays.yml"), actions("../fern/overlays.yml")
	if want != got || !strings.Contains(got, "x-fern-sdk-method-name: list") {
		t.Errorf("the overlays differ (comments apart)\n  oRPC:\n%s\n  Go:\n%s", want, got)
	}
	// The names are the overlay's alone: a contract that also said them would hide a broken overlay.
	for _, method := range []string{"get", "post"} {
		if name := at(spec(t, API(Env{}).OpenAPI), "paths", "/api/notes", method, "x-fern-sdk-method-name"); name != nil {
			t.Errorf("%s /notes names its SDK method in the contract (%v): the overlay does that", method, name)
		}
	}
}

// The parts of the spec that Huma has no struct tag for are written in code (spec.go, contract.go).
func TestTheSpecHasWhatHumaHasNoTagFor(t *testing.T) {
	doc := spec(t, API(Env{}).OpenAPI)
	for _, path := range [][]string{
		{"webhooks", "noteCreated", "post", "requestBody", "content", "application/json", "schema"},
		{"x-fern-webhook-signature"},
		{"x-fern-idempotency-headers"},
		{"components", "securitySchemes", OAuth, "flows", "clientCredentials", "tokenUrl"},
		{"paths", "/api/oauth/token", "post", "requestBody", "content", "application/x-www-form-urlencoded", "schema"},
		{"paths", "/api/files", "post", "requestBody", "content", "multipart/form-data", "schema", "properties", "file"},
		{"paths", "/api/chat", "post", "responses", "200", "content", "text/event-stream", "schema"},
	} {
		if at(doc, path...) == nil {
			t.Errorf("the spec has no %s", strings.Join(path, "."))
		}
	}
	// The token endpoint is the one operation without security; the Idempotency-Key header is not a
	// parameter (the document-level extension declares it); the channel is not in OpenAPI.
	if security, ok := at(doc, "paths", "/api/oauth/token", "post", "security").([]any); !ok || len(security) != 0 {
		t.Errorf("getToken's security: %v, want []", at(doc, "paths", "/api/oauth/token", "post", "security"))
	}
	if at(doc, "paths", "/api/notes", "post", "parameters") != nil {
		t.Errorf("createNote has parameters: %v", at(doc, "paths", "/api/notes", "post", "parameters"))
	}
	if at(doc, "paths", livePath) != nil {
		t.Errorf("%s is in OpenAPI", livePath)
	}
}
