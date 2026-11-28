package app

import (
	"net/http"
	"strings"

	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/cloud"
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
//   - the request brought no cookie and no Authorization: a reader who has chosen a style or a language, or
//     signed in, is answered by the Worker;
//   - the answer gives no cookie.
//
// Cloudflare keeps a copy for each value of the request headers an answer names (Vary), compared as written, so a
// kept page names only the four it is drawn from: Cookie and Authorization (the readers with neither, which is
// most, share the one copy; one with either finds none and reaches the Worker), HX-Target (htmx asking for a part of the page
// must not be given the page) and X-Forwarded-Proto (a request over http is sent to https, not given the page:
// the scheme is not part of what Cloudflare keeps a copy under). Not the browser's languages, not how it was asked for: a page load, a link followed
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

// anyone is whether a request brings nothing of a reader: no cookie and no token. One that does is answered by
// the Worker, for them.
func anyone(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.Header.Get("Cookie") == "" && r.Header.Get("Authorization") == "" && r.Header.Get(cloud.NewestHeader) == ""
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
	h.Set("Vary", "HX-Target, Cookie, Authorization, X-Forwarded-Proto")
}
