package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A real HTTP server over the handlers, as `go run .` serves them.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(Handler(Env{Var: func(string) string { return "test" }}))
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
