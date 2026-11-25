package intl

import (
	"cmp"
	"net/http"
	"net/url"
	"strings"
)

// Site puts each request's language in its context (Handler), by the URL rules a site with languages needs:
//
//	/about        the source locale (intl.toml's first): unprefixed, and hreflang x-default
//	/de/about     every other shipped locale, under its lowercase id ("/pt-br/", "/zh-hant/")
//	/en/about     301 → /about;  /DE/about, /pt-BR/ → 301 to the lowercase prefix;  /de → 301 /de/
//
// There are no redirects from Accept-Language (Google: separate URLs per language and no automatic redirects;
// Googlebot sends none). A page view remembers its locale in a cookie, and only the bare "/" follows it, so a
// returning reader lands on their language's home page. Until the reader chooses a language, every page offers the
// one their browser asks for (Suggested); following a link from a page of the site in one language to one in another
// is choosing it, and so is "?locale=<id>" on any page (Decline, the language list's link to the source locale's
// home page), which redirects to the clean URL. A locale whose catalog is incomplete is served noindex. The viewer's
// time zone and clock come from the cookies Preferences sets, else the connection's zone. Every response is
// Cache-Control: private (unless the app's handler says otherwise): it is this reader's.
type Site struct {
	// IsPage reports whether path (without its locale prefix) is a page people read; only a page view remembers
	// its locale. nil: every path.
	IsPage func(path string) bool
	// ConnectionTimeZone is the server's guess of the viewer's IANA time zone (Cloudflare's request.cf.timezone).
	// nil: none, "UTC".
	ConnectionTimeZone func(*http.Request) string
	// Origin is the request's scheme and host, for absolute URLs. nil: from the request (and X-Forwarded-Proto).
	Origin func(*http.Request) string
}

// The cookies Site reads and Preferences sets.
const (
	LocaleCookie          = "locale"        // the locale of the last page viewed
	ChosenLocaleCookie    = "chosen_locale" // the locale the reader chose: from the language list, the offer, Decline
	TimeZoneCookie        = "tz"            // the viewer's chosen IANA time zone
	HourCycleCookie       = "hc"            // the viewer's chosen clock: "h12" or "h23"
	BrowserTimeZoneCookie = "browser_tz"    // the browser's own zone, which intl-browser.mjs reports from the first page
)

// Handler resolves the request's locale from its URL prefix, strips the prefix, and puts Request in the context.
func (s Site) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		def := Default()
		path := r.URL.Path
		seg, rest := path, ""
		if len(path) > 1 {
			if i := strings.IndexByte(path[1:], '/'); i >= 0 {
				seg, rest = path[1:i+1], path[i+1:]
			} else {
				seg = path[1:]
			}
		}
		loc := def
		if len(path) > 1 {
			if found := byPrefix(seg); found != nil {
				rest = sameSite(rest) // "/en//evil.example" must not redirect to //evil.example
				switch {
				case found == def:
					redirect(w, r, cmp.Or(rest, "/"), http.StatusMovedPermanently)
					return
				case "/"+seg != Prefix(found) || rest == "":
					redirect(w, r, Prefix(found)+cmp.Or(rest, "/"), http.StatusMovedPermanently)
					return
				}
				loc, path = found, rest
			}
		}
		// A page view: a GET of a page, loaded by the browser or by an htmx-boosted link (which swaps in the whole
		// page, the offer included); not htmx's request for a part of one.
		htmx := r.Header.Get("HX-Request") != "" && r.Header.Get("HX-Boosted") == ""
		page := r.Method == http.MethodGet && !htmx && (s.IsPage == nil || s.IsPage(path))
		chosen := cookieLocale(r, ChosenLocaleCookie)
		if page {
			// An explicit choice, "?locale=de" on any page (the language list's link to the source locale's home
			// page, "/?locale=en", or Decline): kept, and a redirect to the page in it at its clean URL, so the
			// parameter URL is never indexed as a duplicate.
			if picked := byPrefix(r.URL.Query().Get(LocaleCookie)); picked != nil {
				keep(w, r, ChosenLocaleCookie, picked)
				keep(w, r, LocaleCookie, picked)
				q := r.URL.Query()
				q.Del(LocaleCookie)
				http.Redirect(w, r, withQuery(Prefix(picked)+path, q.Encode()), http.StatusFound)
				return
			}
			// Following a link from a page of this site in another language (the language list, the offer) is
			// choosing this one.
			if from := s.referrer(r); from != nil && from != loc && chosen != loc {
				keep(w, r, ChosenLocaleCookie, loc)
				chosen = loc
			}
		}
		if path == "/" && loc == def && page {
			if c, err := r.Cookie(LocaleCookie); err == nil {
				if remembered := byPrefix(c.Value); remembered != nil && remembered != def {
					w.Header().Add("Vary", "Cookie")
					redirect(w, r, Prefix(remembered)+"/", http.StatusFound)
					return
				}
			}
		}
		d := Decision{LanguageFrom: "default", AcceptLanguage: r.Header.Get("Accept-Language")}
		if loc != def || path != r.URL.Path {
			d.LanguageFrom = "url"
		}
		if c, err := r.Cookie(LocaleCookie); err == nil {
			d.LastViewed = c.Value
		}
		if chosen != nil {
			d.ChosenLanguage = chosen.ID()
		}
		// The language the reader wants, which Suggested offers on a page in another: the one they chose, else the
		// one their browser asks for first.
		var wanted *Locale
		if page {
			wanted = chosen
			if wanted == nil {
				w.Header().Add("Vary", "Accept-Language")
				wanted = preferred(r.Header.Get("Accept-Language"))
			}
			keep(w, r, LocaleCookie, loc)
		}
		w.Header().Set("Content-Language", loc.ID())
		if !Complete(loc.ID()) {
			// Part of the page would be in the source language: not indexed as this language, nor as a duplicate.
			w.Header().Set("X-Robots-Tag", "noindex")
		}
		req := Request{Locale: loc, Path: withQuery(path, r.URL.RawQuery), Origin: s.origin(r), TimeZone: "UTC", Preferred: wanted}
		if c, err := r.Cookie(HourCycleCookie); err == nil && (c.Value == "h12" || c.Value == "h23") {
			req.HourCycle = c.Value
		}
		// The zone: the one the reader chose; else the one their browser reported (from their second page view on);
		// else Cloudflare's guess from their IP (right unless a VPN, a proxy or travel misleads it); else UTC.
		if c, err := r.Cookie(TimeZoneCookie); err == nil && ValidTimeZone(c.Value) {
			d.ChosenZone = c.Value
		}
		if c, err := r.Cookie(BrowserTimeZoneCookie); err == nil && ValidTimeZone(c.Value) {
			d.BrowserZone = c.Value
		}
		if s.ConnectionTimeZone != nil && ValidTimeZone(s.ConnectionTimeZone(r)) {
			d.ConnectionZone = s.ConnectionTimeZone(r)
		}
		switch {
		case d.ChosenZone != "":
			req.TimeZone, req.TimeZoneChosen, d.ZoneFrom = d.ChosenZone, true, "chosen"
		case d.BrowserZone != "":
			req.TimeZone, d.ZoneFrom = d.BrowserZone, "browser"
		case d.ConnectionZone != "":
			req.TimeZone, d.ZoneFrom = d.ConnectionZone, "connection"
		default:
			req.TimeZone, d.ZoneFrom = "UTC", "default"
		}
		req.Decision = d
		// What follows is written for this reader (their cookies, their browser's languages, where they connect
		// from): no shared cache may give it to another.
		if w.Header().Get("Cache-Control") == "" {
			w.Header().Set("Cache-Control", "private")
		}
		r2 := r.Clone(WithRequest(r.Context(), req))
		r2.URL.Path, r2.URL.RawPath = path, ""
		next.ServeHTTP(w, r2)
	})
}

// Preferences answers the preferences form (POST tz, hc, return): it stores the time zone and clock cookies (an
// empty value clears one, back to automatic) and redirects to the page the form was on, which renders with them.
func (s Site) Preferences(w http.ResponseWriter, r *http.Request) {
	tz, hc := r.FormValue("tz"), r.FormValue("hc")
	if tz != "" && !ValidTimeZone(tz) {
		http.Error(w, "unknown time zone", http.StatusBadRequest)
		return
	}
	if hc != "" && hc != "h12" && hc != "h23" {
		http.Error(w, "unknown hour cycle", http.StatusBadRequest)
		return
	}
	for name, value := range map[string]string{TimeZoneCookie: tz, HourCycleCookie: hc} {
		c := &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode, Secure: secure(r)}
		if value == "" {
			c.MaxAge = -1
		}
		http.SetCookie(w, c)
	}
	http.Redirect(w, r, SafeReturn(r.FormValue("return")), http.StatusSeeOther)
}

// ConnectionZone is the server's guess of the viewer's time zone, "UTC" without one: "Automatic (…)" in a
// preferences form.
func (s Site) ConnectionZone(r *http.Request) string {
	if s.ConnectionTimeZone != nil {
		if id := s.ConnectionTimeZone(r); ValidTimeZone(id) {
			return id
		}
	}
	return "UTC"
}

func (s Site) origin(r *http.Request) string {
	if s.Origin != nil {
		return s.Origin(r)
	}
	scheme, host := r.URL.Scheme, r.URL.Host
	if host == "" {
		host = r.Host
	}
	if scheme == "" {
		scheme = "http"
		if secure(r) {
			scheme = "https"
		}
	}
	return scheme + "://" + host
}

// SafeReturn keeps a same-site redirect target: the path and query of a root-relative URL, or of an http(s) URL
// (never its host), else "/". Protocol-relative URLs, other schemes and control characters (which browsers strip,
// turning "/\t/evil.example" into //evil.example) give "/".
func SafeReturn(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "" && p.Scheme != "http" && p.Scheme != "https" || p.Scheme == "" && p.Host != "" || p.Opaque != "" {
		return "/"
	}
	out := p.EscapedPath()
	if p.RawQuery != "" {
		out += "?" + p.RawQuery
	}
	if out == "" || out[0] != '/' || sameSite(out) != out {
		return "/"
	}
	return out
}

// ReturnTo is the page an htmx request was made from (htmx's HX-Current-URL, else the Referer), kept by SafeReturn:
// where a form loaded into that page sends the reader back to (the preferences form's return).
func ReturnTo(r *http.Request) string {
	return SafeReturn(cmp.Or(r.Header.Get("HX-Current-URL"), r.Referer()))
}

// sameSite is path with one leading slash: "//evil.example" or "/\evil.example" (which browsers read as another
// host) become "/evil.example", a path of this site.
func sameSite(path string) string {
	if path == "" {
		return path
	}
	return "/" + strings.TrimLeft(path, "/\\")
}

// byPrefix is the shipped locale a URL segment or cookie names, case-insensitively, or nil.
func byPrefix(seg string) *Locale {
	if seg == "" || len(seg) > 16 {
		return nil
	}
	return shipped.Find(seg)
}

// cookieLocale is the shipped locale the cookie name names, or nil.
func cookieLocale(r *http.Request, name string) *Locale {
	if c, err := r.Cookie(name); err == nil {
		return byPrefix(c.Value)
	}
	return nil
}

// keep stores l in the cookie name (LocaleCookie or ChosenLocaleCookie), unless it holds l already.
func keep(w http.ResponseWriter, r *http.Request, name string, l *Locale) {
	if c, err := r.Cookie(name); err == nil && c.Value == l.ID() {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: l.ID(), Path: "/", MaxAge: 365 * 24 * 3600,
		SameSite: http.SameSiteLaxMode, Secure: secure(r)})
}

// referrer is the locale of the page of this site whose link r followed (its Referer), nil when it came from
// elsewhere or the browser didn't say.
func (s Site) referrer(r *http.Request) *Locale {
	u, err := url.Parse(r.Referer())
	if err != nil || u.Host == "" || u.Scheme+"://"+u.Host != s.origin(r) {
		return nil
	}
	seg, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	if l := byPrefix(seg); l != nil {
		return l
	}
	return Default()
}

func secure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func redirect(w http.ResponseWriter, r *http.Request, to string, code int) {
	http.Redirect(w, r, withQuery(to, r.URL.RawQuery), code)
}

func withQuery(path, query string) string {
	if query == "" {
		return path
	}
	return path + "?" + query
}
