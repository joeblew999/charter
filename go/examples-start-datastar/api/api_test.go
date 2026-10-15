package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// inMemory is the API as `go run .` has it: one store and hub.
func inMemory() Env {
	memory := &MemStore{}
	return Env{
		Var:   func(string) string { return "test" },
		Store: func() (Store, error) { return memory, nil },
		Hub:   func() (Hub, error) { return memory, nil },
	}
}

// A real HTTP server over the handlers, as `go run .` serves them.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(Handler(inMemory()))
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, method, url, body string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(out), res.Header
}

func TestHello(t *testing.T) {
	srv := server(t)
	status, body, _ := do(t, "GET", srv.URL+"/api/hello", "")
	if status != 200 || strings.TrimSpace(body) != `{"message":"Hello from test"}` {
		t.Fatalf("HTTP %d %s", status, body)
	}
}

func TestCreateAndList(t *testing.T) {
	srv := server(t)
	for _, body := range []string{"first", "  second  "} {
		if status, out, _ := do(t, "POST", srv.URL+"/api/messages", `{"body":"`+body+`"}`); status != 200 {
			t.Fatalf("POST %q: HTTP %d %s", body, status, out)
		}
	}
	status, body, _ := do(t, "GET", srv.URL+"/api/messages?limit=10", "")
	var list ListOutput
	if err := json.Unmarshal([]byte(body), &list.Body); err != nil || status != 200 {
		t.Fatalf("HTTP %d %s", status, body)
	}
	if len(list.Body.Data) != 2 || list.Body.Data[0].Body != "second" || list.Body.Data[1].ID != 1 {
		t.Fatalf("newest first, trimmed: %s", body)
	}
}

func TestAMessageIsOneTo280CharactersNotOnlySpaces(t *testing.T) {
	srv := server(t)
	for _, body := range []string{"", "   ", strings.Repeat("é", MaxBody+1)} {
		if status, out, _ := do(t, "POST", srv.URL+"/api/messages", `{"body":"`+body+`"}`); status != 422 {
			t.Errorf("POST %q: HTTP %d %s, want 422", body, status, out)
		}
	}
	if status, out, _ := do(t, "POST", srv.URL+"/api/messages", `{"body":"`+strings.Repeat("é", MaxBody)+`"}`); status != 200 {
		t.Errorf("%d characters: HTTP %d %s", MaxBody, status, out)
	}
}

func TestUnknownPathAndMethod(t *testing.T) {
	srv := server(t)
	if status, _, _ := do(t, "GET", srv.URL+"/api/nope", ""); status != 404 {
		t.Errorf("unknown path: HTTP %d, want 404", status)
	}
	if status, _, _ := do(t, "DELETE", srv.URL+"/api/hello", ""); status != 405 {
		t.Errorf("DELETE /api/hello: HTTP %d, want 405", status)
	}
}

func TestTheWorkerServesBothSpecsWithItsOriginAsServer(t *testing.T) {
	srv := server(t)
	_, openapi, _ := do(t, "GET", srv.URL+"/api/openapi.json", "")
	_, asyncapi, _ := do(t, "GET", srv.URL+"/api/asyncapi.json", "")
	if !strings.Contains(openapi, `"servers":[{"url":"`+srv.URL+`"}]`) {
		t.Errorf("openapi servers: %s", openapi)
	}
	if !strings.Contains(asyncapi, `"host":"`+strings.TrimPrefix(srv.URL, "http://")+`","protocol":"ws"`) {
		t.Errorf("asyncapi servers: %s", asyncapi)
	}
}

// The contract's operations are MCP tools: a tool call runs the same handler as the REST route.
func TestMCPCallsRunTheSameOperationsAsREST(t *testing.T) {
	srv := server(t)
	status, body, _ := do(t, "POST", srv.URL+"/api/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"hello","arguments":{}}}`)
	var answer struct {
		Result struct {
			Content []struct{ Text string }
			IsError bool
		}
	}
	if err := json.Unmarshal([]byte(body), &answer); err != nil || status != 200 {
		t.Fatalf("HTTP %d %s", status, body)
	}
	_, rest, _ := do(t, "GET", srv.URL+"/api/hello", "")
	if answer.Result.IsError || len(answer.Result.Content) != 1 || answer.Result.Content[0].Text != strings.TrimSpace(rest) {
		t.Fatalf("hello over MCP: %s\nover REST: %s", body, rest)
	}
}
