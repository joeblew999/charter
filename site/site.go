// Package site wires a server-rendered site together: the browser's files Vite builds, its languages and how it
// answers search engines, in one handler and one <head>.
//
//	//go:embed dist/.vite/manifest.json
//	var dist embed.FS
//
//	func main() {
//		s := &site.Site{Name: "My App", Dist: dist, Pages: []string{"/", "/about"}}
//		transport.Run(s.Handler(mux))
//	}
//
// and in the layout:
//
//	<html { site.HTML(ctx)... }>
//		<head>
//			<site.Head title={title} description={description}/>
//		</head>
//
// What that gives:
//
//   - Vite (github.com/gsxhq/vite): with charter dev the page loads web/main.js from Vite's dev server and reloads on
//     a save; built, it loads the hashed files of dist/, which Workers Static Assets serve (wrangler.jsonc's assets).
//   - Languages (site/intl), when the app has them (charter.toml's [intl]): a URL per language (/about, /de/about),
//     <html lang dir>, the canonical URL and every language's alternate in <head>.
//   - Search engines (site/seo): /robots.txt, /sitemap.xml listing Pages in every language, /llms.txt, noindex on
//     whatever is not one of Pages and on everything in a preview, and one address when the Worker has a domain.
//
// The Worker's variables it reads (wrangler.jsonc's vars): APP_DOMAIN, the canonical host, which only production has;
// APP_PREVIEW, set to anything where the site must not be indexed (staging, previews).
package site

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/gsxhq/gsx"
	"github.com/gsxhq/vite"

	"github.com/joeblew999/charter/cf/bindings"
	"github.com/joeblew999/charter/cf/transport"
	"github.com/joeblew999/charter/site/gsxintl"
	"github.com/joeblew999/charter/site/intl"
	"github.com/joeblew999/charter/site/seo"
	"github.com/joeblew999/charter/site/web"
)

// Site is a site: what it is called, the pages people read, and Vite's manifest.
type Site struct {
	// Name is the site's name, which search results show beside its pages.
	Name string
	// Dist has Vite's manifest, dist/.vite/manifest.json (//go:embed dist/.vite/manifest.json). nil: read from disk,
	// for a test or go run . in the app's folder.
	Dist fs.FS
	// Pages are the pages people read and search engines may index, each a path without a language ("/", "/about").
	Pages []string
	// IsPage says whether a path, without a language, is such a page. nil: one of Pages.
	IsPage func(path string) bool
	// Entry is the Vite entry every page loads. Empty: web/main.js.
	Entry string
	// Icon is the site's icon. Empty: /favicon.svg (public/favicon.svg).
	Icon string
	// LLMs writes /llms.txt for the site's origin (seo.LLMsTxt writes the layout). nil: none.
	LLMs func(origin string) string

	once sync.Once
	vite *vite.Vite
	err  error
}

type key struct{}

// page is what Head and HTML read from the request's context.
type page struct {
	site *Site
	r    *http.Request
}

// Handler is next as the site: its assets, its languages and its answers to search engines around it.
func (s *Site) Handler(next http.Handler) http.Handler {
	isPage := s.IsPage
	if isPage == nil {
		isPage = func(path string) bool { return slices.Contains(s.Pages, path) }
	}
	// A page's path as search engines ask for it has its language and its query: /de/about?x=1.
	indexable := func(pathAndQuery string) bool {
		path, _, _ := strings.Cut(pathAndQuery, "?")
		return isPage(unprefixed(path))
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vite's files and public/ are Static Assets on Workers, answered before the Worker runs; off Workers they
		// come from dist on disk.
		if strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == s.icon() {
			bindings.StaticAssets("dist").ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, page{s, r})))
	})
	var h http.Handler = inner
	if len(intl.Shipped()) > 0 {
		h = intl.Site{
			IsPage:             isPage,
			ConnectionTimeZone: func(r *http.Request) string { return transport.CFOf(r).Timezone },
			Origin:             web.Origin,
		}.Handler(h)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, err := s.assets()
		if err != nil {
			http.Error(w, "site: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Read per request: on Workers a variable is the request's environment's.
		seo.Site{
			Host:    bindings.Var("APP_DOMAIN"),
			Sitemap: s.sitemap,
			LLMs:    s.LLMs,
			IsPage:  indexable,
			Preview: bindings.Var("APP_PREVIEW") != "",
			Origin:  web.Origin,
		}.Handler(v.Middleware(h)).ServeHTTP(w, r)
	})
}

// assets is what the site needs before its first request, made once: off Workers the languages' Intl, and Vite: in dev (VITE_DEV_URL, which gsx dev sets) the dev server's URLs, else the manifest's.
func (s *Site) assets() (*vite.Vite, error) {
	s.once.Do(func() {
		dist := s.Dist
		if dist == nil {
			dist = os.DirFS(".")
		}
		if len(intl.Shipped()) > 0 {
			if s.err = startIntl(); s.err != nil {
				return
			}
		}
		s.vite, s.err = vite.New(vite.Config{DevURL: os.Getenv("VITE_DEV_URL"), DevBase: "/__vite/", Dist: dist, DistDir: "dist", StaticURL: "/"})
	})
	return s.vite, s.err
}

func (s *Site) sitemap(origin string) string {
	if len(intl.Shipped()) > 0 {
		return intl.Sitemap(origin, s.Pages)
	}
	return seo.Sitemap(origin, s.Pages)
}

func (s *Site) icon() string {
	if s.Icon == "" {
		return "/favicon.svg"
	}
	return s.Icon
}

func (s *Site) entry() string {
	if s.Entry == "" {
		return "web/main.js"
	}
	return s.Entry
}

// unprefixed is path without its language, for a site with languages.
func unprefixed(path string) string {
	if len(intl.Shipped()) == 0 {
		return path
	}
	return intl.Unprefixed(path)
}

// HTML is <html>'s attributes: the page's language and direction (<html { site.HTML(ctx)... }>).
func HTML(ctx context.Context) gsx.Attrs {
	if len(intl.Shipped()) == 0 {
		return gsx.Attrs{{Key: "lang", Value: "en"}}
	}
	return gsxintl.HTML(ctx)
}

// head is what Head needs of the request.
type head struct {
	Name, Icon, Home, Lang, Page, Entry string
	Languages                           bool
	Assets                              vite.Bundle
}

func headOf(ctx context.Context, entry string) head {
	p, _ := ctx.Value(key{}).(page)
	if p.site == nil {
		return head{Lang: "en"}
	}
	h := head{Name: p.site.Name, Icon: p.site.icon(), Lang: "en", Languages: len(intl.Shipped()) > 0, Entry: entry}
	if h.Entry == "" {
		h.Entry = p.site.entry()
	}
	if v := vite.FromContext(ctx); v != nil {
		h.Assets = v.Entry(h.Entry)
	}
	h.Page = unprefixed(p.r.URL.Path)
	if h.Languages {
		h.Lang = intl.Current(ctx).ID()
	}
	if h.Page == "/" {
		h.Home = web.Origin(p.r) + p.r.URL.Path
	}
	return h
}
