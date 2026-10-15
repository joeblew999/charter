// Command spec writes the API's OpenAPI and AsyncAPI specs from the Go contract, offline (no Worker
// needed), for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `npm run spec`. The same functions the Worker
// serves /api/openapi.json and /api/asyncapi.json with (api/spec.go, go/lib/serve).
//
// Either way it first fails if an operation cannot be the MCP tool it would be (mcp.Check): the
// specs don't show that, and otherwise the first client to list the tools finds out.
package main

import (
	"github.com/joeblew999/charter/go/examples-start-htmx/api"
	"github.com/joeblew999/charter/go/specs"
)

func main() {
	spec := api.API(api.Env{})
	specs.Must(spec.Tools())
	specs.Main(spec.OpenAPI, spec.AsyncAPI)
}
