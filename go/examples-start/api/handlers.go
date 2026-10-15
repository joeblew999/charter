package api

import (
	"context"
	"net/http"
)

// Env is what the platform supplies: bindings on Cloudflare (platform_js.go), the process's
// environment elsewhere (platform_other.go). Add what your handlers need, such as a store on the
// D1 binding DB.
type Env struct {
	Var func(name string) string
}

// Handler serves the contract on env the standard way (go/lib/serve): the operations under /api,
// the specs at /api/openapi.json and /api/asyncapi.json, the operations as MCP tools at /api/mcp.
func Handler(env Env) http.Handler {
	return API(env).Handler()
}

func (env Env) hello(context.Context, *struct{}) (*HelloOutput, error) {
	out := &HelloOutput{}
	out.Body.Message = "Hello from " + env.Var("APP_NAME")
	return out, nil
}
