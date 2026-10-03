// Package api is the API, contract first, in Go. Every route is a Huma operation whose input and
// output are Go structs; the struct tags are the schema. From this one definition come the handlers'
// validation, the OpenAPI and AsyncAPI specs (spec.go), the MCP tools, and from the specs Fern's
// SDKs, CLI and docs. OperationID and Tags name the SDK methods, and Extensions carry Fern's
// x-fern-*. The `example` tags are what the SDKs' READMEs show (TestExamplesAreThereAndValid).
// Add a route to Routes, then: mise run spec.
package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/humaworkers"
)

// What the specs say about the API as a whole.
const (
	Title       = "charter-start-go"
	Version     = "1.0.0"
	Description = "charter-start-go: Huma contract (Go) -> OpenAPI -> Fern."
	LiveTitle   = "charter-start-go live"
)

type HelloOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello from charter-start-go"`
	}
}

// Routes is the contract with its implementation on env.
func Routes(env Env) []humaworkers.Route {
	return []humaworkers.Route{
		{Method: http.MethodGet, Path: "/api/hello", OperationID: "hello", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "hello", Method: http.MethodGet, Path: "/api/hello",
				Summary: "Say hello", Tags: []string{"meta"},
				// Fern's names for the SDK method: client.meta.hello() and `cli meta hello`.
				Extensions: map[string]any{"x-fern-sdk-group-name": "meta", "x-fern-sdk-method-name": "hello"},
			}, env.hello)
		}},
	}
}
