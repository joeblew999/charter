package app

import (
	"net/http"
	"slices"
	"strings"

	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/intl"
)

// A fragment is a route with Props and no Page: markup that is swapped into a page (a dialog's form, a row), with an
// address of its own. A browser can open that address (a link in a new tab, a reload, Back) and would get bare
// markup: no layout, no styles. guard is structpages' own answer to that (its route conventions, "Guarding fragment
// URLs"), for every app: such a request is sent to the page the fragment is part of, before the fragment's Props
// runs. Only an htmx request for part of a page gets the fragment.
//
// Where it is sent: the fragment's own Landing(*http.Request) (string, error) when it has one; else the nearest
// page above it in the tree that has no parameters; else the front page.
func guard(next http.Handler, pn *structpages.PageNode) http.Handler {
	_, hasPage := pn.Components["Page"]
	fragment := !hasPage && pn.Props != nil
	if !hasPage && !fragment {
		return next // what a page calls, a stream, a download
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addVary(w.Header(), "HX-Request", "HX-Target", "HX-Request-Type", "HX-Boosted", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Cookie")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		wants := wanted(r)
		// A page that must be a page load (its type has FullLoad): a boosted link to it is sent there as a
		// navigation, which htmx does when told so (HX-Redirect).
		if wants == wholePage && hasPage && fullLoad(pn) {
			w.Header().Set("HX-Redirect", r.URL.RequestURI())
			return
		}
		if wants == partOfAPage || fragment {
			w.Header().Set("Cache-Control", "private, no-store")
		}
		if !fragment || wants == partOfAPage {
			next.ServeHTTP(w, r)
			return
		}
		to := landing(r, pn)
		if wants == wholePage {
			w.Header().Set("HX-Location", to)
			return
		}
		http.Redirect(w, r, to, http.StatusSeeOther)
	})
}

// fullLoad is whether a page says it must be a page load: one whose scripts run as a document loads, or whose
// document is not the app's own.
func fullLoad(pn *structpages.PageNode) bool {
	own, ok := pn.Value.Interface().(interface{ FullLoad() bool })
	return ok && own.FullLoad()
}

// landing is the page a fragment is part of, in the request's language.
func landing(r *http.Request, pn *structpages.PageNode) string {
	if own, ok := pn.Value.Interface().(interface {
		Landing(*http.Request) (string, error)
	}); ok {
		if to, err := own.Landing(r); err == nil {
			return to
		}
	}
	for above := pn.Parent; above != nil; above = above.Parent {
		// The page itself, or the group's own page (its /{$} child).
		for _, node := range append([]*structpages.PageNode{above}, above.Children...) {
			if _, isPage := node.Components["Page"]; !isPage || node == pn {
				continue
			}
			if path := strings.TrimSuffix(node.FullRoute(), "{$}"); !strings.Contains(path, "{") {
				return intl.URL(r.Context(), path)
			}
		}
	}
	return intl.URL(r.Context(), "/")
}

type want int

const (
	document    want = iota // a browser's navigation, curl, a test
	wholePage               // an htmx request that swaps the whole body (a boosted link)
	partOfAPage             // an htmx request for part of a page
)

// wanted reads what a GET is for. Anything that is no htmx request wants a document. htmx 4's HX-Request-Type: full
// is not read as a page load: htmx sends it for any hx-select swap too; the target says what is replaced.
func wanted(r *http.Request) want {
	switch r.Header.Get("Sec-Fetch-Dest") {
	case "document", "iframe", "frame":
		return document
	}
	if r.Header.Get("Sec-Fetch-Mode") == "navigate" || r.Header.Get("HX-Request") != "true" {
		return document
	}
	if tag, _, _ := strings.Cut(r.Header.Get("HX-Target"), "#"); tag == "body" {
		return wholePage
	}
	return partOfAPage
}

// addVary adds tokens to the Vary header once each, keeping what is there.
func addVary(h http.Header, tokens ...string) {
	var all []string
	for _, v := range h.Values("Vary") {
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" {
				all = append(all, t)
			}
		}
	}
	for _, t := range tokens {
		if !slices.ContainsFunc(all, func(s string) bool { return strings.EqualFold(s, t) }) {
			all = append(all, t)
		}
	}
	h.Set("Vary", strings.Join(all, ", "))
}
