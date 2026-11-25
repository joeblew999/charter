// An API and its pages as one Cloudflare Worker in Go: the contract and handlers in api/, the server-rendered pages
// (gsx and htmx 4, live over SSE) in pages/, the browser's files in web/ (Vite).
//
//	go tool charter dev      gsx dev: a save rebuilds and reloads
//	go tool charter preview  wrangler dev: the Worker as Cloudflare runs it
//	go tool charter check    everything before a push
package main

import (
	"embed"
	"log"
	"os"

	"github.com/gsxhq/vite"

	"github.com/joeblew999/charter/cf/bindings"
	"github.com/joeblew999/charter/cf/d1"
	"github.com/joeblew999/charter/cf/hub"
	"github.com/joeblew999/charter/cf/transport"
	"github.com/joeblew999/charter/examples/site/api"
	"github.com/joeblew999/charter/examples/site/pages"
)

// Vite's manifest (charter build): which hashed file each entry in web/ became. The files themselves are Workers
// Static Assets (wrangler.jsonc's assets).
//
//go:embed dist/.vite/manifest.json
var dist embed.FS

func main() {
	// VITE_DEV_URL is set by gsx dev: the browser's files come from Vite's dev server. Without it, from dist.
	v, err := vite.New(vite.Config{DevURL: os.Getenv("VITE_DEV_URL"), DevBase: "/__vite/", Dist: dist, DistDir: "dist", StaticURL: "/"})
	if err != nil {
		log.Fatal(err)
	}
	// The bindings of wrangler.jsonc, through charter: each has a stand-in off Workers (go run ., go test), so there
	// is no file per platform. DB is D1, the log; HUB the Durable Object that wakes every open page.
	env := api.Env{
		Var: bindings.Var,
		Store: func() (api.Store, error) {
			db, err := d1.Open("DB")
			if err != nil {
				return nil, err
			}
			return api.D1Store{DB: db}, nil
		},
		Hub: func() (api.Hub, error) { return hub.DurableObject[api.Message]("HUB", "messages") },
	}
	transport.Run(v.Middleware(pages.Handler(env, api.Handler(env))))
}
