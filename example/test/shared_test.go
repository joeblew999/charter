package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
)

// Only a page that says it is the same for every reader may be kept by Cloudflare's cache: the docs are, a page
// that reads the database is not, and nothing of the API is. Everything says which it is, since the cache keeps an
// answer that says nothing.
func TestOnlyAPageForEveryReaderMayBeKept(t *testing.T) {
	t.Parallel()
	if res, _ := asked(t, http.MethodGet, at(screens.DocsIndex{}), "", "Accept", "text/html"); !strings.HasPrefix(res.Header.Get("Cache-Control"), "public") {
		t.Errorf("the docs: Cache-Control %q", res.Header.Get("Cache-Control"))
	}
	for _, path := range []string{at(pages.Home{}), "/api/notes", "/openapi.json", "/healthz"} {
		res, _ := asked(t, http.MethodGet, path, "", "Accept", "text/html")
		if said := res.Header.Get("Cache-Control"); said == "" || strings.HasPrefix(said, "public") {
			t.Errorf("%s: Cache-Control %q", path, said)
		}
	}
}
