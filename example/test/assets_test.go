package test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
)

// What a page links to that Vite built (its stylesheet, its script) has its contents in its name, and is served to
// be kept for good: a browser that has it does not ask again on the next page. Asked about every time, as
// Cloudflare serves a static asset unless told otherwise, the stylesheet's and the font's answers come after the
// page is shown, and the page is seen to change.
func TestWhatViteBuiltIsKeptForGood(t *testing.T) {
	linked := regexp.MustCompile(`(?:href|src)="(/assets/[^"]+)"`).FindAllStringSubmatch(apptest.Get(t, "/"), -1)
	if len(linked) < 2 {
		t.Fatalf("the page links to %d files of Vite's, want its stylesheet and its script", len(linked))
	}
	for _, found := range linked {
		res, err := http.Get(apptest.Base(t) + found[1])
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if cache := res.Header.Get("Cache-Control"); res.StatusCode != http.StatusOK || !strings.Contains(cache, "immutable") || !strings.Contains(cache, "max-age=31536000") {
			t.Errorf("%s: %s, Cache-Control %q, want it kept for a year, immutable", found[1], res.Status, cache)
		}
	}
}
