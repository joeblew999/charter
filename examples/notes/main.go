// The notes API on Cloudflare Workers in Go, from a contract (api/contract.go): auth, SSE and WebSocket streams, MCP,
// rate limits, a D1 database, SDKs in several languages.
//
//	go tool charter dev      wrangler dev → http://localhost:8787/api/docs
//	go tool charter check && go tool charter test    before a push
//	go run .                 the same handler as a plain Go server, no Cloudflare
package main

import (
	"github.com/joeblew999/charter/bindings"
	"github.com/joeblew999/charter/d1"
	"github.com/joeblew999/charter/examples/notes/api"
	"github.com/joeblew999/charter/live"
	"github.com/joeblew999/charter/transport"
)

func main() {
	// The bindings of wrangler.jsonc, through charter: each has a stand-in off Workers (go run ., go test), so there
	// is no file per platform. DB is D1; HUB the Durable Object every stream of notes listens to.
	transport.Run(api.Handler(api.Env{
		Var: bindings.Var,
		Store: func() (api.Store, error) {
			db, err := d1.Open("DB")
			if err != nil {
				return nil, err
			}
			return api.D1Store{DB: db}, nil
		},
		Hub: func() (api.Hub, error) { return live.DurableObject[api.Note]("HUB", "notes") },
	}))
}
