// Command api-go is the notes API as a Cloudflare Worker in Go: the contract and handlers are in
// api/, the bindings in platform_js.go. Built with TinyGo for Wasm (mise run api-go:build) it runs
// on Workers; built for the host, transport.Run starts a plain HTTP server on :9900 (or $PORT) with
// an in-memory store (platform_other.go). transport carries the WebSocket in both places.
package main

import (
	"github.com/joeblew999/orpc-api/api-go/api"
	"github.com/joeblew999/orpc-api/api-go/transport"
)

func main() {
	transport.Run(api.Handler(env()))
}
