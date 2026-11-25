package intl

import (
	"context"
	"strings"
)

// Prefix is a locale's URL prefix (Site): "" for the source locale, "/de", "/pt-br", "/zh-hant" otherwise.
func Prefix(l *Locale) string {
	if l == Default() {
		return ""
	}
	return "/" + Key(l)
}

// Key is a locale's id in lower case, as URL prefixes use it: "de", "pt-br".
func Key(l *Locale) string { return strings.ToLower(l.ID()) }

// Unprefixed is path without its locale prefix: "/de/about" is "/about", "/de" is "/", "/about" stays. For what runs
// before Site and asks which page a request is for (seo.Site's IsPage, say).
func Unprefixed(path string) string {
	seg, rest := path, ""
	if len(path) > 1 {
		if i := strings.IndexByte(path[1:], '/'); i >= 0 {
			seg, rest = path[1:i+1], path[i+1:]
		} else {
			seg = path[1:]
		}
	}
	if len(path) > 1 && byPrefix(seg) != nil {
		if rest == "" {
			return "/"
		}
		return rest
	}
	return path
}

// URL is an in-app path in the request's locale: "/about" in the source locale, "/de/about" in German. In markup,
// the localize filter: href={ "/about" |> localize }. Paths that aren't per language (assets) don't go through it.
func URL(ctx context.Context, path string) string { return Prefix(Current(ctx)) + path }

// Link is the request's page in one shipped locale, for a language list.
type Link struct {
	Lang    string // the locale id, for hreflang and lang
	Name    string // the locale's name in its own language
	Href    string
	Current bool // the request's own locale
}

// Links is the request's page in every shipped locale, in intl.toml's order. The source locale's home links to
// "/?locale=<id>": the bare "/" follows a remembered locale, and choosing the source locale must override it.
func Links(ctx context.Context) []Link {
	path := "/"
	if r, ok := FromContext(ctx); ok && r.Path != "" {
		path = r.Path
	}
	cur := Current(ctx)
	out := make([]Link, 0, len(shipped))
	for _, l := range shipped {
		href := Prefix(l) + path
		if href == "/" {
			href = "/?" + LocaleCookie + "=" + l.ID()
		}
		out = append(out, Link{Lang: l.ID(), Name: l.Name(), Href: href, Current: l == cur})
	}
	return out
}

// Alternate is one hreflang link: a locale id, or "x-default", and the page's absolute URL in it.
type Alternate struct {
	Lang, Href string
}

// Canonical is page (a path and the query parameters that make it a different page, without a locale prefix) in the
// request's locale, absolute.
func Canonical(ctx context.Context, page string) string {
	r, _ := FromContext(ctx)
	return r.Origin + Prefix(Current(ctx)) + page
}

// Alternates is page in every shipped locale and x-default (the source locale's URL): the same list on every
// locale's version, so the links are reciprocal.
func Alternates(ctx context.Context, page string) []Alternate {
	r, _ := FromContext(ctx)
	return alternates(r.Origin, page)
}

func alternates(origin, page string) []Alternate {
	out := make([]Alternate, 0, len(shipped)+1)
	for _, l := range shipped {
		out = append(out, Alternate{Lang: l.ID(), Href: origin + Prefix(l) + page})
	}
	return append(out, Alternate{Lang: "x-default", Href: origin + page})
}

// Sitemap is a sitemap.xml of pages in every shipped locale, each URL with all its alternates (Google: Localized
// versions of your page, sitemap method).
func Sitemap(origin string, pages []string) string {
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, page := range pages {
		alts := alternates(origin, page)
		for _, self := range alts[:len(alts)-1] {
			b.WriteString("  <url>\n    <loc>" + esc.Replace(self.Href) + "</loc>\n")
			for _, a := range alts {
				b.WriteString(`    <xhtml:link rel="alternate" hreflang="` + a.Lang + `" href="` + esc.Replace(a.Href) + `"/>` + "\n")
			}
			b.WriteString("  </url>\n")
		}
	}
	b.WriteString("</urlset>\n")
	return b.String()
}

// OGLocale is a locale in Open Graph's language_TERRITORY form, from its likely subtags (Intl.Locale maximize):
// "en" (en-Latn-US) → "en_US", "pt-BR" → "pt_BR", "zh-Hant" (zh-Hant-TW) → "zh_TW".
func OGLocale(l *Locale) string {
	tags := strings.Split(l.Maximal(), "-")
	for _, t := range tags[1:] {
		if len(t) == 2 || len(t) == 3 && t[0] >= '0' && t[0] <= '9' {
			return tags[0] + "_" + t
		}
	}
	return tags[0]
}

// TimeZone is the viewer's time zone as the server knows it: their choice, else the connection's (Site), else
// "UTC". The browser formats in its own zone unless the viewer chose one.
func TimeZone(ctx context.Context) string {
	if r, ok := FromContext(ctx); ok && r.TimeZone != "" {
		return r.TimeZone
	}
	return "UTC"
}

// ChosenTimeZone is the zone the viewer chose, "" when they didn't (the browser's own then): <html data-tz>.
func ChosenTimeZone(ctx context.Context) string {
	if r, ok := FromContext(ctx); ok && r.TimeZoneChosen {
		return r.TimeZone
	}
	return ""
}

// HourCycle is the viewer's chosen clock, "h12" or "h23", "" for the language's own: <html data-hc>.
func HourCycle(ctx context.Context) string {
	r, _ := FromContext(ctx)
	return r.HourCycle
}

// ValidTimeZone reports whether id looks like an IANA time zone id ("Europe/Berlin", "America/Argentina/Buenos_Aires",
// "UTC", "Etc/GMT+5"): letters first, then letters, digits, _ + -, at most three /-separated parts. Whether it exists
// is Intl's to say: the browser ignores a zone its Intl rejects.
func ValidTimeZone(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	parts := strings.Split(id, "/")
	if len(parts) > 3 || !letter(id[0]) {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for i := 0; i < len(p); i++ {
			c := p[i]
			if !letter(c) && !(c >= '0' && c <= '9') && c != '_' && c != '+' && c != '-' {
				return false
			}
		}
	}
	return true
}

func letter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
