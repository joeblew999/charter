// Command spec writes the conformance API's OpenAPI and AsyncAPI specs from its Go contract
// (../../api), offline, for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `npm run spec`. The same functions the
// server gives /openapi.json and /asyncapi.json with (api/spec.go). Either way it first fails if an
// operation cannot be the MCP tool it would be (api.Tools).
package main

import (
	"github.com/joeblew999/charter/go/conformance/api"
	"github.com/joeblew999/charter/go/specfile"
)

func main() {
	specfile.Must(api.Tools())
	specfile.Main(api.OpenAPI, api.AsyncAPI)
}
