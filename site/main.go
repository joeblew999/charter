// charter's own site: its docs, as a charter app. The docs are the Markdown of docs/; the pages that show them are
// charter's docs screen (pages/screens), the same every app's docs get.
//
//	go tool charter dev       work on it: a save rebuilds and reloads
//	go tool charter check && go tool charter test
//	go tool charter deploy    then: go tool charter test --deployed
package main

import (
	"embed"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/seo"
	"github.com/joeblew999/charter/site/docs"
	"github.com/joeblew999/charter/site/pages"
)

// Vite's manifest (charter build): which hashed file each entry in web/ became.
//
//go:embed dist/.vite/manifest.json
var dist embed.FS

func main() {
	(&app.App{
		Name:  "charter",
		Dist:  dist,
		Pages: pages.Mount,
		Docs: &app.Docs{
			Name:    "charter",
			Summary: "Go apps on Cloudflare Workers, one way: pages, an API from a contract, a live feed behind both, languages and search engines, in one Worker.",
			Pages:   docs.Pages,
			Repo:    pages.Repo,
			Dir:     "site",
			Agents:  "If you are a coding agent working on a charter app, work with its Claude Code plugins: the marketplace joeblew999/charter lists charter's own skill and those of gsx, gsxui and structpages, each from its owner's repository (in Claude Code: `/plugin marketplace add joeblew999/charter`, then `/plugin install charter@charter`, and the same for gsx, gsxui and structpages; an app made with charter enables all four in its .claude/settings.json, and `go tool charter install` installs them). The skills are the rules to work by; these docs are the reference they point to.",
			More: []seo.LLMsSection{{Name: "The example, running", Links: []seo.LLMsLink{
				{Title: "Notes", URL: pages.Example + "/", Note: "The live feed: a note written here shows at once on every open page."},
				{Title: "How it's built", URL: pages.Example + "/built/", Note: "Each thing Cloudflare gives a Worker doing one job in the app, on a page of its own."},
				{Title: "The API's contract", URL: pages.Example + "/api/openapi.json", Note: "OpenAPI, written from the Go contract."},
				{Title: "The source", URL: pages.Repo + "/tree/main/example", Note: "The example's code: what go tool charter create copies."},
			}}},
		},
	}).Run()
}
