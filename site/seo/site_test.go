package seo_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joeblew999/charter/site/seo"
)

func site() http.Handler {
	s := seo.Site{
		Host:          "app.example.com",
		Sitemap:       func(origin string) string { return "<urlset>" + origin + "</urlset>" },
		IsPage:        func(path string) bool { return !strings.HasPrefix(path, "/fragments/") },
		ContentSignal: "search=yes",
		LLMs: func(origin string) string {
			return seo.LLMsTxt("App", "What it is.", "", seo.LLMsSection{Name: "Pages", Links: []seo.LLMsLink{{Title: "Home", URL: origin + "/"}}})
		},
	}
	return s.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("app " + r.URL.Path)) }))
}

func do(h http.Handler, method, url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, url, nil))
	return w
}

// TestRobotsAndSitemap: both at the site's root only, with absolute URLs from the request's origin.
func TestRobotsAndSitemap(t *testing.T) {
	h := site()
	if b := do(h, "GET", "https://app.example.com/robots.txt").Body.String(); b != "User-agent: *\nContent-Signal: search=yes\nAllow: /\n\nSitemap: https://app.example.com/sitemap.xml\n" {
		t.Errorf("robots.txt:\n%s", b)
	}
	if b := do(h, "GET", "https://app.example.com/sitemap.xml").Body.String(); b != "<urlset>https://app.example.com</urlset>" {
		t.Errorf("sitemap.xml: %s", b)
	}
	for _, p := range []string{"/de/robots.txt", "/de/sitemap.xml"} {
		if b := do(h, "GET", "https://app.example.com"+p).Body.String(); b != "app "+p {
			t.Errorf("%s: answered %q, want the app's", p, b)
		}
	}
}

// TestLLMs: /llms.txt at the site's root, from the request's origin, as llmstxt.org lays it out; not marked noindex,
// and on one address like the other files.
func TestLLMs(t *testing.T) {
	h := site()
	w := do(h, "GET", "https://app.example.com/llms.txt")
	if b := w.Body.String(); b != "# App\n\n> What it is.\n\n## Pages\n\n- [Home](https://app.example.com/)\n" {
		t.Errorf("llms.txt:\n%s", b)
	}
	if ct, robots := w.Header().Get("Content-Type"), w.Header().Get("X-Robots-Tag"); ct != "text/plain; charset=utf-8" || robots != "" {
		t.Errorf("llms.txt: Content-Type %q, X-Robots-Tag %q", ct, robots)
	}
	if w := do(h, "GET", "http://app.example.com/llms.txt"); w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "https://app.example.com/llms.txt" {
		t.Errorf("http llms.txt: %d %q", w.Code, w.Header().Get("Location"))
	}
	if b := do(h, "GET", "https://app.example.com/de/llms.txt").Body.String(); b != "app /de/llms.txt" {
		t.Errorf("/de/llms.txt answered %q, want the app's", b)
	}
	none := seo.Site{}.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("app")) }))
	if b := do(none, "GET", "https://app.example.com/llms.txt").Body.String(); b != "app" {
		t.Errorf("no LLMs: /llms.txt answered %q, want the app's", b)
	}
}

// TestLLMsTxt: the details and every link's note when there are some.
func TestLLMsTxt(t *testing.T) {
	got := seo.LLMsTxt("App", "Sum.", "More here.\n", seo.LLMsSection{Name: "Docs", Links: []seo.LLMsLink{
		{Title: "A", URL: "https://a/", Note: "the first"}, {Title: "B", URL: "https://b/"}}})
	if want := "# App\n\n> Sum.\n\nMore here.\n\n## Docs\n\n- [A](https://a/): the first\n- [B](https://b/)\n"; got != want {
		t.Errorf("LLMsTxt:\n%s\nwant:\n%s", got, want)
	}
}

// TestSitemap: one language's pages at the origin, escaped.
func TestSitemap(t *testing.T) {
	got := seo.Sitemap("https://app.example.com", []string{"/", "/a?b=1&c=2"})
	if want := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<url><loc>https://app.example.com/</loc></url>
<url><loc>https://app.example.com/a?b=1&amp;c=2</loc></url>
</urlset>
`; got != want {
		t.Errorf("Sitemap:\n%s\nwant:\n%s", got, want)
	}
}

// TestNoindex: everything that isn't a page is noindex, pages aren't; a preview is noindex throughout.
func TestNoindex(t *testing.T) {
	h := site()
	for _, tc := range []struct {
		method, url string
		noindex     bool
	}{
		{"GET", "https://app.example.com/about", false},
		{"HEAD", "https://app.example.com/about", false},
		{"GET", "https://app.example.com/fragments/stats", true},
		{"GET", "https://app.example.com/robots.txt", false},
		{"GET", "https://app.example.com/sitemap.xml", false},
		{"POST", "https://app.example.com/about", true},
	} {
		if got := do(h, tc.method, tc.url).Header().Get("X-Robots-Tag") == "noindex"; got != tc.noindex {
			t.Errorf("%s %s: noindex %v, want %v", tc.method, tc.url, got, tc.noindex)
		}
	}
	preview := seo.Site{Preview: true}.Handler(http.NotFoundHandler())
	if do(preview, "GET", "https://x.workers.dev/").Header().Get("X-Robots-Tag") != "noindex" {
		t.Error("a preview page is not noindex")
	}
}

// TestOneAddress: a page on another host or over plain http moves to https on Host, path and query kept; what isn't a
// page, a write, a local server, and a site without Host stay where they are.
func TestOneAddress(t *testing.T) {
	h := site()
	for _, tc := range []struct{ method, url, to string }{
		{"GET", "http://app.example.com/de/about?x=1", "https://app.example.com/de/about?x=1"},
		{"GET", "https://app.123.workers.dev/", "https://app.example.com/"},
		{"HEAD", "http://www.app.example.com/board", "https://app.example.com/board"},
		{"GET", "https://app.123.workers.dev/robots.txt", "https://app.example.com/robots.txt"},
		{"GET", "http://app.example.com/sitemap.xml", "https://app.example.com/sitemap.xml"},
		{"GET", "https://app.example.com/about", ""},
		{"GET", "https://app.123.workers.dev/fragments/stats", ""},
		{"POST", "http://app.example.com/about", ""},
		{"GET", "http://localhost:8913/", ""},
		{"GET", "http://127.0.0.1:8913/about", ""},
		{"GET", "http://[::1]:8913/", ""},
	} {
		w := do(h, tc.method, tc.url)
		if got := w.Header().Get("Location"); got != tc.to || tc.to != "" && w.Code != http.StatusMovedPermanently {
			t.Errorf("%s %s: %d → %q, want %q", tc.method, tc.url, w.Code, got, tc.to)
		}
	}
	if w := do(seo.Site{}.Handler(http.NotFoundHandler()), "GET", "http://app.123.workers.dev/"); w.Header().Get("Location") != "" {
		t.Error("a site without Host redirected")
	}
}
