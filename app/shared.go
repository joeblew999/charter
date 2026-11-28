package app

import (
	"net/http"
	"strings"

	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/intl"
)

// A page that is the same for every reader says so, and Cloudflare keeps it: its type has Shared() bool.
//
//	func (About) Shared() bool { return true }
//
// The Worker's own cache (wrangler.jsonc's "cache": Workers Cache) is asked before the Worker runs. A page it has is
// answered from there, from the nearest of Cloudflare's places, with no Worker started and no Go: the first hit of a
// reader, which otherwise waits for both, is as quick as any other. A deploy starts an empty cache, so what is kept
// is never of older code.
//
// What may be kept is decided here, once, as the page goes out, and only all of this makes it so:
//
//   - its type says Shared, and it answered 200 to a GET for a page (a part of a page is never kept);
//   - the request brought nothing of a person: no Authorization, and no cookie but the ones a page is drawn from
//     (the language, the time zone and clock, gsxui's style). A session, a bookmark of the database, any cookie of
//     the app's own: the answer is that reader's;
//   - the answer gives no cookie.
//
// Cloudflare keeps one copy for each set of the request headers the page says it was drawn from (Vary: the cookies,
// the browser's languages, what htmx asked for), so a reader with another style or language gets their own.
//
// Everything else the Worker answers says private (bookmark.go's give, worker/go.mjs): with its cache on, Cloudflare
// keeps an answer that says nothing for two hours.
//
// A shared page shows nothing it read for the request: no row of a database, no time in the reader's zone (which
// may come from where they connect, and that is no header), nothing of who asks.
const sharedFor = "public, max-age=0, s-maxage=86400, stale-while-revalidate=604800"

// isShared is whether a route's type says its page is the same for every reader.
func isShared(pn *structpages.PageNode) bool {
	own, ok := pn.Value.Interface().(interface{ Shared() bool })
	return ok && own.Shared()
}

// drawnFrom are the cookies a page is drawn from, and that say nothing of who reads it.
var drawnFrom = map[string]bool{
	intl.LocaleCookie: true, intl.ChosenLocaleCookie: true, intl.TimeZoneCookie: true, intl.HourCycleCookie: true,
	StyleCookie: true,
}

// anyone is whether a request brings nothing of a person.
func anyone(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.Header.Get("Authorization") != "" || r.Header.Get(cloud.NewestHeader) != "" {
		return false
	}
	for _, c := range r.Cookies() {
		if !drawnFrom[c.Name] {
			return false
		}
	}
	return true
}

// share marks an answer that every reader may be given, when it is one (the rules above). It is called when the
// page is whole and its status known, before its headers go.
func share(h http.Header, r *http.Request, status int) {
	if status != http.StatusOK || !anyone(r) || wanted(r) == partOfAPage || len(h.Values("Set-Cookie")) > 0 {
		return
	}
	if kept := h.Get("Cache-Control"); kept != "" && !strings.EqualFold(kept, "private") {
		return // the page said something else itself (no-store)
	}
	h.Set("Cache-Control", sharedFor)
	// What else the answer was drawn from: that the request brought no token (a cache gives a kept answer to any
	// request whose headers named here are as they were, so one with a token is not given this), and the scheme the
	// page's own links to itself have.
	addVary(h, "Authorization", cloud.NewestHeader, "X-Forwarded-Proto")
}
