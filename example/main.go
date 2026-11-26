// The example: one app that uses all of charter. Pages (structpages, gsx, gsxui, htmx 4) in three languages, an API
// from a contract with auth, limits, streams, MCP and SDKs, one live feed behind both, and a card for everything
// Cloudflare gives a Worker.
//
//	go tool charter dev       work on it: a save rebuilds and reloads → http://localhost:5173
//	go tool charter check && go tool charter test
//	go tool charter deploy    then: go tool charter test --deployed
package main

import (
	"embed"

	"github.com/joeblew999/charter/app"
	charter "github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/cloud"
	_ "github.com/joeblew999/charter/example/locales" // the app's languages (locales/*.toml)
	"github.com/joeblew999/charter/example/pages"
)

// Vite's manifest (charter build): which hashed file each entry in web/ became.
//
//go:embed dist/.vite/manifest.json
var dist embed.FS

func main() {
	// What the Worker does that is no request: a Cron Trigger, a queue's consumer, email (package cloud).
	cloud.Background()
	(&app.App{
		Name:  "charter-example",
		Dist:  dist,
		Pages: pages.Mount,
		API:   api.Handler(api.Env{Var: charter.Var}),
	}).Run()
}
