// Package docs is the pages of an app's docs: the same for every app that has docs, charter's own site among them.
// An app mounts them in its tree and names its docs in its App:
//
//	type Pages struct {
//		home Home       `route:"GET /{$} Home"`
//		docs docs.Pages `route:"/docs Docs"`
//	}
//
//	(&app.App{Pages: pages.Mount, Docs: &app.Docs{Name: "My App", Pages: mydocs.Pages, Repo: "https://github.com/me/my-app"}}).Run()
//
// The pages are built from gsxui's components, imported as they are (github.com/gsxhq/gsxui/ui). charter build
// tells the app's Tailwind where their classes are.
package docs

import (
	"context"
	"net/http"
	"strings"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/intl"
)

// Pages is the docs as a group of pages: /docs/ is docs/index.md, /docs/<name> every other page.
type Pages struct {
	index Index `route:"GET /{$} Docs"`
	page  Page  `route:"GET /{slug} Doc"`
}

// Index is docs/index.md.
type Index struct{}

// Page is every other page of the docs.
type Page struct{}

// View is what a page of the docs shows: the page, and the docs it is one of.
type View struct {
	Docs *app.Docs
	Doc  app.Doc
}

func (Index) Props(docs *app.Docs) (View, error) { return view(docs, "") }

func (Page) Props(r *http.Request, docs *app.Docs) (View, error) {
	return view(docs, r.PathValue("slug"))
}

func view(docs *app.Docs, slug string) (View, error) {
	doc, ok := docs.Find(slug)
	if !ok {
		return View{}, app.NotFound("no such page of the docs")
	}
	return View{docs, doc}, nil
}

// link is a docs page's address in the reader's language.
func link(ctx context.Context, d app.Doc) string { return intl.URL(ctx, d.Path()) }

// home is the app's front page, in the reader's language.
func home(ctx context.Context) string { return intl.URL(ctx, "/") }

// repo is the repository's name as a reader knows it: owner/name.
func repo(url string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimRight(url, "/"), "https://"), "github.com/")
}

// prose styles the HTML the Markdown became: Tailwind classes on the elements inside the article, in gsxui's tokens,
// since the HTML itself carries none.
const prose = "flex max-w-3xl flex-col gap-4 leading-7 " +
	"[&_h1]:text-3xl [&_h1]:font-semibold [&_h1]:tracking-tight " +
	"[&_h2]:mt-6 [&_h2]:scroll-mt-4 [&_h2]:border-b [&_h2]:pb-2 [&_h2]:text-xl [&_h2]:font-semibold " +
	"[&_h3]:mt-2 [&_h3]:scroll-mt-4 [&_h3]:text-lg [&_h3]:font-medium " +
	"[&_a]:underline [&_a]:underline-offset-4 " +
	"[&_ul]:list-disc [&_ul]:space-y-1 [&_ul]:ps-6 [&_ol]:list-decimal [&_ol]:space-y-1 [&_ol]:ps-6 " +
	"[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-sm " +
	"[&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:bg-muted [&_pre]:p-4 [&_pre_code]:bg-transparent [&_pre_code]:p-0 " +
	"[&_table]:w-full [&_table]:text-sm [&_th]:border-b [&_th]:px-3 [&_th]:py-2 [&_th]:text-start [&_th]:font-medium " +
	"[&_td]:border-b [&_td]:px-3 [&_td]:py-2 [&_td]:align-top"

func tocClass(depth int) string {
	if depth > 2 {
		return "ps-3"
	}
	return ""
}

func linkClass(current bool) string {
	if current {
		return "rounded-md bg-muted px-3 py-1.5 text-sm font-medium"
	}
	return "rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
}
