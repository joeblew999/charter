package pages

import (
	"net/http"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/docs"
	"github.com/joeblew999/charter/seo"
)

// Docs are the app's docs: the Markdown files of docs/, which charter build writes as docs.Pages. Each is a page of
// its own, at /docs/ and /docs/<its file's name>, in the sitemap and in /llms.txt.
type Docs struct {
	index DocsIndex `route:"GET /{$} Docs"`
	page  DocPage   `route:"GET /{slug} Doc"`
}

// DocsIndex is docs/index.md, at /docs/.
type DocsIndex struct{}

func (DocsIndex) Props() (app.Doc, error) { return doc("") }

// DocPage is every other page of the docs.
type DocPage struct{}

func (DocPage) Props(r *http.Request) (app.Doc, error) { return doc(r.PathValue("slug")) }

// Paths are the pages' addresses: what puts each in the sitemap, as a page search engines may index.
func (DocPage) Paths() []string { return app.DocPaths(docs.Pages) }

func doc(slug string) (app.Doc, error) {
	d, ok := app.FindDoc(docs.Pages, slug)
	if !ok {
		return app.Doc{}, app.NotFound("no such page of the docs")
	}
	return d, nil
}

// LLMs is /llms.txt, for a language model (https://llmstxt.org): what the app is, its pages, and its docs, each with
// what it is about. It is app.App's LLMs.
func LLMs(origin string, url func(page any) string) string {
	read := make([]seo.LLMsLink, 0, len(docs.Pages))
	for _, d := range docs.Pages {
		read = append(read, seo.LLMsLink{Title: d.Title, URL: origin + d.Path(), Note: d.Description})
	}
	// The pages' addresses come from the tree, as every link does.
	home, bindings := url(Home{}), url(Bindings{})
	return seo.LLMsTxt("charter", "Go apps on Cloudflare Workers, one way: pages, an API from a contract, a live feed behind both, languages and search engines, in one Worker.",
		"This site is a charter app, and its source is https://github.com/joeblew999/charter/tree/main/example. A coding agent gets charter's skill, with gsx's, gsxui's and structpages', from the plugin marketplace joeblew999/charter.",
		seo.LLMsSection{Name: "Docs", Links: read},
		seo.LLMsSection{Name: "Live", Links: []seo.LLMsLink{
			{Title: "Notes", URL: origin + home, Note: "The live feed: a note written here shows at once on every open page."},
			{Title: "Bindings", URL: origin + bindings, Note: "Everything Cloudflare gives a Worker, one card each, used through workers-go."},
			{Title: "The API's contract", URL: origin + "/api/openapi.json", Note: "OpenAPI, written from the Go contract."},
		}})
}
