// Command spec writes the showcase API's OpenAPI and AsyncAPI specs from its Go contract
// (../../api), offline, for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `mise run spec`. The same functions the
// server gives /openapi.json and /asyncapi.json with (api/spec.go).
package main

import (
	"github.com/joeblew999/charter/examples/showcase-go/api"
	"github.com/joeblew999/charter/go/specfile"
)

func main() {
	specfile.Main(api.OpenAPI, api.AsyncAPI)
}
