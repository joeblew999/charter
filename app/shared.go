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
// Cloudflare keeps a copy for each value of the request headers an answer names (Vary), compared as written, so a
// kept page names only the two it is drawn from: Cookie (a reader with none, which is most, shares one copy; a
// reader with a style or a language of their own gets theirs) and HX-Target (htmx asking for a part of the page
// must not be given the page). Not the browser's languages, not how it was asked for: a page load, a link followed
// in the app, curl and a crawler get the same copy.
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
	// A target that is not the whole page, from something that is not htmx: kept, it would be what htmx is given
	// when it asks for that part.
	if target, _, _ := strings.Cut(r.Header.Get("HX-Target"), "#"); target != "" && target != "body" {
		return
	}
	if kept := h.Get("Cache-Control"); kept != "" && !strings.EqualFold(kept, "private") {
		return // the page said something else itself (no-store)
	}
	h.Set("Cache-Control", sharedFor)
	h.Set("Vary", "HX-Target, Cookie")
}
