package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joeblew999/charter/go/lib/searchconsole"
)

// site serves a two-locale site (/ and /de/) whose pages can be broken one way at a time.
func site(t *testing.T, breakWith string) (base string, urls []string, client *http.Client) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := srv.URL
		switch r.URL.Path {
		case "/robots.txt":
			body := "User-agent: *\nAllow: /\n\nSitemap: " + b + "/sitemap.xml\n"
			if breakWith == "robots-disallow" {
				body = "User-agent: *\nDisallow: /\n\nSitemap: " + b + "/sitemap.xml\n"
			}
			fmt.Fprint(w, body)
		case "/", "/de/":
			if breakWith == "redirect" && r.URL.Path == "/de/" {
				http.Redirect(w, r, "/", http.StatusMovedPermanently)
				return
			}
			lang, self, other, otherLang := "en", b+"/", b+"/de/", "de"
			if r.URL.Path == "/de/" {
				lang, self, other, otherLang = "de", b+"/de/", b+"/", "en"
			}
			canonical, h1, back := self, "<h1>Hi</h1>", `<link rel="alternate" hreflang="`+otherLang+`" href="`+other+`">`
			switch {
			case breakWith == "canonical" && lang == "de":
				canonical = b + "/"
			case breakWith == "two-h1" && lang == "en":
				h1 += "<h1>again</h1>"
			case breakWith == "one-way" && lang == "de":
				back = ""
			case breakWith == "noindex-header" && lang == "en":
				w.Header().Set("X-Robots-Tag", "noindex")
			}
			fmt.Fprintf(w, `<!doctype html><html lang="%s"><head><title>Home</title>
<meta name="description" content="A page."><link rel="canonical" href="%s">
<link rel="alternate" hreflang="%s" href="%s">%s<link rel="alternate" hreflang="x-default" href="%s/">
</head><body>%s</body></html>`, lang, canonical, lang, self, back, b, h1)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, []string{srv.URL + "/", srv.URL + "/de/"}, srv.Client()
}

func TestAudit(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"robots-disallow": `"Disallow: /" blocks the whole site`,
		"redirect":        "HTTP 301 (sitemap URLs must answer 200, not redirect)",
		"canonical":       "is not the page's own URL",
		"two-h1":          "2 <h1> elements",
		"one-way":         "does not link back",
		"noindex-header":  "X-Robots-Tag: noindex",
	}
	for breakWith, want := range cases {
		t.Run(or(breakWith, "clean"), func(t *testing.T) {
			base, urls, client := site(t, breakWith)
			problems := audit(context.Background(), client, base, base+"/sitemap.xml", urls)
			got := strings.Join(problems, "\n")
			if want == "" && len(problems) > 0 {
				t.Fatalf("clean site has problems:\n%s", got)
			}
			if want != "" && !strings.Contains(got, want) {
				t.Fatalf("want a problem containing %q, got:\n%s", want, got)
			}
		})
	}
}

func TestPageProblemsLengths(t *testing.T) {
	p := page{url: "https://a.example/", status: 200, lang: "en", canonical: "https://a.example/", h1: 1,
		title: strings.Repeat("t", 61), description: strings.Repeat("d", 161)}
	got := strings.Join(pageProblems(p, "a.example"), "\n")
	for _, want := range []string{"title is 61 characters wide", "meta description is 161 characters wide"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	}
}

func TestClassify(t *testing.T) {
	const u = "https://app.example.com/en-in/about"
	const link = "https://search.google.com/search-console/inspect?resource_id=sc-domain:example.com&id=abc123"
	for _, tc := range []struct {
		name          string
		in            searchconsole.Inspection
		show, problem bool
	}{
		{"indexed", searchconsole.Inspection{Verdict: "PASS", CoverageState: "Submitted and indexed"}, false, false},
		{"waiting", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Discovered - currently not indexed", Link: link}, true, false},
		{"unknown", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "URL is unknown to Google", Link: link}, true, false},
		{"duplicate", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Duplicate, Google chose different canonical than user",
			GoogleCanonical: "https://app.example.com/about", Link: link}, true, false},
		{"404", searchconsole.Inspection{Verdict: "FAIL", CoverageState: "Not found (404)", Link: link}, true, true},
		{"noindex", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Excluded by ‘noindex’ tag", Link: link}, true, true},
		{"crawled not indexed", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Crawled - currently not indexed", Link: link}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, show := classify(u, tc.in)
			if show != tc.show || f.Problem != tc.problem {
				t.Fatalf("classify = show %v problem %v, want show %v problem %v (%+v)", show, f.Problem, tc.show, tc.problem, f)
			}
			if show && f.Link != tc.in.Link {
				t.Errorf("link = %q, want Google's %q", f.Link, tc.in.Link)
			}
		})
	}
}

func TestIsNoindex(t *testing.T) {
	for v, want := range map[string]bool{"noindex": true, "googlebot: noindex, nofollow": true, "none": true,
		"index, follow": false, "max-snippet:-1": false, "": false, "nonexistent": false} {
		if got := isNoindex(v); got != want {
			t.Errorf("isNoindex(%q) = %v, want %v", v, got, want)
		}
	}
}
