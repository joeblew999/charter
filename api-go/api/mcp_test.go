package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/joeblew999/orpc-api/api-go/humaworkers"
)

// mcp POSTs one JSON-RPC request to /api/mcp and returns the HTTP status and the decoded answer.
func mcp(t *testing.T, base, method, params string) (int, map[string]any) {
	t.Helper()
	status, body, _ := do(t, "POST", base+"/api/mcp", `{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`)
	var answer map[string]any
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		t.Fatalf("%s: HTTP %d %s", method, status, body)
	}
	return status, answer
}

// toolCall is a tool call's result: its text, structuredContent and isError.
func toolCall(t *testing.T, base, name, arguments string) (string, any, bool) {
	t.Helper()
	status, answer := mcp(t, base, "tools/call", `{"name":"`+name+`","arguments":`+arguments+`}`)
	result, _ := answer["result"].(map[string]any)
	if status != 200 || result == nil {
		t.Fatalf("tools/call %s: HTTP %d %v", name, status, answer)
	}
	text := ""
	for _, block := range result["content"].([]any) {
		text += block.(map[string]any)["text"].(string)
	}
	return text, result["structuredContent"], result["isError"] == true
}

func TestMCPToolsAreTheContractsOperations(t *testing.T) {
	srv, _ := server(t)
	status, answer := mcp(t, srv.URL, "tools/list", `{}`)
	if status != 200 {
		t.Fatalf("HTTP %d %v", status, answer)
	}
	got, _ := json.Marshal(answer["result"])
	// The three operations that answer once. Not watchNotes (a stream) and not liveNotes (a channel).
	want := `{"tools":[
		{"name":"hello","description":"Say hello","annotations":{"readOnlyHint":true},
		 "inputSchema":{"type":"object","properties":{},"additionalProperties":false},
		 "outputSchema":{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}},
		{"name":"listNotes","description":"List notes, newest first (cursor pagination)","annotations":{"readOnlyHint":true},
		 "inputSchema":{"type":"object","additionalProperties":false,"properties":{
			"cursor":{"type":"string","description":"Opaque cursor from the previous page's next_cursor"},
			"limit":{"type":"integer","format":"int32","minimum":1,"maximum":100,"default":20}}},
		 "outputSchema":{"type":"object","additionalProperties":false,"required":["data"],"properties":{
			"data":{"type":"array","items":{"$ref":"#/$defs/Note"}},
			"next_cursor":{"type":"string","description":"Pass as cursor for the next page; absent on the last page"}},
			"$defs":{"Note":{"type":"object","additionalProperties":false,"required":["id","body","created_at"],"properties":{
				"id":{"type":"integer","format":"int64"},"body":{"type":"string"},"created_at":{"type":"string"}}}}}},
		{"name":"createNote","description":"Create a note","annotations":{"readOnlyHint":false,"destructiveHint":false,"idempotentHint":false},
		 "inputSchema":{"type":"object","additionalProperties":false,"required":["body"],"properties":{"body":{"type":"string","minLength":1}}},
		 "outputSchema":{"type":"object","additionalProperties":false,"required":["id","body","created_at"],"properties":{
			"id":{"type":"integer","format":"int64"},"body":{"type":"string"},"created_at":{"type":"string"}}}}
	]}`
	var gotValue, wantValue any
	json.Unmarshal(got, &gotValue)
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("tools/list:\n got %s\nwant %s", got, want)
	}
}

func TestMCPCallsRunTheSameOperationsAsREST(t *testing.T) {
	srv, memory := server(t)
	status, answer := mcp(t, srv.URL, "initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}`)
	want := map[string]any{
		"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}},
		"serverInfo": map[string]any{"name": Title, "version": Version}, "instructions": Description,
	}
	if status != 200 || !reflect.DeepEqual(answer["result"], want) {
		t.Fatalf("initialize: HTTP %d %v, want %v", status, answer["result"], want)
	}

	text, structured, isError := toolCall(t, srv.URL, "hello", `{}`)
	if text != `{"message":"Hello from test"}` || isError {
		t.Fatalf("hello: %s", text)
	}
	if message := structured.(map[string]any)["message"]; message != "Hello from test" {
		t.Fatalf("hello structuredContent: %v", structured)
	}

	// A note created through MCP is in the store, and one created over REST is listed through MCP.
	text, structured, isError = toolCall(t, srv.URL, "createNote", `{"body":"from mcp"}`)
	if isError || structured.(map[string]any)["body"] != "from mcp" {
		t.Fatalf("createNote: %s", text)
	}
	if notes, _ := memory.Before(t.Context(), 1<<62, 10); len(notes) != 1 || notes[0].Body != "from mcp" {
		t.Fatalf("store: %v", notes)
	}
	rest := create(t, srv.URL, "from rest")
	text, structured, isError = toolCall(t, srv.URL, "listNotes", `{"limit":1}`)
	list := structured.(map[string]any)
	if data := list["data"].([]any); isError || len(data) != 1 || data[0].(map[string]any)["body"] != "from rest" || list["next_cursor"] != "2" {
		t.Fatalf("listNotes: %s", text)
	}
	// The same bytes as the REST route answers.
	if _, body, _ := do(t, "GET", srv.URL+"/api/notes?limit=1", ""); strings.TrimSpace(body) != text {
		t.Fatalf("listNotes over MCP: %s\nover REST: %s", text, body)
	}
	text, _, _ = toolCall(t, srv.URL, "listNotes", `{"limit":1,"cursor":"2"}`)
	if !strings.Contains(text, `"from mcp"`) || strings.Contains(text, "next_cursor") || rest.ID != 2 {
		t.Fatalf("listNotes page 2: %s", text)
	}
}

func TestMCPErrors(t *testing.T) {
	srv, memory := server(t)
	// What the contract refuses is a tool error carrying Huma's problem, with the location.
	for name, test := range map[string]struct{ tool, arguments, location string }{
		"a query parameter out of range": {"listNotes", `{"limit":1000}`, `"location":"query.limit"`},
		"the handler's own rule":         {"listNotes", `{"cursor":"x"}`, `"location":"query.cursor"`},
		"an empty body":                  {"createNote", `{"body":""}`, `"location":"body.body"`},
		"no body":                        {"createNote", `{}`, `"location":"body"`},
		"the Resolve rule":               {"createNote", `{"body":"a ` + END + `"}`, `"location":"body.body"`},
	} {
		text, structured, isError := toolCall(t, srv.URL, test.tool, test.arguments)
		if !isError || !strings.Contains(text, test.location) || !strings.Contains(text, `"status":422`) || structured != nil {
			t.Errorf("%s: isError %v, %s; want an error with %s", name, isError, text, test.location)
		}
	}
	if notes, _ := memory.Before(t.Context(), 1<<62, 10); len(notes) != 0 {
		t.Errorf("refused notes were stored: %v", notes)
	}

	// What is not a call at all is a protocol error.
	code := func(answer map[string]any) any {
		failure, _ := answer["error"].(map[string]any)
		return failure["code"]
	}
	for name, test := range map[string]struct {
		method, params string
		code           float64
	}{
		"unknown tool":         {"tools/call", `{"name":"deleteNote","arguments":{}}`, -32602},
		"a stream is no tool":  {"tools/call", `{"name":"watchNotes","arguments":{"seconds":1}}`, -32602},
		"a channel is no tool": {"tools/call", `{"name":"liveNotes","arguments":{}}`, -32602},
		"unknown method":       {"resources/list", `{}`, -32601},
	} {
		if status, answer := mcp(t, srv.URL, test.method, test.params); status != 200 || code(answer) != test.code {
			t.Errorf("%s: HTTP %d %v, want error %v", name, status, answer, test.code)
		}
	}
	status, body, _ := do(t, "POST", srv.URL+"/api/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"`)
	var answer map[string]any
	json.Unmarshal([]byte(body), &answer)
	if status != 400 || code(answer) != float64(-32700) {
		t.Errorf("malformed JSON: HTTP %d %s, want 400 and error -32700", status, body)
	}
	if status, _, header := do(t, "GET", srv.URL+"/api/mcp", ""); status != 405 || header.Get("Allow") != "POST" {
		t.Errorf("GET: HTTP %d Allow %q, want 405 POST", status, header.Get("Allow"))
	}
}

// Route.OperationID is how a tool call finds its one operation: it must be the operation's own id.
func TestEveryRouteNamesItsOperation(t *testing.T) {
	for _, route := range Routes(Env{}) {
		ops := humaworkers.New(config(), []humaworkers.Route{route}).Operations()
		if len(ops) != 1 || route.OperationID == "" || ops[0].OperationID != route.OperationID || ops[0].Method != route.Method || ops[0].Path != route.Path {
			t.Errorf("route %s %s (%q) registers %+v", route.Method, route.Path, route.OperationID, ops)
		}
	}
}
