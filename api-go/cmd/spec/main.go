// Command spec writes the API's OpenAPI and AsyncAPI specs from the Go contract, offline (no Worker
// needed), for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `mise run api-go:spec`. The same functions the Worker
// serves /api/openapi.json and /api/asyncapi.json with (api/spec.go).
//
// Either way it first fails if an operation cannot be the MCP tool it would be (humamcp.Check): the
// specs don't show that, and otherwise the first client to list the tools finds out.
package main

import (
	"github.com/joeblew999/orpc-api/api-go/api"
	"github.com/joeblew999/orpc-api/api-go/humamcp"
	"github.com/joeblew999/orpc-api/api-go/humaworkers"
	"github.com/joeblew999/orpc-api/api-go/specfile"
)

func main() {
	specfile.Must(humamcp.Check(humaworkers.New(humaworkers.Config(api.Title, api.Version), api.Routes(api.Env{}))))
	specfile.Main(api.OpenAPI, api.AsyncAPI)
}
