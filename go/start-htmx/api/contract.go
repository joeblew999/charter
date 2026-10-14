// Package api is the API, contract first, in Go. Every route is a Huma operation whose input and
// output are Go structs; the struct tags are the schema. From this one definition come the handlers'
// validation, the OpenAPI and AsyncAPI specs (spec.go), the MCP tools, and from the specs Fern's
// SDKs, CLI and docs. OperationID and Tags name the SDK methods, and Extensions carry Fern's
// x-fern-*. The `example` tags are what the SDKs' READMEs show (TestExamplesAreThereAndValid).
// Add a route to Routes, then: mise run spec.
//
// The pages (package pages) are not in the contract: they are HTML for browsers. They call the same
// handlers (Env.Post, Env.Recent, Env.Feed), so a message posted from a page or through the API is
// the same message, and every open page gets it.
package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/humaworkers"
)

// What the specs say about the API as a whole.
const (
	Title       = "charter-start-htmx"
	Version     = "1.0.0"
	Description = "charter-start-htmx: Huma contract (Go) -> OpenAPI -> Fern, and server-rendered pages (gsx, htmx 4)."
	LiveTitle   = "charter-start-htmx live"
)

// MaxBody is the longest message, in characters: the contract's maxLength below says the same.
const MaxBody = 280

// Message is the one resource.
type Message struct {
	ID        int64  `json:"id" example:"42"`
	Body      string `json:"body" example:"Hello, everyone"`
	CreatedAt string `json:"created_at" example:"2026-10-03 12:00:00"`
}

// Position is the message's place in the log: what a stream resumes after (go/follow).
func (m Message) Position() int64 { return m.ID }

type HelloOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello from charter-start-htmx"`
	}
}

type ListInput struct {
	Limit int32 `query:"limit" minimum:"1" maximum:"100" default:"20" example:"20"`
}

type ListOutput struct {
	Body struct {
		Data []Message `json:"data"`
	}
}

type CreateInput struct {
	Body struct {
		Body string `json:"body" minLength:"1" maxLength:"280" example:"Hello, everyone"`
	}
}

type MessageOutput struct {
	Body Message
}

// sdk is Fern's names for the SDK method: client.<group>.<method>() and `cli <group> <method>`.
func sdk(group, method string) map[string]any {
	return map[string]any{"x-fern-sdk-group-name": group, "x-fern-sdk-method-name": method}
}

// Routes is the contract with its implementation on env.
func Routes(env Env) []humaworkers.Route {
	return []humaworkers.Route{
		{Method: http.MethodGet, Path: "/api/hello", OperationID: "hello", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "hello", Method: http.MethodGet, Path: "/api/hello",
				Summary: "Say hello", Tags: []string{"meta"},
				Extensions: sdk("meta", "hello"),
			}, env.hello)
		}},
		{Method: http.MethodGet, Path: "/api/messages", OperationID: "listMessages", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "listMessages", Method: http.MethodGet, Path: "/api/messages",
				Summary: "The newest messages, newest first", Tags: []string{"messages"},
				Extensions: sdk("messages", "list"),
			}, env.list)
		}},
		{Method: http.MethodPost, Path: "/api/messages", OperationID: "createMessage", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "createMessage", Method: http.MethodPost, Path: "/api/messages",
				Summary:     "Post a message",
				Description: "Every open page shows it at once: the pages' stream (/messages/stream) sends it as HTML.",
				Tags:        []string{"messages"},
				Extensions:  sdk("messages", "create"),
			}, env.create)
		}},
	}
}
