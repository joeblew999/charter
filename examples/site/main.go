// An API and its pages as one Cloudflare Worker in Go: the contract and handlers in api/, the server-rendered pages
// (gsx, gsxui and htmx 4, live over SSE) in pages/, their text in locales/, the browser's files in web/ (Vite).
//
//	go tool charter dev      a save rebuilds and reloads, on wrangler; leave it running
//	go tool charter check && go tool charter test    before a push
package main

import (
	"embed"

	"github.com/joeblew999/charter/bindings"
	"github.com/joeblew999/charter/d1"
	"github.com/joeblew999/charter/examples/site/api"
	"github.com/joeblew999/charter/examples/site/cloud"
	_ "github.com/joeblew999/charter/examples/site/locales" // the app's languages (charter.toml's [intl])
	"github.com/joeblew999/charter/examples/site/pages"
	"github.com/joeblew999/charter/gsxapp"
	"github.com/joeblew999/charter/live"
	"github.com/joeblew999/charter/transport"
)

// Vite's manifest (charter build): which hashed file each entry in web/ became.
//
//go:embed dist/.vite/manifest.json
var dist embed.FS

func main() {
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
		Hub: func() (api.Hub, error) { return live.DurableObject[api.Message]("HUB", "messages") },
	}
	// The site: Vite's files, a URL per language, robots.txt and the sitemap, around the pages and the API.
	s := &gsxapp.App{Name: "charter-site", Dist: dist, Pages: []string{"/", "/cloud"}}
	// What the Worker does that is no request: a Cron Trigger, a queue's consumer, email (package cloud).
	cloud.Background()
	transport.Run(s.Handler(pages.Handler(env, api.Handler(env))))
}
