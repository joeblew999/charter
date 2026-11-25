// Package seo is how a site answers search engines: Site, the middleware that serves robots.txt, the sitemap and
// llms.txt, marks what isn't a page noindex and gives every page one address; Sitemap and LLMsTxt, what it serves; and
// SnippetWidth, how wide Google shows a title. It imports no gsx and is small enough for a Worker. Beside it:
// seo/audit (the site as Googlebot reads it, for an app's tests), seo/config (an app's seo.toml), gsxseo (the page's
// <head>, for a gsx app) and the mise-seo binary, which a repo gets through mise (docs/seo/README.md, which mise-seo
// init writes).
package seo

import (
	"io"
	"net/http"
	"strings"
)

// Site is how an app answers search engines, as middleware around the rest of it (outside a locale middleware such
// as intl.Site, so it sees the paths crawlers ask for):
//
//   - /robots.txt: one group that lets every crawler read everything, and where the sitemap is;
//   - /sitemap.xml: the app's Sitemap for the request's origin;
//   - /llms.txt: the app's LLMs for the request's origin, the site as agents read it (https://llmstxt.org);
//   - X-Robots-Tag: noindex on every response that isn't a page (IsPage) or one of those files, and on
//     everything under Preview;
//   - one address: a GET of a page or of those files on another host, or over plain http, is redirected (301) to
//     https://Host with the same path and query.
//
// The files are only at the site's root, as crawlers ask for them: /de/robots.txt goes on to the app.
type Site struct {
	// Host is the canonical host ("app.example.com"). Empty: nothing redirects (a preview, local, a test). A
	// loopback host never redirects.
	Host string
	// Sitemap writes the sitemap for origin ("https://app.example.com"), e.g. intl.Sitemap. nil: no /sitemap.xml.
	Sitemap func(origin string) string
	// LLMs writes /llms.txt for origin: what the site is and where to read more, for agents (LLMsTxt writes the
	// llmstxt.org layout). nil: no /llms.txt.
	LLMs func(origin string) string
	// IsPage reports whether a GET or HEAD of path, the URL's path and query as the request has them
	// ("/de/board?topic=games"), is for a page people read and Google may index; everything else gets X-Robots-Tag:
	// noindex. The same shape as intl.Site's IsPage, so one page table answers both. nil: every GET and HEAD is a page.
	IsPage func(path string) bool
	// Preview marks every response noindex: a preview must never compete with production.
	Preview bool
	// ContentSignal is robots.txt's Content-Signal line (Cloudflare's content signals: what crawlers may use the
	// content for), e.g. "search=yes, ai-input=yes, ai-train=yes". Empty: none.
	ContentSignal string
	// Origin is the request's scheme and host ("https://app.example.com"). nil: from the request, and
	// X-Forwarded-Proto behind a proxy.
	Origin func(r *http.Request) string
}

// Handler wraps next.
func (s Site) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read := r.Method == http.MethodGet || r.Method == http.MethodHead
		robots := read && r.URL.Path == "/robots.txt"
		sitemap := read && r.URL.Path == "/sitemap.xml" && s.Sitemap != nil
		llms := read && r.URL.Path == "/llms.txt" && s.LLMs != nil
		file := robots || sitemap || llms
		page := read && !file && (s.IsPage == nil || s.IsPage(r.URL.RequestURI()))
		origin := s.origin(r)
		if (page || file) && s.Host != "" && !loopback(r) && origin != "https://"+s.Host {
			target := "https://" + s.Host + r.URL.EscapedPath()
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		if s.Preview || !page && !file {
			w.Header().Set("X-Robots-Tag", "noindex")
		}
		switch {
		case robots:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			io.WriteString(w, s.robots(origin))
		case sitemap:
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			io.WriteString(w, s.Sitemap(origin))
		case llms:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			io.WriteString(w, s.LLMs(origin))
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// robots is robots.txt: one * group that allows everything (a named group, say Googlebot, would make that crawler
// ignore *), and the sitemap's absolute URL.
func (s Site) robots(origin string) string {
	body := "User-agent: *\n"
	if s.ContentSignal != "" {
		body += "Content-Signal: " + s.ContentSignal + "\n"
	}
	body += "Allow: /\n"
	if s.Sitemap != nil {
		body += "\nSitemap: " + origin + "/sitemap.xml\n"
	}
	return body
}

func (s Site) origin(r *http.Request) string {
	if s.Origin != nil {
		return s.Origin(r)
	}
	return Origin(r)
}

// Origin is the request's scheme and host ("https://app.example.com"), as Site reads it without its own Origin: from
// the request, and X-Forwarded-Proto behind a proxy. A page's canonical URL is its origin and path, as the sitemap
// lists it.
func Origin(r *http.Request) string {
	scheme, host := r.URL.Scheme, r.URL.Host
	if host == "" {
		host = r.Host
	}
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
	}
	return scheme + "://" + host
}

// loopback reports whether the request is for localhost or a loopback address: a local server never redirects. (Plain
// string handling: the net package is thin under TinyGo.)
func loopback(r *http.Request) bool {
	host := r.URL.Host
	if host == "" {
		host = r.Host
	}
	if strings.HasPrefix(host, "[") { // [::1]:8913
		host, _, _ = strings.Cut(host[1:], "]")
	} else if h, _, ok := strings.Cut(host, ":"); ok { // 127.0.0.1:8913
		host = h
	}
	return host == "localhost" || host == "::1" || strings.HasPrefix(host, "127.")
}
