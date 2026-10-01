// Command spec writes the API's OpenAPI and AsyncAPI specs from the Go contract, offline (no Worker
// needed), for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `mise run api-go:spec`. The same functions the Worker
// serves /api/openapi.json and /api/asyncapi.json with (api/spec.go).
package main

import (
	"github.com/joeblew999/orpc-api/api-go/api"
	"github.com/joeblew999/orpc-api/api-go/specfile"
)

func main() {
	specfile.Main(api.OpenAPI, api.AsyncAPI)
}
