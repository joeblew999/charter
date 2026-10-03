// The API and its pages as one Cloudflare Worker in Go: the contract and handlers are in api/, the
// server-rendered pages (gsx and htmx 4) in pages/, the bindings in platform_js.go. Built with
// TinyGo for Wasm (mise run build) it runs on Workers; built for the host, transport.Run starts a
// plain HTTP server on :9900 (or $PORT) with an in-memory store (platform_other.go).
package main

import (
	"github.com/joeblew999/charter/examples/start-htmx/api"
	"github.com/joeblew999/charter/examples/start-htmx/pages"
	"github.com/joeblew999/charter/go/transport"
)

func main() {
	env := env()
	transport.Run(pages.Handler(env, api.Handler(env)))
}
