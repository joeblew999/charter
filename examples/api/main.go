// The smallest API on Cloudflare Workers in Go: one route, from a contract (api/contract.go).
//
//	go tool charter dev      wrangler dev → http://localhost:8787/api/docs
//	go tool charter check    everything before a push
//	go run .                 the same handler as a plain Go server, no Cloudflare
package main

import (
	"github.com/joeblew999/charter/cf/bindings"
	"github.com/joeblew999/charter/cf/transport"
	"github.com/joeblew999/charter/examples/api/api"
)

func main() {
	// bindings.Var is the Worker's variable (wrangler.jsonc's vars); off Workers, the process's environment.
	transport.Run(api.Handler(api.Env{Var: bindings.Var}))
}
