// Command spec writes the conformance API's OpenAPI and AsyncAPI specs from its Go contract
// (../../api), offline, for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `npm run spec`. The same functions the
// server gives /api/openapi.json and /api/asyncapi.json with (api/spec.go, go/lib/serve). Either way
// it first fails if an operation cannot be the MCP tool it would be (Tools).
package main

import (
	"github.com/joeblew999/charter/go/conformance/api"
	"github.com/joeblew999/charter/go/specs"
)

func main() {
	spec := api.API(api.Env{})
	specs.Must(spec.Tools())
	specs.Main(spec.OpenAPI, spec.AsyncAPI)
}
