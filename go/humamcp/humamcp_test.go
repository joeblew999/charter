package humamcp

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/humaworkers"
)

type thing struct {
	ID    int64    `json:"id"`
	Name  string   `json:"name"`
	Tags  []string `json:"tags,omitempty"`
	Trace string   `json:"trace,omitempty"`
	Auth  string   `json:"auth,omitempty"`
}

type getInput struct {
	ID      int64    `path:"id" doc:"The thing's id"`
	Verbose bool     `query:"verbose"`
	Fields  []string `query:"fields"`
	Trace   string   `header:"X-Trace"`
	Auth    string   `header:"Authorization" hidden:"true"`
	Access  string   `header:"Cf-Access-Jwt-Assertion" hidden:"true"`
}

type createInput struct {
	Dry  bool `query:"dry"`
	Body struct {
		Name  string   `json:"name" minLength:"2"`
		Tags  []string `json:"tags,omitempty"`
		Owner *thing   `json:"owner,omitempty"`
	}
}

type listInput struct {
	Body []string
}

// credentials is what the operation was shown: a bearer token, or Access's JWT.
func credentials(authorization, access string) string {
	if authorization == "" && access != "" {
		return "access " + access
	}
	return authorization
}

type thingOutput struct{ Body thing }

type thingsOutput struct{ Body []thing }

// An API with one operation of each kind, counting registrations.
func testAPI(registered map[string]int) *humaworkers.API {
	route := func(op huma.Operation, register func(huma.API, huma.Operation)) humaworkers.Route {
		return humaworkers.Route{Method: op.Method, Path: op.Path, OperationID: op.OperationID, Register: func(api huma.API) {
			registered[op.OperationID]++
			register(api, op)
		}}
	}
	get := func(_ context.Context, in *getInput) (*thingOutput, error) {
		if in.ID == 404 {
			return nil, huma.Error404NotFound("no such thing")
		}
		name := "thing"
		if in.Verbose {
			name = "a verbose thing"
		}
		return &thingOutput{Body: thing{ID: in.ID, Name: name, Tags: in.Fields, Trace: in.Trace, Auth: credentials(in.Auth, in.Access)}}, nil
	}
	stream := func(context.Context, *struct{}) (*huma.StreamResponse, error) {
		return &huma.StreamResponse{Body: func(hc huma.Context) { hc.BodyWriter().Write([]byte("data: 1\n\n")) }}, nil
	}
	sse := map[string]*huma.Response{"200": {Description: "OK", Content: map[string]*huma.MediaType{"text/event-stream": {}}}}
	return humaworkers.New(humaworkers.Config("things", "1.2.3"), []humaworkers.Route{
		route(huma.Operation{OperationID: "getThing", Method: "GET", Path: "/things/{id}", Summary: "Get a thing", Description: "By id."},
			func(api huma.API, op huma.Operation) { huma.Register(api, op, get) }),
		route(huma.Operation{OperationID: "createThing", Method: "POST", Path: "/things", Summary: "Create a thing"},
			func(api huma.API, op huma.Operation) {
				huma.Register(api, op, func(_ context.Context, in *createInput) (*thingOutput, error) {
					return &thingOutput{Body: thing{ID: 9007199254740993, Name: in.Body.Name, Tags: in.Body.Tags}}, nil
				})
			}),
		route(huma.Operation{OperationID: "replaceThings", Method: "PUT", Path: "/things"},
			func(api huma.API, op huma.Operation) {
				huma.Register(api, op, func(_ context.Context, in *listInput) (*thingsOutput, error) {
					out := &thingsOutput{}
					for _, name := range in.Body {
						out.Body = append(out.Body, thing{Name: name})
					}
					return out, nil
				})
			}),
		route(huma.Operation{OperationID: "deleteThing", Method: "DELETE", Path: "/things/{id}", DefaultStatus: 204},
			func(api huma.API, op huma.Operation) {
				huma.Register(api, op, func(context.Context, *struct {
					ID string `path:"id"`
				}) (*struct{}, error) {
					return nil, nil
				})
			}),
		route(huma.Operation{OperationID: "watchThings", Method: "GET", Path: "/things/watch", Responses: sse},
			func(api huma.API, op huma.Operation) { huma.Register(api, op, stream) }),
		route(huma.Operation{OperationID: "hiddenThing", Method: "GET", Path: "/hidden/{id}", Hidden: true},
			func(api huma.API, op huma.Operation) { huma.Register(api, op, get) }),
		route(Expose(huma.Operation{OperationID: "secretThing", Method: "GET", Path: "/secret/{id}", Hidden: true}, true),
			func(api huma.API, op huma.Operation) { huma.Register(api, op, get) }),
		route(Expose(huma.Operation{OperationID: "internalThing", Method: "GET", Path: "/internal/{id}"}, false),
			func(api huma.API, op huma.Operation) { huma.Register(api, op, get) }),
	})
}

type answer struct {
	Status int
	Header http.Header
	Raw    string
	ID     any
	Result map[string]any
	Error  *struct {
		Code    int
		Message string
		Data    map[string]any
	}
}

// post sends one message as a client of the handshake era would; headers are name, value pairs.
func post(t *testing.T, h http.Handler, body string, headers ...string) answer {
	t.Helper()
	req := httptest.NewRequest("POST", "http://things.example/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	a := answer{Status: rec.Code, Header: rec.Header(), Raw: rec.Body.String()}
	if strings.HasPrefix(a.Raw, "{") {
		if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
			t.Fatalf("%s: %v", a.Raw, err)
		}
	}
	return a
}

func rpc(method, params string) string {
	if params == "" {
		params = "{}"
	}
	return `{"jsonrpc":"2.0","id":7,"method":"` + method + `","params":` + params + `}`
}

func call(t *testing.T, h http.Handler, name, arguments string, headers ...string) map[string]any {
	t.Helper()
	a := post(t, h, rpc("tools/call", `{"name":"`+name+`","arguments":`+arguments+`}`), headers...)
	if a.Status != 200 || a.Result == nil {
		t.Fatalf("tools/call %s: HTTP %d %s", name, a.Status, a.Raw)
	}
	return a.Result
}

// text is a tool result's one text block.
func textOf(t *testing.T, result map[string]any) string {
	t.Helper()
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content: %v", result["content"])
	}
	block := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Fatalf("content: %v", content)
	}
	return block["text"].(string)
}

func tools(t *testing.T, h http.Handler) map[string]map[string]any {
	t.Helper()
	a := post(t, h, rpc("tools/list", ""))
	if a.Status != 200 || a.Result == nil {
		t.Fatalf("tools/list: HTTP %d %s", a.Status, a.Raw)
	}
	byName := map[string]map[string]any{}
	for _, tool := range a.Result["tools"].([]any) {
		byName[tool.(map[string]any)["name"].(string)] = tool.(map[string]any)
	}
	return byName
}

func same(t *testing.T, what string, got any, want string) {
	t.Helper()
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		out, _ := json.Marshal(got)
		t.Errorf("%s:\n got %s\nwant %s", what, out, want)
	}
}

func TestEveryOperationThatAnswersOnceIsATool(t *testing.T) {
	h := Handler(testAPI(map[string]int{}))
	// Calls before the list (a server that lives on) don't change its order.
	call(t, h, "secretThing", `{"id": 1}`)
	call(t, h, "createThing", `{"name": "first"}`)
	a := post(t, h, rpc("tools/list", ""))
	var names []string
	for _, tool := range a.Result["tools"].([]any) {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	// In the routes' order. Not the stream, not the hidden one, not the one opted out; the hidden one opted in.
	if want := []string{"getThing", "createThing", "replaceThings", "deleteThing", "secretThing"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("tools %v, want %v", names, want)
	}
	if a.Result["resultType"] != nil || a.Result["ttlMs"] != nil {
		t.Errorf("a handshake-era result has 2026-07-28 fields: %s", a.Raw)
	}
}

func TestInputSchemaIsParametersAndBodyInOneObject(t *testing.T) {
	list := tools(t, Handler(testAPI(map[string]int{})))

	// Path, query and header parameters; the hidden header is not there.
	get := list["getThing"]
	same(t, "getThing.inputSchema", get["inputSchema"], `{
		"type": "object", "additionalProperties": false, "required": ["id"],
		"properties": {
			"id": {"type": "integer", "format": "int64", "description": "The thing's id"},
			"verbose": {"type": "boolean"},
			"fields": {"type": ["array", "null"], "items": {"type": "string"}},
			"X-Trace": {"type": "string"}
		}}`)
	if get["description"] != "Get a thing\n\nBy id." {
		t.Errorf("description %q", get["description"])
	}
	same(t, "getThing.annotations", get["annotations"], `{"readOnlyHint": true}`)

	// An object body: its properties are arguments, next to the query parameter, and a schema it
	// refers to is in $defs.
	create := list["createThing"]
	same(t, "createThing.inputSchema", create["inputSchema"], `{
		"type": "object", "additionalProperties": false, "required": ["name"],
		"properties": {
			"dry": {"type": "boolean"},
			"name": {"type": "string", "minLength": 2},
			"tags": {"type": ["array", "null"], "items": {"type": "string"}},
			"owner": {"$ref": "#/$defs/Thing"}
		},
		"$defs": {"Thing": {
			"type": "object", "additionalProperties": false, "required": ["id", "name"],
			"properties": {"id": {"type": "integer", "format": "int64"}, "name": {"type": "string"}, "tags": {"type": ["array", "null"], "items": {"type": "string"}}, "trace": {"type": "string"}, "auth": {"type": "string"}}
		}}}`)
	same(t, "createThing.annotations", create["annotations"], `{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false}`)
	// The output is an object: its schema is given, standing alone.
	same(t, "createThing.outputSchema", create["outputSchema"], `{
		"type": "object", "additionalProperties": false, "required": ["id", "name"],
		"properties": {"id": {"type": "integer", "format": "int64"}, "name": {"type": "string"}, "tags": {"type": ["array", "null"], "items": {"type": "string"}}, "trace": {"type": "string"}, "auth": {"type": "string"}}}`)

	// A body that is not an object is the argument "body"; an array output has no outputSchema.
	replace := list["replaceThings"]
	same(t, "replaceThings.inputSchema", replace["inputSchema"], `{
		"type": "object", "additionalProperties": false, "required": ["body"],
		"properties": {"body": {"type": ["array", "null"], "items": {"type": "string"}}}}`)
	if replace["outputSchema"] != nil {
		t.Errorf("replaceThings.outputSchema: %v", replace["outputSchema"])
	}
	same(t, "replaceThings.annotations", replace["annotations"], `{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": true}`)
}

func TestACallRunsTheOperation(t *testing.T) {
	h := Handler(testAPI(map[string]int{}))

	// Arguments go to the path, the query and a header; the caller's Authorization is passed on.
	result := call(t, h, "getThing", `{"id": 42, "verbose": true, "fields": ["a", "b"], "X-Trace": "t1"}`, "Authorization", "Bearer k")
	want := `{"id":42,"name":"a verbose thing","tags":["a","b"],"trace":"t1","auth":"Bearer k"}`
	if got := textOf(t, result); got != want || result["isError"] != nil {
		t.Fatalf("getThing: %v", result)
	}
	same(t, "structuredContent", result["structuredContent"], want)
	// A caller Cloudflare Access let in: its JWT is passed on too.
	if got := textOf(t, call(t, h, "getThing", `{"id": 1}`, "Cf-Access-Jwt-Assertion", "eyJ.a.b")); !strings.Contains(got, `"auth":"access eyJ.a.b"`) {
		t.Fatalf("getThing through Access: %s", got)
	}

	// The body's properties, flat, with the query parameter beside them. A 64-bit id comes back whole.
	result = call(t, h, "createThing", `{"name": "new", "tags": ["x"], "dry": true}`)
	if got := textOf(t, result); got != `{"id":9007199254740993,"name":"new","tags":["x"]}` {
		t.Fatalf("createThing: %s", got)
	}
	// And goes in whole.
	if got := textOf(t, call(t, h, "getThing", `{"id": 9007199254740993}`)); !strings.Contains(got, `"id":9007199254740993`) {
		t.Fatalf("getThing: %s", got)
	}

	// A body under "body"; an array answer is text only.
	result = call(t, h, "replaceThings", `{"body": ["a", "b"]}`)
	if got := textOf(t, result); got != `[{"id":0,"name":"a"},{"id":0,"name":"b"}]` || result["structuredContent"] != nil {
		t.Fatalf("replaceThings: %v", result)
	}

	// No content: an empty result, not an error.
	result = call(t, h, "deleteThing", `{"id": "1"}`)
	if content := result["content"].([]any); len(content) != 0 || result["isError"] != nil {
		t.Fatalf("deleteThing: %v", result)
	}
}

func TestWhatTheOperationRefusesIsAToolError(t *testing.T) {
	h := Handler(testAPI(map[string]int{}))
	for name, test := range map[string]struct{ tool, arguments, want string }{
		"validation, with Huma's location": {"createThing", `{"name": "x"}`, `"location":"body.name"`},
		"a wrong type":                     {"getThing", `{"id": "abc"}`, `"location":"path.id"`},
		"an argument the body rejects":     {"createThing", `{"name": "new", "nope": 1}`, `"location":"body.nope"`},
		"the handler's own error":          {"getThing", `{"id": 404}`, `"status":404`},
		"a missing path parameter":         {"getThing", `{}`, `path.id: expected required path parameter to be present`},
		"an argument nothing takes":        {"getThing", `{"id": 1, "nope": 1, "also": 2}`, `unexpected arguments: also, nope`},
		"a missing body":                   {"replaceThings", `{}`, `request body is required`},
	} {
		result := call(t, h, test.tool, test.arguments)
		if got := textOf(t, result); result["isError"] != true || !strings.Contains(got, test.want) || result["structuredContent"] != nil {
			t.Errorf("%s: %v, want an error with %s", name, result, test.want)
		}
	}
}

func TestACallRegistersOnlyItsOwnOperation(t *testing.T) {
	registered := map[string]int{}
	h := Handler(testAPI(registered))
	for range 2 {
		call(t, h, "getThing", `{"id": 1}`)
	}
	post(t, h, rpc("initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}`))
	if !reflect.DeepEqual(registered, map[string]int{"getThing": 1}) {
		t.Fatalf("registered %v, want getThing once and nothing else", registered)
	}
}

func TestProtocolErrors(t *testing.T) {
	h := Handler(testAPI(map[string]int{}))
	for name, test := range map[string]struct {
		body   string
		status int
		code   int
	}{
		"malformed JSON":             {`{"jsonrpc":"2.0","id":1,"method":`, 400, -32700},
		"not JSON-RPC 2.0":           {`{"id":1,"method":"ping"}`, 400, -32600},
		"a response":                 {`{"jsonrpc":"2.0","id":1,"result":{}}`, 400, -32600},
		"a batch":                    {`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, 400, -32600},
		"unknown method":             {rpc("resources/list", ""), 200, -32601},
		"unknown tool":               {rpc("tools/call", `{"name":"nope","arguments":{}}`), 200, -32602},
		"no tool name":               {rpc("tools/call", `{}`), 200, -32602},
		"an operation not exposed":   {rpc("tools/call", `{"name":"internalThing","arguments":{"id":1}}`), 200, -32602},
		"a stream":                   {rpc("tools/call", `{"name":"watchThings","arguments":{}}`), 200, -32602},
		"arguments that are a list":  {rpc("tools/call", `{"name":"getThing","arguments":[1]}`), 200, -32602},
		"params that are not object": {`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"x"}`, 200, -32602},
	} {
		a := post(t, h, test.body)
		if a.Status != test.status || a.Error == nil || a.Error.Code != test.code {
			t.Errorf("%s: HTTP %d %s, want HTTP %d and error %d", name, a.Status, a.Raw, test.status, test.code)
		}
	}
	if a := post(t, h, rpc("tools/call", `{"name":"nope"}`)); a.Error == nil || a.Error.Message != "Unknown tool: nope" || a.ID != float64(7) {
		t.Errorf("unknown tool: %s", a.Raw)
	}
}

func TestTheTransportHasNoStreamAndNoSession(t *testing.T) {
	h := Handler(testAPI(map[string]int{}))
	for _, method := range []string{"GET", "DELETE"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "http://things.example/mcp", nil))
		if rec.Code != 405 || rec.Header().Get("Allow") != "POST" {
			t.Errorf("%s: HTTP %d Allow %q, want 405 POST", method, rec.Code, rec.Header().Get("Allow"))
		}
	}
	// Notifications are accepted, with no body.
	if a := post(t, h, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); a.Status != 202 || a.Raw != "" {
		t.Errorf("notification: HTTP %d %q, want 202 and no body", a.Status, a.Raw)
	}
	// A page of another site is refused; the server's own origin and no origin are not.
	if a := post(t, h, rpc("ping", ""), "Origin", "http://evil.example"); a.Status != 403 {
		t.Errorf("foreign Origin: HTTP %d, want 403", a.Status)
	}
	if a := post(t, h, rpc("ping", ""), "Origin", "http://things.example"); a.Status != 200 || a.Result == nil {
		t.Errorf("own Origin: HTTP %d %s", a.Status, a.Raw)
	}
}

func TestHandshakeEra(t *testing.T) {
	h := Handler(testAPI(map[string]int{}))
	initialize := func(version string) answer {
		return post(t, h, rpc("initialize", `{"protocolVersion":"`+version+`","capabilities":{},"clientInfo":{"name":"t","version":"1"}}`))
	}
	a := initialize("2025-06-18")
	same(t, "initialize", a.Result, `{"protocolVersion": "2025-06-18", "capabilities": {"tools": {}}, "serverInfo": {"name": "things", "version": "1.2.3"}}`)
	if a.Header.Get("Mcp-Session-Id") != "" || a.Header.Get("Content-Type") != "application/json" {
		t.Errorf("initialize headers: %v", a.Header)
	}
	// A version not served: the newest one of the era is offered.
	for _, version := range []string{"2024-11-05", "2026-07-28", "2099-01-01"} {
		if got := initialize(version).Result["protocolVersion"]; got != "2025-11-25" {
			t.Errorf("initialize %s: offered %v, want 2025-11-25", version, got)
		}
	}
	if a := post(t, h, rpc("ping", "")); a.Status != 200 || len(a.Result) != 0 || a.Error != nil {
		t.Errorf("ping: HTTP %d %s", a.Status, a.Raw)
	}
	// Later requests name the negotiated version; one that is not served is refused.
	if a := post(t, h, rpc("tools/list", ""), "MCP-Protocol-Version", "2025-11-25"); a.Status != 200 || a.Result == nil {
		t.Errorf("tools/list with the version header: HTTP %d %s", a.Status, a.Raw)
	}
	if a := post(t, h, rpc("tools/list", ""), "MCP-Protocol-Version", "2024-11-05"); a.Status != 400 || a.Error == nil || a.Error.Code != -32022 {
		t.Errorf("tools/list with an old version header: HTTP %d %s", a.Status, a.Raw)
	}
}

// modern sends a request as 2026-07-28 says: the version and capabilities in _meta, mirrored headers.
func modern(t *testing.T, h http.Handler, method, params string, headers ...string) answer {
	t.Helper()
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":{"name":"t","version":"1"}}`
	if params != "" {
		meta = params + "," + meta
	}
	return post(t, h, rpc(method, "{"+meta+"}"), append([]string{"MCP-Protocol-Version", "2026-07-28", "Mcp-Method", method}, headers...)...)
}

func TestStatelessEra(t *testing.T) {
	h := Handler(testAPI(map[string]int{}))
	const server = `{"io.modelcontextprotocol/serverInfo": {"name": "things", "version": "1.2.3"}}`

	a := modern(t, h, "server/discover", "")
	same(t, "server/discover", a.Result, `{"resultType": "complete", "supportedVersions": ["2026-07-28", "2025-11-25", "2025-06-18"],
		"capabilities": {"tools": {}}, "ttlMs": 300000, "cacheScope": "public", "_meta": `+server+`}`)

	a = modern(t, h, "tools/list", "")
	if a.Status != 200 || a.Result["resultType"] != "complete" || a.Result["ttlMs"] != float64(300000) || a.Result["cacheScope"] != "public" || len(a.Result["tools"].([]any)) != 5 {
		t.Errorf("tools/list: HTTP %d %s", a.Status, a.Raw)
	}
	same(t, "tools/list _meta", a.Result["_meta"], server)

	a = modern(t, h, "tools/call", `"name":"getThing","arguments":{"id":3}`, "Mcp-Name", "getThing")
	same(t, "tools/call", a.Result, `{"resultType": "complete", "_meta": `+server+`,
		"content": [{"type": "text", "text": "{\"id\":3,\"name\":\"thing\"}"}], "structuredContent": {"id": 3, "name": "thing"}}`)
	// The name may be Base64 in its header.
	if a := modern(t, h, "tools/call", `"name":"getThing","arguments":{"id":3}`, "Mcp-Name", "=?base64?Z2V0VGhpbmc=?="); a.Status != 200 || a.Result == nil {
		t.Errorf("tools/call with a Base64 Mcp-Name: HTTP %d %s", a.Status, a.Raw)
	}
	// A tool error is still a result.
	if a := modern(t, h, "tools/call", `"name":"getThing","arguments":{"id":404}`, "Mcp-Name", "getThing"); a.Status != 200 || a.Result["isError"] != true || a.Result["resultType"] != "complete" {
		t.Errorf("tools/call that fails: HTTP %d %s", a.Status, a.Raw)
	}

	meta := func(version string) string {
		return `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + version + `","io.modelcontextprotocol/clientCapabilities":{}}}`
	}
	for name, test := range map[string]struct {
		a      answer
		status int
		code   int
	}{
		"unknown method: 404":           {modern(t, h, "resources/list", ""), 404, -32601},
		"ping left the protocol":        {modern(t, h, "ping", ""), 404, -32601},
		"initialize with _meta":         {modern(t, h, "initialize", ""), 404, -32601},
		"unknown tool":                  {modern(t, h, "tools/call", `"name":"nope"`, "Mcp-Name", "nope"), 200, -32602},
		"no version header":             {post(t, h, rpc("tools/list", meta("2026-07-28")), "Mcp-Method", "tools/list"), 400, -32020},
		"another version in the header": {post(t, h, rpc("tools/list", meta("2026-07-28")), "MCP-Protocol-Version", "2025-11-25", "Mcp-Method", "tools/list"), 400, -32020},
		"no Mcp-Method":                 {post(t, h, rpc("tools/list", meta("2026-07-28")), "MCP-Protocol-Version", "2026-07-28"), 400, -32020},
		"another Mcp-Method":            {modern(t, h, "tools/list", "", "Mcp-Method", "tools/call"), 400, -32020},
		"no Mcp-Name":                   {modern(t, h, "tools/call", `"name":"getThing","arguments":{"id":3}`), 400, -32020},
		"another Mcp-Name":              {modern(t, h, "tools/call", `"name":"getThing","arguments":{"id":3}`, "Mcp-Name", "createThing"), 400, -32020},
		"a version not served":          {post(t, h, rpc("tools/list", meta("2027-01-01")), "MCP-Protocol-Version", "2027-01-01", "Mcp-Method", "tools/list"), 400, -32022},
		"no client capabilities": {post(t, h, rpc("tools/list", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`),
			"MCP-Protocol-Version", "2026-07-28", "Mcp-Method", "tools/list"), 400, -32602},
		"the version header without _meta": {post(t, h, rpc("tools/list", ""), "MCP-Protocol-Version", "2026-07-28", "Mcp-Method", "tools/list"), 400, -32602},
	} {
		if test.a.Status != test.status || test.a.Error == nil || test.a.Error.Code != test.code {
			t.Errorf("%s: HTTP %d %s, want HTTP %d and error %d", name, test.a.Status, test.a.Raw, test.status, test.code)
		}
	}
	a = post(t, h, rpc("tools/list", meta("2027-01-01")), "MCP-Protocol-Version", "2027-01-01", "Mcp-Method", "tools/list")
	same(t, "unsupported version data", a.Error.Data, `{"supported": ["2026-07-28", "2025-11-25", "2025-06-18"], "requested": "2027-01-01"}`)
}

// The reporter's shape: POST /x/{id} whose body has an id too.
type clashInput struct {
	ID   string `path:"id"`
	Body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
}

type clashOutput struct {
	Body struct {
		Path string `json:"path"`
		ID   string `json:"id"`
		Name string `json:"name"`
	}
}

// No Route.OperationID: the operation is still found by id.
func clashRoute(id, path string, register func(huma.API, huma.Operation)) humaworkers.Route {
	return humaworkers.Route{Method: "POST", Path: path, Register: func(api huma.API) {
		register(api, huma.Operation{OperationID: id, Method: "POST", Path: path})
	}}
}

func clashAPI() *humaworkers.API {
	nothing := func(api huma.API, op huma.Operation) {
		huma.Register(api, op, func(context.Context, *struct{}) (*struct{}, error) { return nil, nil })
	}
	return humaworkers.New(humaworkers.Config("t", "1"), []humaworkers.Route{
		clashRoute("before", "/before", nothing),
		clashRoute("clash", "/x/{id}", func(api huma.API, op huma.Operation) {
			huma.Register(api, op, func(_ context.Context, in *clashInput) (*clashOutput, error) {
				out := &clashOutput{}
				out.Body.Path, out.Body.ID, out.Body.Name = in.ID, in.Body.ID, in.Body.Name
				return out, nil
			})
		}),
		// No object can hold these arguments: a parameter named body, and a body that is the argument body.
		clashRoute("bodyTwice", "/twice", func(api huma.API, op huma.Operation) {
			huma.Register(api, op, func(context.Context, *struct {
				Mode string `query:"body"`
				Body []string
			}) (*struct{}, error) {
				return nil, nil
			})
		}),
		clashRoute("after", "/after", nothing),
	})
}

func TestABodyPropertyNamedAsAParameterKeepsTheBodyWhole(t *testing.T) {
	h := Handler(clashAPI())
	same(t, "clash.inputSchema", tools(t, h)["clash"]["inputSchema"], `{
		"type": "object", "additionalProperties": false, "required": ["id", "body"],
		"properties": {
			"id": {"type": "string"},
			"body": {
				"type": "object", "additionalProperties": false, "required": ["id", "name"],
				"properties": {"id": {"type": "string"}, "name": {"type": "string"}}
			}
		}}`)
	result := call(t, h, "clash", `{"id": "from-the-path", "body": {"id": "from-the-body", "name": "n"}}`)
	same(t, "clash result", result["structuredContent"], `{"path": "from-the-path", "id": "from-the-body", "name": "n"}`)
	// The flat form has no place for the second id.
	if result := call(t, h, "clash", `{"id": "x", "name": "n"}`); result["isError"] != true || !strings.Contains(textOf(t, result), "unexpected arguments: name") {
		t.Errorf("flat arguments: %v", result)
	}
}

func TestAnOperationThatCannotBeAToolIsLeftOutAndTheOthersAreListed(t *testing.T) {
	var logged strings.Builder
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	api := clashAPI()
	a := post(t, Handler(api), rpc("tools/list", ""))
	var names []string
	for _, tool := range a.Result["tools"].([]any) {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	if want := []string{"before", "clash", "after"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("tools %v (%s), want %v", names, a.Raw, want)
	}
	// Why, in the log and from Check, which a contract's tests and its spec command call.
	problems := Check(api)
	if want := `bodyTwice has a parameter named "body"`; len(problems) != 1 || !strings.Contains(problems[0].Error(), want) || !strings.Contains(logged.String(), want) {
		t.Errorf("want %q in Check (%v) and in the log (%s)", want, problems, logged.String())
	}
	// A call says why too.
	if result := call(t, Handler(api), "bodyTwice", `{"body": "x"}`); result["isError"] != true || !strings.Contains(textOf(t, result), `a parameter named "body"`) {
		t.Errorf("tools/call bodyTwice: %v", result)
	}
	if problems := Check(testAPI(map[string]int{})); problems != nil {
		t.Errorf("Check on an API without problems: %v", problems)
	}
}
