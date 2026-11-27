// The example: one app that uses all of charter. Pages (structpages, gsx, gsxui, htmx 4) in three languages, an API
// from a contract with auth, limits, streams, MCP and SDKs, one live feed behind both, and a card for everything
// Cloudflare gives a Worker.
//
//	go tool charter dev       work on it: a save rebuilds and reloads → http://localhost:5173
//	go tool charter check && go tool charter test
//	go tool charter deploy    then: go tool charter test --deployed
package main

import (
	"context"
	"embed"

	"github.com/joeblew999/charter/app"
	charter "github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/built"
	"github.com/joeblew999/charter/example/docs"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
)

// Vite's manifest (charter build): which hashed file each entry in web/ became.
//
//go:embed dist/.vite/manifest.json
var dist embed.FS

func main() {
	// What the Worker does that is no request: a queue's consumer, a Cron Trigger's task, email (package built).
	built.Background()
	(&app.App{
		Name:  "charter-example",
		Dist:  dist,
		Pages: pages.Mount,
		// The app's own docs (docs/*.md), shown by charter's docs pages, which the tree mounts at /docs.
		Docs: &app.Docs{
			Name:    "charter-example",
			Summary: "charter's example: notes that are live on every open page, an API from a contract, and a card for everything Cloudflare gives a Worker.",
			Pages:   docs.Pages,
			Repo:    "https://github.com/joeblew999/charter",
			Dir:     "example",
			Agents:  "A coding agent working on this app uses the Claude Code plugins of the marketplace joeblew999/charter: charter, gsx, gsxui and structpages. The app's .claude/settings.json enables them.",
		},
		// Who may do what, for the pages as for the API (api.Auth): a page or a group says what it needs with a Needs
		// method, and whoever is not signed in is sent to the sign-in screen.
		Auth:   api.Auth,
		SignIn: screens.SignInPage{},
		// The app's own lines on the admin screen's overview: asked when that page is.
		Facts: []app.Fact{
			{Label: locales.FactsNotes, Value: func(ctx context.Context) (any, error) {
				report, err := api.Report(ctx)
				return report.Notes, err
			}},
			{Label: locales.FactsWriters, Value: func(ctx context.Context) (any, error) {
				report, err := api.Report(ctx)
				return report.Authors, err
			}},
		},
		Args: []any{api.Auth},
		// What a page asks for by type, made from the request: the notes' store (D1), and who is asking.
		RequestArgs: []any{api.OpenStore, api.Auth.Caller},
		API:         api.Handler(api.Env{Var: charter.Var}),
	}).Run()
}
