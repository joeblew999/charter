// The conformance API (api/: every Fern feature we use, from a Go
// contract) as a server. Built with TinyGo for Wasm (npm run build) it runs on
// Cloudflare Workers, with worker.mjs as the Worker's entry; built for the host, transport.Run
// starts a plain HTTP server on :9900 (or $PORT). transport carries the WebSocket in both places.
package main

import (
	"github.com/joeblew999/charter/go/conformance/api"
	"github.com/joeblew999/charter/go/lib/transport"
)

func main() {
	transport.Run(api.Handler(env()))
}
