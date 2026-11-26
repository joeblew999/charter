// Package app runs a Go app as a Cloudflare Worker: its pages, its API, its browser files, its languages and its
// answers to search engines, in one value.
//
//	//go:embed dist/.vite/manifest.json
//	var dist embed.FS
//
//	func main() {
//		(&app.App{Name: "My App", Dist: dist, Pages: pages.Mount, API: api.Handler()}).Run()
//	}
//
// and in the layout:
//
//	<html { app.HTML(ctx)... }>
//		<head>
//			<app.Head title={title} description={description}/>
//		</head>
//
// What that gives:
//
//   - Pages (github.com/jackielii/structpages): Pages is the tree of them, each page a type with its route in the
//     field's tag and a Page component. The tree is the one source of routes: links are { About{} |> url } (URL), and
//     the pages search engines may index are the tree's GET routes without parameters.
//   - Everything that is no page goes to API.
//   - Vite (github.com/gsxhq/vite): with charter dev a page loads web/main.js from Vite's dev server and reloads on a
//     save; built, it loads the hashed files of dist/, which Workers Static Assets serve.
//   - Languages (intl): a URL per language (/about, /de/about), <html lang dir>, the canonical URL and every
//     language's alternate in <head>.
//   - Search engines (seo): /robots.txt, /sitemap.xml in every language, /llms.txt, noindex on what is no page and
//     on everything in a preview, https only, and one address when the Worker has a domain.
//
// The Worker's variables it reads (wrangler.jsonc's vars): APP_DOMAIN, the canonical host; APP_PREVIEW, set to
// anything where the site must not be indexed.
//
// A Worker runs on Cloudflare: on wrangler dev locally (go tool charter dev), deployed remotely. Nothing here runs
// anywhere else; the package compiles for the machine it is written on so that editors and go vet need no setup.
package app

import (
	"bytes"
	"context"
	"io/fs"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/gsxhq/gsx"
	"github.com/gsxhq/vite"
	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/seo"
)

// App is the app: what it is called, its pages, its API, and Vite's manifest.
type App struct {
	// Name is the site's name, which search results show beside its pages.
	Name string
	// Dist has Vite's manifest, dist/.vite/manifest.json (//go:embed dist/.vite/manifest.json).
	Dist fs.FS
	// Pages mounts the page tree, as structpages reads it: a struct whose fields are the pages, each with its route
	// in its tag. The app's own package has the call, so that structpages-lint finds the tree:
	//
	//	func Mount(mux structpages.Mux, options ...structpages.Option) (*structpages.StructPages, error) {
	//		return structpages.Mount(mux, Pages{}, "/", "My App", options...)
	//	}
	//
	// nil: an app without pages.
	Pages func(mux structpages.Mux, options ...structpages.Option) (*structpages.StructPages, error)
	// Args are what the pages' Props and handlers ask for by type (structpages.WithArgs): the app's stores.
	Args []any
	// API answers everything that is not a page. nil: 404.
	API http.Handler
	// Entry is the Vite entry every page loads. Empty: web/main.js.
	Entry string
	// Icon is the site's icon. Empty: /favicon.svg (public/favicon.svg).
	Icon string
	// LLMs writes /llms.txt for the site's origin (seo.LLMsTxt writes the layout). nil: none.
	LLMs func(origin string) string

	once  sync.Once
	vite  *vite.Vite
	err   error
	pages []string // the pages search engines may index, each a path without a language
}

// Run serves the app as a Worker and never returns.
func (a *App) Run() { Run(a.Handler()) }

type key struct{}

// page is what Head and HTML read from the request's context.
type page struct {
	site *App
	r    *http.Request
}

// Handler is the app as one handler.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	if a.API != nil {
		mux.Handle("/", a.API)
	}
	if a.Pages != nil {
		// Every route of the tree passes here once, as it is mounted: that is where the list of pages comes from.
		each := func(next http.Handler, pn *structpages.PageNode) http.Handler {
			if path, ok := indexed(pn); ok {
				a.pages = append(a.pages, path)
			}
			// A page goes out whole; an action or a stream writes as it goes.
			if _, isPage := pn.Components["Page"]; isPage {
				return whole(next)
			}
			return next
		}
		if _, err := a.Pages(mux, structpages.WithArgs(a.Args...),
			structpages.WithMiddlewares(each), structpages.WithTargetSelector(structpages.HTMXv4RenderTarget)); err != nil {
			panic(err) // the page tree is code: the first run shows a mistake in it
		}
	}
	isPage := func(path string) bool { return slices.Contains(a.pages, path) }
	// A page's path as search engines ask for it has its language and its query: /de/about?x=1.
	indexable := func(pathAndQuery string) bool {
		path, _, _ := strings.Cut(pathAndQuery, "?")
		return isPage(intl.Unprefixed(path))
	}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, page{a, r})))
	})
	h = intl.Site{
		IsPage:             isPage,
		ConnectionTimeZone: func(r *http.Request) string { return CFOf(r).Timezone },
		Origin:             Origin,
	}.Handler(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A path that touches no binding: what gsx dev probes after each rebuild, what a Go runtime answers as it
		// starts warm, what a health check asks.
		if r.URL.Path == "/healthz" {
			w.Write([]byte("ok"))
			return
		}
		// One address per page: a deployed site asked over http answers with the https one. (A zone can do this
		// itself; workers.dev does not.)
		if origin := Origin(r); strings.HasPrefix(origin, "http://") && !local(r) {
			http.Redirect(w, r, "https://"+strings.TrimPrefix(origin, "http://")+r.URL.RequestURI(), http.StatusMovedPermanently)
			return
		}
		v, err := a.assets()
		if err != nil {
			http.Error(w, "app: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Read per request: on Workers a variable is the request's environment's.
		seo.Site{
			Host:    cloud.Var("APP_DOMAIN"),
			Sitemap: func(origin string) string { return intl.Sitemap(origin, a.pages) },
			LLMs:    a.LLMs,
			IsPage:  indexable,
			Preview: cloud.Var("APP_PREVIEW") != "",
			Origin:  Origin,
		}.Handler(v.Middleware(h)).ServeHTTP(w, r)
	})
}

// indexed is the path of a route that is a page search engines may index: a GET without parameters whose type has a
// Page component, and does not say Noindex.
func indexed(pn *structpages.PageNode) (path string, ok bool) {
	if pn.Method != "" && pn.Method != http.MethodGet {
		return "", false
	}
	if _, has := pn.Components["Page"]; !has {
		return "", false
	}
	path = strings.TrimSuffix(pn.FullRoute(), "{$}")
	if strings.Contains(path, "{") {
		return "", false
	}
	if n, is := pn.Value.Interface().(interface{ Noindex() bool }); is && n.Noindex() {
		return "", false
	}
	if path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	return path, true
}

// whole sends a page with its length, not in chunks: htmx's history restore wants that on Workers.
func whole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := &buffered{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(b, r)
		w.Header().Set("Content-Length", strconv.Itoa(b.body.Len()))
		w.WriteHeader(b.status)
		w.Write(b.body.Bytes())
	})
}

type buffered struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (b *buffered) WriteHeader(status int)      { b.status = status }
func (b *buffered) Write(p []byte) (int, error) { return b.body.Write(p) }

// URL is a page's address, from its route in the tree, in the request's language: gsx.toml's url filter, written
// href={ About{} |> url }. A page with a Query method ("?topic=x") has it added.
func URL(ctx context.Context, page any) (string, error) {
	u, err := structpages.URLFor(ctx, page)
	if err != nil {
		return "", err
	}
	if q, ok := page.(interface{ Query() string }); ok {
		u += q.Query()
	}
	return intl.URL(ctx, u), nil
}

// assets is Vite, made once: under charter dev (VITE_DEV_URL, a variable of the Worker there) the dev server's
// URLs, else the manifest's.
func (a *App) assets() (*vite.Vite, error) {
	a.once.Do(func() {
		dist := a.Dist
		if dist == nil {
			dist = os.DirFS(".")
		}
		a.vite, a.err = vite.New(vite.Config{DevURL: cloud.Var("VITE_DEV_URL"), DevBase: "/__vite/", Dist: dist, DistDir: "dist", StaticURL: "/"})
	})
	return a.vite, a.err
}

func (a *App) icon() string {
	if a.Icon == "" {
		return "/favicon.svg"
	}
	return a.Icon
}

func (a *App) entry() string {
	if a.Entry == "" {
		return "web/main.js"
	}
	return a.Entry
}

// HTML is <html>'s attributes: the page's language and direction (<html { HTML(ctx)... }>).
func HTML(ctx context.Context) gsx.Attrs { return intl.HTML(ctx) }

// head is what Head needs of the request.
type head struct {
	Name, Icon, Home, Lang, Page, Entry string
	Assets                              vite.Bundle
}

func headOf(ctx context.Context, entry string) head {
	p, _ := ctx.Value(key{}).(page)
	if p.site == nil {
		return head{Lang: "en"}
	}
	h := head{Name: p.site.Name, Icon: p.site.icon(), Entry: entry}
	if h.Entry == "" {
		h.Entry = p.site.entry()
	}
	if v := vite.FromContext(ctx); v != nil {
		h.Assets = v.Entry(h.Entry)
	}
	h.Page = intl.Unprefixed(p.r.URL.Path)
	h.Lang = intl.Current(ctx).ID()
	if h.Page == "/" {
		h.Home = Origin(p.r) + p.r.URL.Path
	}
	return h
}

// local says the request was made to this machine (wrangler dev), where there is no https to move to.
func local(r *http.Request) bool {
	host := r.URL.Hostname()
	if host == "" {
		host, _, _ = strings.Cut(r.Host, ":")
	}
	return host == "localhost" || net.ParseIP(strings.Trim(host, "[]")).IsLoopback()
}
