package test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/site/pages"
	"github.com/joeblew999/charter/site/pages/screens"
)

// A page that is the same for every reader is kept by Cloudflare, which answers with it before the Worker runs; an
// answer that is one reader's is never kept. These ask the Worker with plain HTTP: it is the headers that say.

// answered is one GET of a page, as a browser asks for one.
func answered(t *testing.T, path string, headers ...string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, apptest.Base(t)+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "en")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	return res
}

func kept(res *http.Response) bool {
	return strings.HasPrefix(res.Header.Get("Cache-Control"), "public")
}

func TestAPageForEveryReaderMayBeKeptAndAReadersOwnMayNot(t *testing.T) {
	t.Parallel()
	for _, path := range []string{at(pages.Home{}), at(screens.DocsIndex{}), at(screens.DocsPage{}, "pages"), at(screens.Theme{})} {
		res := answered(t, path)
		if res.StatusCode != http.StatusOK || !kept(res) {
			t.Errorf("%s, asked by no one in particular: %d, Cache-Control %q", path, res.StatusCode, res.Header.Get("Cache-Control"))
		}
		if cookies := res.Header.Values("Set-Cookie"); len(cookies) > 0 {
			t.Errorf("%s gives a cookie, and an answer with one is kept by no cache: %q", path, cookies)
		}
		if vary := res.Header.Get("Vary"); !strings.Contains(vary, "Cookie") {
			t.Errorf("%s does not say it was drawn from the request's cookies: Vary %q", path, vary)
		}
		// A reader's style and language are cookies a page is drawn from: theirs is kept too, apart.
		if res := answered(t, path, "Cookie", "locale=en; gsxui-style=maia"); !kept(res) {
			t.Errorf("%s in a reader's style: Cache-Control %q", path, res.Header.Get("Cache-Control"))
		}
		// Whoever brings something of a person gets an answer of their own.
		for _, theirs := range [][]string{{"Cookie", "session=someone"}, {"Cookie", "d1=DB=1"}, {"Cookie", "anything=else"}, {"Authorization", "Bearer someone"}} {
			if res := answered(t, path, theirs...); kept(res) {
				t.Errorf("%s asked with %s: Cache-Control %q", path, theirs[0], res.Header.Get("Cache-Control"))
			}
		}
		// A part of a page is not a page.
		if res := answered(t, path, "HX-Request", "true", "HX-Target", "div#doc"); kept(res) {
			t.Errorf("%s asked for as a part of a page: Cache-Control %q", path, res.Header.Get("Cache-Control"))
		}
	}
	// Nothing else says it may be kept, and everything says something: Cloudflare keeps an answer that says nothing.
	for _, path := range []string{"/docs/no-such-page", "/no-such-page", "/sitemap.xml", "/healthz"} {
		if res := answered(t, path); kept(res) || res.Header.Get("Cache-Control") == "" {
			t.Errorf("%s: Cache-Control %q", path, res.Header.Get("Cache-Control"))
		}
	}
}

// On Cloudflare the page is then answered from the cache (Cf-Cache-Status), and a reader with a session is not.
func TestCloudflareAnswersAKeptPageItself(t *testing.T) {
	if apptest.Local() {
		t.Skip("wrangler dev has no cache before the Worker")
	}
	path := at(screens.DocsPage{}, "pages")
	var said string
	for range 10 {
		res := answered(t, path)
		if said = res.Header.Get("Cf-Cache-Status"); said == "HIT" {
			break
		}
		time.Sleep(time.Second)
	}
	if said != "HIT" {
		t.Fatalf("%s was not answered from the cache in 10 tries: Cf-Cache-Status %q", path, said)
	}
	if res := answered(t, path, "Cookie", "session=someone"); res.Header.Get("Cf-Cache-Status") == "HIT" || kept(res) {
		t.Errorf("a reader with a session was given the kept page: Cf-Cache-Status %q, Cache-Control %q", res.Header.Get("Cf-Cache-Status"), res.Header.Get("Cache-Control"))
	}
}
