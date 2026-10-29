package standard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRefuses: the standard test fails on an API that does not serve the standard, naming each thing it gets wrong.
// That it passes on one that does is shown by every project's check, which runs it on the project's own server.
func TestRefuses(t *testing.T) {
	// Its spec names another address, its 404 is plain text, and it has no MCP endpoint.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/openapi.json":
			w.Header().Set("content-type", "application/json")
			w.Write([]byte(`{"servers":[{"url":"https://elsewhere.invalid"}],"paths":{"/api/hello":{"get":{"operationId":"hello"}}}}`))
		case "/api/asyncapi.json":
			w.Header().Set("content-type", "application/json")
			w.Write([]byte(`{}`))
		default:
			w.Header().Set("content-type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}
	}))
	defer api.Close()
	err := Run(api.URL, func(string) {})
	if err == nil {
		t.Fatal("the standard test passed an API that does not serve the standard")
	}
	for _, want := range []string{"GET /api/openapi.json names this address", "an unknown path is 404, as problem details", "POST /api/mcp lists tools"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the failure does not name %q: %v", want, err)
		}
	}
}
