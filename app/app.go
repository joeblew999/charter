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
	"net/url"
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
	// Auth is who may do what, for an app with people or programs to tell apart: the same one the API's operations
	// are checked by (api.Auth). A page, or a group of the tree, says what it needs as an operation of the contract
	// does, with a method on its type:
	//
	//	func (Account) Needs() []app.Scope { return []app.Scope{api.Write} }   // nil: anyone who is signed in
	//
	// and every route at or under it is checked before it runs. nil: an app where every page is everyone's.
	Auth Auth
	// Facts are lines the app adds to the admin screen's overview (charter add screen:admin): what an admin wants to
	// know of the app itself, each asked when the page is.
	Facts []Fact
	// SignIn is the page someone who is not signed in is sent to when a page needs them to be (the sign-in
	// screen: screens.SignIn{}), with where they were going in ?return=. nil: such a request is a 401.
	SignIn any
	// Entry is the Vite entry every page loads. Empty: web/main.js.
	Entry string
	// Icon is the site's icon. Empty: /favicon.svg (public/favicon.svg).
	Icon string
	// LLMs writes /llms.txt for the site's origin (seo.LLMsTxt writes the layout); url is a page's address, from
	// the tree. nil: none.
	LLMs func(origin string, url func(page any) string) string

	once   sync.Once
	vite   *vite.Vite
	err    error
	pages  []string        // the sitemap: the pages without parameters search engines may index, each a path without a language
	routes map[string]bool // the tree's pages, by the pattern the mux has them under: whether search engines may index them
	shared map[string]bool // those of them that are the same for every reader (shared.go)
	site   intl.Site
	tree   *structpages.StructPages
}

// Seed registers what fills the app's database for a developer: the rows a new checkout starts with (charter run
// db:seed; charter dev, on a database it has just made). It is written in db/seed.go, a file that is built only for
// that (//go:build seed), with the app's own typed queries, and runs in a Worker started on this machine, on the
// real database binding: a seed is never part of a Worker that is deployed. A deployed database is filled by the
// same seed, from this machine (charter run db:seed --deployed <worker>): SeedIsDeployed says so.
//
//	//go:build seed
//
//	func init() { app.Seed(env.DB, func(ctx context.Context, conn cloud.DB) error { … }) }
func Seed(binding cloud.Database, fill func(ctx context.Context, db cloud.DB) error) {
	seeds = append(seeds, seed{binding, fill})
}

type seed struct {
	binding cloud.Database
	fill    func(ctx context.Context, db cloud.DB) error
}

var seeds []seed

// InvitePath is the address of an invitation's link, with its code: ?code=…
const InvitePath = "/__charter/invite"

// SeedPath is where charter asks a Worker built with its seed to run it.
const SeedPath = "/__charter/seed"

// SeedDeployed is the header charter sends when the database the seed fills is a deployed one (charter run
// db:seed --deployed): the Worker still runs on this machine, with that database for its binding.
const SeedDeployed = "X-Charter-Seed-Deployed"

type seedDeployed struct{}

// SeedIsDeployed says the seed that is running fills a deployed database, where what a local one may have must
// not be: a token whose value is written down in the app's code (api.SeedPerson keeps none there).
func SeedIsDeployed(ctx context.Context) bool {
	deployed, _ := ctx.Value(seedDeployed{}).(bool)
	return deployed
}

// seeded runs the seeds, for a request from this machine.
func seeded(w http.ResponseWriter, r *http.Request) {
	// From this machine itself, by its name: the address charter asks at. An address of the network around it is
	// local for a page, and not for this.
	host, _, _ := strings.Cut(r.Host, ":")
	if r.Method != http.MethodPost || host != "localhost" {
		http.Error(w, "a seed runs for charter, on this machine", http.StatusForbidden)
		return
	}
	ctx := r.Context()
	if r.Header.Get(SeedDeployed) != "" {
		ctx = context.WithValue(ctx, seedDeployed{}, true)
	}
	for _, s := range seeds {
		db, err := s.binding.Open()
		if err == nil {
			err = s.fill(ctx, db)
		}
		if err != nil {
			http.Error(w, "the seed failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Write([]byte("seeded"))
}

// Run serves the app as a Worker and never returns.
func (a *App) Run() { Run(a.Handler()) }

type key struct{}

// page is what Head and HTML read from the request's context.
type page struct {
	site *App
	r    *http.Request
	memo *sync.Map
}

// Request is the request being answered, for a component that is no page's own (a header that says who is signed
// in): nil outside one.
func Request(ctx context.Context) *http.Request {
	p, _ := ctx.Value(key{}).(page)
	return p.r
}

// Name is the app's name (App.Name), for the same.
func Name(ctx context.Context) string {
	if p, ok := ctx.Value(key{}).(page); ok && p.site != nil {
		return p.site.Name
	}
	return ""
}

// Fact is one line of the admin screen's overview that is the app's own: what it is called, and its value now. A
// number is shown as the reader's language writes one, a time.Time as a date, anything else as it prints.
type Fact struct {
	Label intl.Msg
	Value func(ctx context.Context) (any, error)
}

// Facts are the app's facts (App.Facts), for the admin screen.
func Facts(ctx context.Context) []Fact {
	if p, ok := ctx.Value(key{}).(page); ok && p.site != nil {
		return p.site.Facts
	}
	return nil
}

// AuthOf is the app's Auth, for the same: nil for an app without one.
func AuthOf(ctx context.Context) Auth {
	if p, ok := ctx.Value(key{}).(page); ok && p.site != nil {
		return p.site.Auth
	}
	return nil
}

// Memo is where what is worked out once for a request is kept until it is answered (who its caller is): nil
// outside one.
func Memo(ctx context.Context) *sync.Map {
	p, _ := ctx.Value(key{}).(page)
	return p.memo
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
			mux.Handle("/", a.missing(a.API))
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
	// What a page is, is asked of the tree: the route a path matches, as the mux matches it, is a page when its type
	// has a Page component. So a page with a parameter (/notes/{id}) is one too, with nothing listed anywhere.
	routes := byLanguage[languages[0]].(*http.ServeMux)
	matched := func(path string) (indexable, isPage bool) {
		_, pattern := routes.Handler(&http.Request{Method: http.MethodGet, URL: &url.URL{Path: path}, Host: "page"})
		indexable, isPage = a.routes[pattern]
		return indexable, isPage
	}
	isPage := func(path string) bool { _, is := matched(path); return is }
	// A page's path as search engines ask for it has its language and its query: /de/about?x=1.
	indexable := func(pathAndQuery string) bool {
		path, _, _ := strings.Cut(pathAndQuery, "?")
		may, _ := matched(intl.Unprefixed(path))
		return may
	}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux, ok := byLanguage[intl.Current(r.Context())]
		if !ok {
			mux = byLanguage[intl.Default()]
		}
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, page{a, r, &sync.Map{}})))
	})
	a.site.IsPage = isPage
	a.site.IsShared = func(path string) bool {
		_, pattern := routes.Handler(&http.Request{Method: http.MethodGet, URL: &url.URL{Path: path}, Host: "page"})
		return a.shared[pattern]
	}
	h = a.site.Handler(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A path that touches no binding: what gsx dev probes after each rebuild, what a Go runtime answers as it
		// starts warm, what a health check asks.
		if r.URL.Path == "/healthz" {
			w.Write([]byte("ok"))
			return
		}
		// A Worker built with the app's seed (charter run db:seed) runs it when charter asks.
		if r.URL.Path == SeedPath && len(seeds) > 0 {
			seeded(w, r)
			return
		}
		// An invitation's link (charter run people:invite) leads to the sign-in page, which signs its holder up
		// as what it says.
		if r.URL.Path == InvitePath && a.SignIn != nil && a.tree != nil {
			if to, err := a.tree.URLFor(a.SignIn); err == nil {
				http.Redirect(w, r, to+"?invite="+url.QueryEscape(r.URL.Query().Get("code")), http.StatusSeeOther)
				return
			}
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
// shared: the page is the same for every reader, and is marked for Cloudflare to keep when this answer is (shared.go).
func whole(next http.Handler, shared bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := &buffered{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(b, r)
		if shared {
			share(w.Header(), r, b.status)
		}
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
		_, isPage := pn.Components["Page"]
		if collect {
			if path, ok := indexed(pn); ok {
				a.pages = append(a.pages, path)
			}
			if isPage && (pn.Method == "" || pn.Method == http.MethodGet) {
				// The route as the mux knows it, and whether search engines may index what it serves.
				pattern := pn.FullRoute()
				if pn.Method != "" {
					pattern = pn.Method + " " + pattern
				}
				if a.routes == nil {
					a.routes = map[string]bool{}
				}
				n, says := pn.Value.Interface().(interface{ Noindex() bool })
				a.routes[pattern] = !says || !n.Noindex()
				if isShared(pn) {
					if a.shared == nil {
						a.shared = map[string]bool{}
					}
					a.shared[pattern] = true
				}
			}
		}
		// A fragment's address opened as a document goes to the page it is part of (guard.go).
		next = guard(next, pn)
		// What the route needs of its caller, said on its type or on a group above it, is checked before it runs.
		next = a.allowed(next, pn)
		// A page goes out whole; an action or a stream writes as it goes.
		if isPage {
			return whole(next, isShared(pn))
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
		structpages.WithErrorHandler(a.failed), structpages.WithMiddlewares(each),
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

// Body is <body>'s attributes (<body class="…" { Body()... }>): a link to another page of the app, and a form, shows
// the next page with no page load. The document stays (its scripts, its stylesheet, its theme) and htmx puts the
// next page's body in place of this one's (boost). In place of, not morphed into: a page reached by a link is as
// that page loaded would be. Morphing keeps the elements both pages have, and with them what a reader did to
// them: a menu that was open stayed open over the next page, where nothing had opened it and nothing placed it.
// The page's title comes with it. With htmx's preload extension (web/main.js) the request starts when the link is
// pressed, before the click is done.
//
// It is how the swap of a boosted page is said, and of nothing else: an element with its own hx-get keeps its own
// swap.
//
// What must be a page load says so: a page whose type has FullLoad() bool (the sign-in screens, whose scripts run
// as a document loads), and a link with hx-boost="false" (the language list: <html lang> changes).
func Body() gsx.Attrs {
	return gsx.Attrs{{Key: "hx-boost:inherited", Value: "swap:innerHTML transition:true"}}
}

// head is what Head needs of the request.
type head struct {
	Name, Icon, Home, Lang, Page, Entry string
	Assets                              vite.Bundle
	// Dev is whether Vite's dev server serves the site's files (charter dev): the stylesheet then comes by script.
	Dev bool
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
		h.Dev = v.Dev()
	}
	h.Page = intl.Unprefixed(p.r.URL.Path)
	h.Lang = intl.Current(ctx).ID()
	if h.Page == "/" {
		h.Home = Origin(p.r) + p.r.URL.Path
	}
	return h
}

// local says the request was made to this machine or its own network (wrangler dev), where there is no https to
// move to: by the machine's name, a name no one outside can have (.internal, which a container calls its host by;
// .localhost), or an address of a private network (a phone on the same Wi-Fi).
func local(r *http.Request) bool {
	host := r.URL.Hostname()
	if host == "" {
		host, _, _ = strings.Cut(r.Host, ":")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip.IsLoopback() || ip.IsPrivate()
}
