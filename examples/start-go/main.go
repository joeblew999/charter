// The API as a Cloudflare Worker in Go: the contract and handlers are in api/, the bindings in
// platform_js.go. Built with TinyGo for Wasm (mise run build) it runs on Workers; built for the
// host, transport.Run starts a plain HTTP server on :9900 (or $PORT) (platform_other.go).
package main

import (
	"github.com/joeblew999/charter/examples/start-go/api"
	"github.com/joeblew999/charter/go/transport"
)

func main() {
	transport.Run(api.Handler(env()))
}
