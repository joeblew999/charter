package audit

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// site serves a two-locale site (/ and /de/) whose pages can be broken one way at a time.
func site(t *testing.T, breakWith string) (base string, urls []string, client *http.Client) {
	t.Helper()
	var srv *httptest.Server
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := srv.URL
		switch r.URL.Path {
		case "/robots.txt":
			body := "User-agent: *\nAllow: /\n\nSitemap: " + b + "/sitemap.xml\n"
			if breakWith == "robots-disallow" {
				body = "User-agent: *\nDisallow: /\n\nSitemap: " + b + "/sitemap.xml\n"
			}
			fmt.Fprint(w, body)
		case "/llms.txt":
			switch breakWith {
			case "llms":
				fmt.Fprint(w, "# Example\n\n> A site.\n\n## Pages\n\n- [Home]("+b+"/)\n- [Elsewhere](https://example.org/x)\n")
			case "llms-broken-link":
				fmt.Fprint(w, "# Example\n\n> A site.\n\n## Pages\n\n- [Gone]("+b+"/gone)\n")
			case "llms-no-title":
				fmt.Fprint(w, "Example\n\n> A site.\n")
			default:
				http.NotFound(w, r)
			}
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
			case breakWith == "other-scheme" && lang == "de":
				back = `<link rel="alternate" hreflang="en" href="http://` + strings.TrimPrefix(other, "https://") + `">`
			}
			title, icon, og := "Home", `<link rel="icon" href="/static/icon.svg">`, `<meta property="og:title" content="Home"><meta property="og:description" content="A page."><meta property="og:url" content="`+canonical+`">`
			website := ""
			if lang == "en" && breakWith != "no-website" {
				website = `<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite","name":"Example","url":"` + b + `/"}</script>`
			}
			switch {
			case breakWith == "invisible" && lang == "de":
				title = "\u2068Start\u2069 · Example"
			case breakWith == "no-icon" && lang == "de":
				icon = ""
			case breakWith == "no-og" && lang == "de":
				og = ""
			}
			fmt.Fprintf(w, `<!doctype html><html lang="%s"><head><title>%s</title>
<meta name="description" content="A page."><link rel="canonical" href="%s">%s%s%s
<link rel="alternate" hreflang="%s" href="%s">%s<link rel="alternate" hreflang="x-default" href="%s/">
</head><body>%s</body></html>`, lang, title, canonical, icon, og, website, lang, self, back, b, h1)
		default:
			if breakWith == "soft-404" {
				fmt.Fprint(w, "<!doctype html><title>Home</title>")
				return
			}
			http.NotFound(w, r)
		}
	})
	// A site served over plain HTTP (a local build) links on http: just as well; Google's own are https.
	if breakWith == "plain-http" {
		srv = httptest.NewServer(h)
	} else {
		srv = httptest.NewTLSServer(h)
	}
	t.Cleanup(srv.Close)
	return srv.URL, []string{srv.URL + "/", srv.URL + "/de/"}, srv.Client()
}

func TestAudit(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"robots-disallow":  `"Disallow: /" blocks the whole site`,
		"redirect":         "HTTP 301 (sitemap URLs must answer 200, not redirect)",
		"canonical":        "is not the page's own URL",
		"two-h1":           "2 <h1> elements",
		"one-way":          "does not link back",
		"noindex-header":   "X-Robots-Tag: noindex",
		"other-scheme":     "is not an absolute URL on https://",
		"invisible":        "has invisible characters",
		"no-icon":          `no <link rel="icon">`,
		"no-og":            "no og:title or og:description",
		"no-website":       "no WebSite structured data",
		"soft-404":         "for a page that doesn't exist (want 404",
		"plain-http":       "",
		"llms":             "",
		"llms-broken-link": "/gone: HTTP 404",
		"llms-no-title":    `llms.txt: doesn't start with a "# " title`,
	}
	for breakWith, want := range cases {
		t.Run(cmp.Or(breakWith, "clean"), func(t *testing.T) {
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
	got := strings.Join(pageProblems(p, &url.URL{Scheme: "https", Host: "a.example"}), "\n")
	for _, want := range []string{"title is 61 characters wide", "meta description is 161 characters wide"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
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

// TestSitemapPages: one URL per page, the x-default of its language versions.
func TestSitemapPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
<url><loc>https://a.example/</loc><xhtml:link rel="alternate" hreflang="de" href="https://a.example/de/"/><xhtml:link rel="alternate" hreflang="x-default" href="https://a.example/"/></url>
<url><loc>https://a.example/de/</loc><xhtml:link rel="alternate" hreflang="x-default" href="https://a.example/"/></url>
<url><loc>https://a.example/about</loc><xhtml:link rel="alternate" hreflang="x-default" href="https://a.example/about"/></url>
<url><loc>https://a.example/solo</loc></url>
</urlset>`)
	}))
	defer srv.Close()
	got, err := SitemapPages(context.Background(), srv.Client(), srv.URL)
	if want := []string{"https://a.example/", "https://a.example/about", "https://a.example/solo"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("SitemapPages = %q, %v; want %q", got, err, want)
	}
}
