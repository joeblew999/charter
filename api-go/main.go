// Command api-go is the notes API as a Cloudflare Worker in Go: the contract and handlers are in
// api/, the bindings in platform_js.go. Built with TinyGo for Wasm (mise run api-go:build) it runs
// on Workers; built for the host, workers.Serve starts a plain HTTP server on :9900 (or $PORT) with
// an in-memory store, to try the handlers without workerd (no WebSocket there).
package main

import (
	"github.com/syumai/workers-go"

	"github.com/joeblew999/orpc-api/api-go/api"
)

func main() {
	workers.Serve(serve(api.Handler(env())))
}
