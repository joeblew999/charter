// Command showcase is the showcase API (../../showcase: every Fern feature we use, from a Go
// contract) as a server. Built with TinyGo for Wasm (mise run showcase-go:build) it runs on
// Cloudflare Workers, with worker.mjs as the Worker's entry; built for the host, workers.Serve
// starts a plain HTTP server on :9900 (or $PORT). transport carries the WebSocket in both places.
package main

import (
	"github.com/syumai/workers-go"

	"github.com/joeblew999/orpc-api/api-go/showcase"
	"github.com/joeblew999/orpc-api/api-go/transport"
)

func main() {
	workers.Serve(transport.Serve(showcase.Handler(env())))
}
