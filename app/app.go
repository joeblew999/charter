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
//     field's tag and a Page component. The tree is the one source of routes: links are { About{} |> url }
//     (structpages.URLFor, in the reader's language), and
//     the pages search engines may index are the tree's GET routes without parameters, and the docs.
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
	"log"
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
	// Args are what the pages' Props and handlers ask for by type, the same for every request (structpages.WithArgs).
	Args []any
	// RequestArgs make what they ask for by type from the request, once for each request and only when asked
	// (structpages.WithRequestArgs): func(*http.Request) (T, error). A binding belongs to the request, so a store
	// on D1 is one of these, and a page's Props declares the store instead of opening it.
	RequestArgs []any
	// API answers everything that is not a page. nil: 404.
	API http.Handler
	// Docs are the app's docs (docs/*.md, as charter build wrote them), for an app that has them: see Docs.
	Docs *Docs
	// Entry is the Vite entry every page loads. Empty: web/main.js.
	Entry string
	// Icon is the site's icon. Empty: /favicon.svg (public/favicon.svg).
	Icon string
	// LLMs writes /llms.txt for the site's origin (seo.LLMsTxt writes the layout); url is a page's address, from
	// the tree. nil: none.
	LLMs func(origin string, url func(page any) string) string

	once  sync.Once
	vite  *vite.Vite
	err   error
	pages []string // the pages search engines may index, each a path without a language
	site  intl.Site
	tree  *structpages.StructPages
}

// build names the build, which charter build sets when it links the Wasm: charter dev asks /healthz which build is
// answering.
var build string

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
	// The tree is mounted once for each language, each told its language's prefix (structpages.WithURLPrefix): a link
	// from the tree ({ About{} |> url }, structpages.URLFor) is then in the reader's language by itself, a form's
	// target and a stream too, and structpages' own linter sees every one of them.
	byLanguage := map[*intl.Locale]http.Handler{}
	languages := intl.Shipped()
	if len(languages) == 0 {
		languages = intl.Locales{intl.Default()}
	}
	for i, language := range languages {
		mux := http.NewServeMux()
		if a.API != nil {
			mux.Handle("/", a.API)
		}
		if a.Pages != nil {
			tree, err := a.Pages(mux, append(a.Options(i == 0), structpages.WithURLPrefix(intl.Prefix(language)))...)
			if err != nil {
				panic(err) // the page tree is code: the first run shows a mistake in it
			}
			if language == intl.Default() || a.tree == nil {
				a.tree = tree
			}
		}
		byLanguage[language] = mux
	}
	if a.Docs != nil {
		if !slices.Contains(a.pages, "/docs/") {
			panic("app: the app has Docs and its tree no page at /docs/: mount the docs screen there (charter add screen:docs)")
		}
		a.pages = append(a.pages, a.Docs.paths()...)
	}
	isPage := func(path string) bool { return slices.Contains(a.pages, path) }
	// A page's path as search engines ask for it has its language and its query: /de/about?x=1.
	indexable := func(pathAndQuery string) bool {
		path, _, _ := strings.Cut(pathAndQuery, "?")
		return isPage(intl.Unprefixed(path))
	}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux, ok := byLanguage[intl.Current(r.Context())]
		if !ok {
			mux = byLanguage[intl.Default()]
		}
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, page{a, r})))
	})
	a.site.IsPage = isPage
	h = a.site.Handler(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A path that touches no binding: what gsx dev probes after each rebuild, what a Go runtime answers as it
		// starts warm, what a health check asks.
		if r.URL.Path == "/healthz" {
			if build != "" {
				w.Header().Set("X-Charter-Build", build)
			}
			w.Write([]byte("ok"))
			return
		}
		// One address per page: a deployed site asked over http answers with the https one. (A zone can do this
		// itself; workers.dev does not.)
		if origin := Origin(r); strings.HasPrefix(origin, "http://") && !local(r) {
			http.Redirect(w, r, "https://"+strings.TrimPrefix(origin, "http://")+r.URL.RequestURI(), http.StatusMovedPermanently)
			return
		}
		// What a language model reads of the docs: a page as Markdown, all of them as one file.
		if a.Docs != nil && a.Docs.text(w, r) {
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
			LLMs:    a.llms(),
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
	// A group's own page keeps its slash (/docs/): that is the address it answers at.
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

// Options are the tree's options as the app mounts it: what a test that mounts the tree itself passes, with its
// own RequestArgs after them, so that it is the same tree. collect: also note which routes are pages search engines
// may index (the app does, once).
//
//	pages.Mount(mux, append((&app.App{}).Options(false), structpages.WithRequestArgs(fake))...)
func (a *App) Options(collect bool) []structpages.Option {
	// Every route of the tree passes here once, as it is mounted: that is where the list of pages comes from.
	each := func(next http.Handler, pn *structpages.PageNode) http.Handler {
		if path, ok := indexed(pn); ok && collect {
			a.pages = append(a.pages, path)
		}
		// A page goes out whole; an action or a stream writes as it goes.
		if _, isPage := pn.Components["Page"]; isPage {
			return whole(next)
		}
		return next
	}
	// What the screens' routes ask for (the time zone and clock dialog): the site's languages, as it is served.
	a.site.ConnectionTimeZone = func(r *http.Request) string { return CFOf(r).Timezone }
	a.site.Origin = Origin
	args := append(slices.Clone(a.Args), a.site)
	if a.Docs != nil {
		// The docs' pages ask for them by type.
		args = append(args, a.Docs)
	}
	return []structpages.Option{
		structpages.WithArgs(args...), structpages.WithRequestArgs(a.RequestArgs...),
		structpages.WithErrorHandler(failed), structpages.WithMiddlewares(each),
		structpages.WithTargetSelector(structpages.HTMXv4RenderTarget),
		// A field of the tree that is no route is a mistake, said when the app starts.
		structpages.WithWarnEmptyRoute(func(pn *structpages.PageNode) {
			log.Printf("app: %s is in the page tree and has no Page, Props or ServeHTTP: it answers nothing", pn.Name)
		}),
	}
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
