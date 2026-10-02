// Command showcase-spec writes the showcase API's OpenAPI and AsyncAPI specs from its Go contract
// (../../showcase), offline, for Fern:
//
//	go run ./cmd/showcase-spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `mise run showcase:go:spec`. The same functions the
// server gives /openapi.json and /asyncapi.json with (showcase/spec.go).
package main

import (
	"github.com/joeblew999/orpc-api/api/go/showcase"
	"github.com/joeblew999/orpc-api/go/specfile"
)

func main() {
	specfile.Main(showcase.OpenAPI, showcase.AsyncAPI)
}
