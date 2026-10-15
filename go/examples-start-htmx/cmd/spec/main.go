// Command spec writes the API's OpenAPI and AsyncAPI specs from the Go contract, offline (no Worker
// needed), for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `npm run spec`. The same functions the Worker
// serves /api/openapi.json and /api/asyncapi.json with (api/spec.go, go/lib/serve).
//
// Either way it first fails on what the specs don't show (specs.Problems): a route that does not name
// its operation, an operation that cannot be its MCP tool, an example missing or invalid.
package main

import (
	"github.com/joeblew999/charter/go/examples-start-htmx/api"
	"github.com/joeblew999/charter/go/lib/specs"
)

func main() {
	spec := api.API(api.Env{})
	specs.Must(specs.Problems(spec))
	specs.Main(spec.OpenAPI, spec.AsyncAPI)
}
