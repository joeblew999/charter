package humamcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/api-go/humaworkers"
)

// How an operation becomes a tool:
//
//   - name: its OperationID; description: its Summary, then its Description.
//   - inputSchema: one object. Every path, query, header and cookie parameter is a property (hidden
//     ones are not parameters to Huma, so they are left out). A JSON request body that is an object
//     adds its properties next to them, so the arguments are flat, as an oRPC procedure's input is;
//     any other body is the property "body". An object body with a property that has a parameter's
//     name is also the property "body" (PUT /things/{id} with the thing, id and all, as its body),
//     so both values have a place.
//   - outputSchema: the schema of the 2xx application/json response, when it is an object.
//   - $ref: Huma's "#/components/schemas/Note" becomes "#/$defs/Note", with the schemas referred to
//     in $defs, so each tool's schemas stand alone.
//   - annotations: GET is read-only; of the others, all but POST may overwrite or delete.
//
// And how a call becomes a request: arguments go where the operation's parameters say, the rest is
// the JSON body. What Huma answers is the result: the body as text, also as structuredContent when
// it is a JSON object, and isError for 4xx and 5xx (the text is then Huma's problem+json, with the
// locations of what was wrong).

// Tool is one entry of tools/list.
type Tool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

const (
	metadataKey = "mcp"
	bodyName    = "body"
	jsonType    = "application/json"
	schemaRef   = "#/components/schemas/"
)

// Expose says whether op is a tool, whatever IsTool would decide by itself. A streaming operation
// made a tool must end by itself: the whole response is the result.
func Expose(op huma.Operation, tool bool) huma.Operation {
	if op.Metadata == nil {
		op.Metadata = map[string]any{}
	}
	op.Metadata[metadataKey] = tool
	return op
}

// IsTool reports whether op is a tool: by default, every operation with an OperationID that is in
// OpenAPI (not Hidden) and does not stream. A tool call is one request and one answer, so a
// text/event-stream operation or a WebSocket channel has nothing to give it.
func IsTool(op *huma.Operation) bool {
	if op.OperationID == "" {
		return false
	}
	if tool, ok := op.Metadata[metadataKey].(bool); ok {
		return tool
	}
	if op.Hidden {
		return false
	}
	for _, response := range op.Responses {
		if response != nil && response.Content["text/event-stream"] != nil {
			return false
		}
	}
	return true
}

// Tools are the tools among ops (humaworkers' API.Operations()), in their order, with schemas from
// registry (api.OpenAPI().Components.Schemas). An operation that cannot be a tool is left out and
// logged, and the others are listed: Check says which, before a client asks.
func Tools(ops []*huma.Operation, registry huma.Registry) []Tool {
	tools := []Tool{}
	for _, op := range ops {
		if !IsTool(op) {
			continue
		}
		in, err := inputOf(op, registry)
		if err != nil {
			log.Printf("%v: it is not in tools/list", err)
			continue
		}
		tool := Tool{Name: op.OperationID, Description: op.Summary, InputSchema: in.schema(registry), OutputSchema: outputSchema(op, registry)}
		if op.Summary != "" && op.Description != "" {
			tool.Description += "\n\n"
		}
		tool.Description += op.Description
		switch op.Method {
		case http.MethodGet, http.MethodHead:
			tool.Annotations = map[string]any{"readOnlyHint": true}
		default:
			tool.Annotations = map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": op.Method != http.MethodPost,
				"idempotentHint":  op.Method == http.MethodPut || op.Method == http.MethodDelete,
			}
		}
		tools = append(tools, tool)
	}
	return tools
}

// Check is why each operation that would be a tool cannot be one: nil for an API whose tools/list
// is complete. Call it where a contract is tested and where its specs are written (cmd/spec), so
// that a missing tool is found there and not by a client. The fix is to rename what clashes, or
// Expose(op, false).
func Check(api *humaworkers.API) []error {
	var problems []error
	for _, op := range api.Operations() {
		if !IsTool(op) {
			continue
		}
		if _, err := inputOf(op, api.API.OpenAPI().Components.Schemas); err != nil {
			problems = append(problems, err)
		}
	}
	return problems
}

// input is where a tool's arguments go in the operation's request.
type input struct {
	op     *huma.Operation
	params []*huma.Param
	body   *huma.Schema // the JSON request body, or nil
	flat   bool         // the body's properties are arguments. Otherwise it is the argument "body"
}

// inputOf is op's input, or why its arguments cannot be one object: two parameters of one name, or
// a parameter named "body" next to a body that is that argument.
func inputOf(op *huma.Operation, registry huma.Registry) (input, error) {
	in := input{op: op}
	names := map[string]bool{}
	for _, param := range op.Parameters {
		if param == nil || param.Schema == nil {
			continue
		}
		if names[param.Name] {
			return in, fmt.Errorf("humamcp: %s has two parameters named %q", op.OperationID, param.Name)
		}
		names[param.Name] = true
		in.params = append(in.params, param)
	}
	if op.RequestBody == nil || op.RequestBody.Content[jsonType] == nil || op.RequestBody.Content[jsonType].Schema == nil {
		return in, nil
	}
	in.body = resolve(op.RequestBody.Content[jsonType].Schema, registry)
	in.flat = in.body.Type == huma.TypeObject && len(in.body.Properties) > 0
	// A body property named as a parameter is: the body stays whole, so both have a place.
	for name := range in.body.Properties {
		in.flat = in.flat && !names[name]
	}
	if !in.flat && names[bodyName] {
		return in, fmt.Errorf("humamcp: %s has a parameter named %q, which is the argument its request body is", op.OperationID, bodyName)
	}
	return in, nil
}

// schema is the tool's inputSchema.
func (in input) schema(registry huma.Registry) map[string]any {
	defs := definitions{registry: registry, schemas: map[string]any{}}
	properties := map[string]any{}
	required := []string{}
	var additional any = false
	for _, param := range in.params {
		property, _ := defs.plain(param.Schema).(map[string]any)
		if property == nil {
			property = map[string]any{}
		}
		if property["description"] == nil && param.Description != "" {
			property["description"] = param.Description
		}
		properties[param.Name] = property
		if param.Required {
			required = append(required, param.Name)
		}
	}
	if in.body != nil {
		body, _ := defs.plain(in.body).(map[string]any)
		if in.flat {
			flat, _ := body["properties"].(map[string]any)
			for name, property := range flat {
				properties[name] = property
			}
			if list, ok := body["required"].([]any); ok && in.op.RequestBody.Required {
				for _, name := range list {
					required = append(required, name.(string))
				}
			}
			if value, ok := body["additionalProperties"]; ok {
				additional = value
			}
		} else {
			properties[bodyName] = body
			if in.op.RequestBody.Required {
				required = append(required, bodyName)
			}
		}
	}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": additional}
	if len(required) > 0 {
		schema["required"] = required
	}
	defs.into(schema)
	return schema
}

// outputSchema is the schema of op's first 2xx JSON response, if that is an object: MCP before
// 2026-07-28 only knows object results.
func outputSchema(op *huma.Operation, registry huma.Registry) map[string]any {
	statuses := make([]string, 0, len(op.Responses))
	for status := range op.Responses {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		response := op.Responses[status]
		if !strings.HasPrefix(status, "2") || response == nil || response.Content[jsonType] == nil || response.Content[jsonType].Schema == nil {
			continue
		}
		body := resolve(response.Content[jsonType].Schema, registry)
		if body.Type != huma.TypeObject {
			return nil
		}
		defs := definitions{registry: registry, schemas: map[string]any{}}
		schema, _ := defs.plain(body).(map[string]any)
		defs.into(schema)
		return schema
	}
	return nil
}

// resolve follows a schema that only refers to a registry schema.
func resolve(schema *huma.Schema, registry huma.Registry) *huma.Schema {
	for schema.Ref != "" {
		target := registry.SchemaFromRef(schema.Ref)
		if target == nil {
			break
		}
		schema = target
	}
	return schema
}

// definitions collects the registry schemas that a tool's schemas refer to.
type definitions struct {
	registry huma.Registry
	schemas  map[string]any
}

// plain is schema as plain JSON values, with its references pointing into $defs.
func (d definitions) plain(schema *huma.Schema) any {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return nil
	}
	d.rewrite(value)
	return value
}

func (d definitions) rewrite(value any) {
	switch value := value.(type) {
	case map[string]any:
		ref, _ := value["$ref"].(string)
		if name, ok := strings.CutPrefix(ref, schemaRef); ok {
			value["$ref"] = "#/$defs/" + name
			if _, done := d.schemas[name]; !done {
				d.schemas[name] = nil // being collected: a schema may refer to itself
				d.schemas[name] = d.plain(d.registry.Map()[name])
			}
		}
		for _, child := range value {
			d.rewrite(child)
		}
	case []any:
		for _, child := range value {
			d.rewrite(child)
		}
	}
}

func (d definitions) into(schema map[string]any) {
	if len(d.schemas) > 0 {
		schema["$defs"] = d.schemas
	}
}

// run calls op with a tool call's arguments, through api, and returns the tool result.
func run(api *humaworkers.API, r *http.Request, op *huma.Operation, arguments map[string]any) map[string]any {
	in, err := inputOf(op, api.API.OpenAPI().Components.Schemas)
	if err != nil {
		return toolError(err.Error())
	}
	path, query, header := op.Path, url.Values{}, http.Header{}
	// The operation sees the caller's credentials, as it would over REST.
	if authorization := r.Header.Get("Authorization"); authorization != "" {
		header.Set("Authorization", authorization)
	}
	for _, param := range in.params {
		value, given := arguments[param.Name]
		delete(arguments, param.Name)
		if !given || value == nil {
			if param.In == "path" {
				return toolError("path." + param.Name + ": expected required path parameter to be present")
			}
			continue
		}
		switch param.In {
		case "path":
			if text(value) == "" {
				return toolError("path." + param.Name + ": expected a value")
			}
			path = strings.ReplaceAll(path, "{"+param.Name+"}", url.PathEscape(text(value)))
		case "query":
			if list, ok := value.([]any); ok && param.Explode != nil && *param.Explode {
				for _, item := range list {
					query.Add(param.Name, text(item))
				}
			} else {
				query.Set(param.Name, text(value))
			}
		case "header":
			header.Set(param.Name, text(value))
		case "cookie":
			header.Add("Cookie", (&http.Cookie{Name: param.Name, Value: text(value)}).String())
		}
	}
	var body []byte
	if in.body != nil {
		var value any
		if in.flat && (len(arguments) > 0 || op.RequestBody.Required) {
			value, arguments = arguments, nil
		} else if !in.flat {
			value = arguments[bodyName]
			delete(arguments, bodyName)
		}
		if value != nil {
			if body, err = json.Marshal(value); err != nil {
				return toolError(err.Error())
			}
			header.Set("Content-Type", jsonType)
		}
	}
	if len(arguments) > 0 {
		names := make([]string, 0, len(arguments))
		for name := range arguments {
			names = append(names, name)
		}
		sort.Strings(names)
		return toolError("unexpected arguments: " + strings.Join(names, ", "))
	}

	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(r.Context(), op.Method, target, bytes.NewReader(body))
	if err != nil {
		return toolError(err.Error())
	}
	request.Header, request.Host, request.RemoteAddr = header, r.Host, r.RemoteAddr
	request.URL.Scheme, request.URL.Host = r.URL.Scheme, r.URL.Host // absolute on Workers
	response := &recorder{header: http.Header{}}
	api.ServeHTTP(response, request)

	answer := bytes.TrimSpace(response.body.Bytes())
	result := map[string]any{"content": []any{}}
	if len(answer) > 0 {
		result["content"] = []any{map[string]any{"type": "text", "text": string(answer)}}
	}
	if response.status >= 400 {
		result["isError"] = true
	} else if len(answer) > 0 && answer[0] == '{' && json.Valid(answer) {
		result["structuredContent"] = json.RawMessage(answer)
	}
	return result
}

func toolError(text string) map[string]any {
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "isError": true}
}

// text is an argument as a path segment, query value or header: a list is comma-separated, which
// is how Huma reads one.
func text(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	case bool:
		return strconv.FormatBool(value)
	case []any:
		items := make([]string, len(value))
		for i, item := range value {
			items[i] = text(item)
		}
		return strings.Join(items, ",")
	}
	data, _ := json.Marshal(value)
	return string(data)
}

// recorder keeps what the operation answers.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *recorder) Write(p []byte) (int, error) {
	r.WriteHeader(http.StatusOK)
	return r.body.Write(p)
}
