package api

import (
	"context"
	"net/http"

	"github.com/joeblew999/charter/go/humamcp"
	"github.com/joeblew999/charter/go/humaworkers"
)

// Env is what the platform supplies: bindings on Cloudflare (platform_js.go), the process's
// environment elsewhere (platform_other.go). Add what your handlers need, such as a store on the
// D1 binding DB.
type Env struct {
	Var func(name string) string
}

// Handler serves the contract on env, plus the two specs with the request's origin as their server,
// plus the contract as MCP tools (/api/mcp: a tool call runs the same operation as the REST route).
func Handler(env Env) http.Handler {
	routes := humaworkers.New(config(), Routes(env))
	mcp := humamcp.Handler(routes)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var spec func(server string) ([]byte, error)
		switch r.URL.Path {
		case "/api/openapi.json":
			spec = OpenAPI
		case "/api/asyncapi.json":
			spec = AsyncAPI
		case "/api/mcp":
			mcp.ServeHTTP(w, r)
			return
		default:
			routes.ServeHTTP(w, r)
			return
		}
		body, err := spec(origin(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
}

// origin is the request's scheme and host: absolute in r.URL on Workers, from Host on net/http.
func origin(r *http.Request) string {
	if r.URL.Scheme != "" && r.URL.Host != "" {
		return r.URL.Scheme + "://" + r.URL.Host
	}
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

func (env Env) hello(context.Context, *struct{}) (*HelloOutput, error) {
	out := &HelloOutput{}
	out.Body.Message = "Hello from " + env.Var("APP_NAME")
	return out, nil
}
