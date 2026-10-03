package auth

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

type out struct{ Body string }

// An API with a public operation, one that takes the default scope (read) and one that needs write.
func testAPI(t *testing.T, secrets map[string]string) humatest.TestAPI {
	_, api := humatest.New(t)
	Scheme(api.OpenAPI(), "read")
	api.UseMiddleware(Middleware(api, func(name string) string { return secrets[name] },
		Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
		Token{Secret: "READ_TOKEN", Scopes: []string{"read"}},
	))
	handler := func(context.Context, *struct{}) (*out, error) { return &out{Body: "ok"}, nil }
	huma.Register(api, huma.Operation{OperationID: "hello", Method: http.MethodGet, Path: "/hello", Security: Public()}, handler)
	huma.Register(api, huma.Operation{OperationID: "list", Method: http.MethodGet, Path: "/notes"}, handler)
	huma.Register(api, huma.Operation{OperationID: "create", Method: http.MethodPost, Path: "/notes", Security: Needs("write")}, handler)
	return api
}

func TestScopes(t *testing.T) {
	api := testAPI(t, map[string]string{"READ_TOKEN": "r", "WRITE_TOKEN": "w"})
	for _, c := range []struct {
		method, path, token string
		want                int
	}{
		{"GET", "/hello", "", 200},
		{"GET", "/notes", "", 401},
		{"GET", "/notes", "wrong", 401},
		{"GET", "/notes", "r", 200},
		{"GET", "/notes", "w", 200},
		{"POST", "/notes", "", 401},
		{"POST", "/notes", "r", 403},
		{"POST", "/notes", "w", 200},
	} {
		var args []any
		if c.token != "" {
			args = append(args, "Authorization: Bearer "+c.token)
		}
		resp := api.Do(c.method, c.path, args...)
		if resp.Code != c.want {
			t.Errorf("%s %s with %q: %d, want %d: %s", c.method, c.path, c.token, resp.Code, c.want, resp.Body)
		}
		if c.want == 401 && resp.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s %s: a 401 without WWW-Authenticate: Bearer", c.method, c.path)
		}
	}
}

// Without its secrets, the API refuses every token: an unset secret matches nothing, not "".
func TestUnsetSecretsMatchNothing(t *testing.T) {
	api := testAPI(t, nil)
	if resp := api.Do("GET", "/notes", "Authorization: Bearer "); resp.Code != 401 {
		t.Errorf("an empty token on a Worker without secrets: %d, want 401", resp.Code)
	}
	if resp := api.Do("GET", "/hello"); resp.Code != 200 {
		t.Errorf("hello is public: %d", resp.Code)
	}
}

// The specs say what the server enforces: the scheme, the default, and each operation's scopes.
func TestTheSpecSaysSo(t *testing.T) {
	api := testAPI(t, nil)
	spec, err := api.OpenAPI().MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"securitySchemes":{"bearer":{"scheme":"bearer","type":"http"}}`, `"security":[{"bearer":["read"]}]`, `"security":[{"bearer":["write"]}]`, `"security":[]`} {
		if !strings.Contains(string(spec), want) {
			t.Errorf("the spec has no %s", want)
		}
	}
}
