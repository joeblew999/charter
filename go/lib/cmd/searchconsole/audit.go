package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"github.com/joeblew999/charter/go/lib/searchconsole"
)

// googlebotUA is Googlebot Smartphone's user agent: Google indexes mobile-first.
const googlebotUA = "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/130.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

// page is what Google reads from one live page.
type page struct {
	url         string
	status      int
	xRobotsTag  string
	lang        string
	title       string
	description string
	robots      string
	canonical   string
	h1          int
	alternates  map[string]string // hreflang → href
}

// audit fetches robots.txt and every sitemap URL as Googlebot and returns one problem per line (empty = clean).
func audit(ctx context.Context, httpc *http.Client, base, sitemap string, urls []string) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	robots, status, err := fetch(ctx, httpc, strings.TrimRight(base, "/")+"/robots.txt")
	switch {
	case err != nil:
		add("robots.txt: %v", err)
	case status != 200:
		add("robots.txt: HTTP %d", status)
	default:
		for _, p := range robotsProblems(string(robots), sitemap) {
			add("robots.txt: %s", p)
		}
	}

	host := ""
	if u, err := url.Parse(base); err == nil {
		host = u.Host
	}
	pages := make([]page, len(urls))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, u := range urls {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			body, status, err := fetchPage(ctx, httpc, u, &pages[i])
			if err != nil {
				pages[i] = page{url: u, status: -1, title: err.Error()}
				return
			}
			pages[i].url, pages[i].status = u, status
			parsePage(body, &pages[i])
		})
	}
	wg.Wait()

	inSitemap := map[string]bool{}
	for _, u := range urls {
		inSitemap[u] = true
	}
	byURL := map[string]page{}
	for _, p := range pages {
		byURL[p.url] = p
	}
	for _, p := range pages {
		for _, msg := range pageProblems(p, host) {
			add("%s: %s", p.url, msg)
		}
		for lang, href := range p.alternates {
			if lang == "x-default" {
				continue
			}
			if !inSitemap[href] {
				add("%s: hreflang %s → %s is not in the sitemap", p.url, lang, href)
				continue
			}
			if back := byURL[href]; !hasAlternate(back, p.url) {
				add("%s: hreflang %s → %s does not link back (hreflang must be reciprocal)", p.url, lang, href)
			}
		}
	}
	return problems
}

// pageProblems checks one page against what Google needs to index it as its own canonical URL.
func pageProblems(p page, host string) []string {
	var out []string
	if p.status != 200 {
		if p.status == -1 {
			return []string{"fetch failed: " + p.title}
		}
		return []string{fmt.Sprintf("HTTP %d (sitemap URLs must answer 200, not redirect)", p.status)}
	}
	if isNoindex(p.xRobotsTag) {
		out = append(out, "X-Robots-Tag: "+p.xRobotsTag)
	}
	if isNoindex(p.robots) {
		out = append(out, `meta robots "`+p.robots+`"`)
	}
	if p.canonical != p.url {
		out = append(out, fmt.Sprintf("canonical %q is not the page's own URL", p.canonical))
	}
	if p.lang == "" {
		out = append(out, "<html> has no lang")
	}
	switch n := searchconsole.SnippetWidth(p.title); {
	case n == 0:
		out = append(out, "no <title>")
	case n > searchconsole.MaxTitleWidth:
		out = append(out, fmt.Sprintf("title is %d characters wide (Google shows about %d)", n, searchconsole.MaxTitleWidth))
	}
	switch n := searchconsole.SnippetWidth(p.description); {
	case n == 0:
		out = append(out, "no meta description")
	case n > searchconsole.MaxDescriptionWidth:
		out = append(out, fmt.Sprintf("meta description is %d characters wide (Google shows about %d)", n, searchconsole.MaxDescriptionWidth))
	}
	if p.h1 != 1 {
		out = append(out, fmt.Sprintf("%d <h1> elements (want exactly 1)", p.h1))
	}
	if len(p.alternates) > 0 {
		if _, ok := p.alternates["x-default"]; !ok {
			out = append(out, "hreflang set has no x-default")
		}
		if !hasAlternate(p, p.url) {
			out = append(out, "hreflang set does not include the page itself")
		}
		for lang, href := range p.alternates {
			if u, err := url.Parse(href); err != nil || u.Scheme != "https" || u.Host != host {
				out = append(out, fmt.Sprintf("hreflang %s → %q is not an absolute https URL on %s", lang, href, host))
			}
		}
	}
	return out
}

// robotsProblems checks that robots.txt lets Google crawl everything and points at the sitemap.
func robotsProblems(body, sitemap string) []string {
	var out []string
	if !strings.Contains(body, "Sitemap: "+sitemap) {
		out = append(out, "no \"Sitemap: "+sitemap+"\" line")
	}
	for line := range strings.SplitSeq(body, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "Disallow: /") {
			out = append(out, `"Disallow: /" blocks the whole site`)
		}
	}
	return out
}

// hasAlternate reports whether a page lists u under a language (x-default doesn't count: it isn't a language version).
func hasAlternate(p page, u string) bool {
	for lang, href := range p.alternates {
		if lang != "x-default" && href == u {
			return true
		}
	}
	return false
}

// isNoindex reports whether robots directives (meta content or X-Robots-Tag, optionally "googlebot: …") forbid indexing.
func isNoindex(v string) bool {
	for _, d := range strings.FieldsFunc(strings.ToLower(v), func(r rune) bool { return r == ',' || r == ' ' || r == ':' }) {
		if d == "noindex" || d == "none" {
			return true
		}
	}
	return false
}

// parsePage reads the head elements Google uses, plus the <h1> count.
func parsePage(body []byte, p *page) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return
	}
	p.alternates = map[string]string{}
	attr := func(n *html.Node, key string) string {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
		return ""
	}
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "html":
			p.lang = attr(n, "lang")
		case "title":
			if n.FirstChild != nil {
				p.title = strings.TrimSpace(n.FirstChild.Data)
			}
		case "meta":
			switch attr(n, "name") {
			case "description":
				p.description = attr(n, "content")
			case "robots", "googlebot":
				p.robots = strings.TrimSpace(p.robots + " " + attr(n, "content"))
			}
		case "link":
			switch attr(n, "rel") {
			case "canonical":
				p.canonical = attr(n, "href")
			case "alternate":
				if l := attr(n, "hreflang"); l != "" {
					p.alternates[strings.ToLower(l)] = attr(n, "href")
				}
			}
		case "h1":
			p.h1++
		}
	}
}

// fetchPage GETs a URL as Googlebot without following redirects and records X-Robots-Tag.
func fetchPage(ctx context.Context, httpc *http.Client, u string, p *page) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", googlebotUA)
	noRedirect := *httpc
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := noRedirect.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	p.xRobotsTag = strings.Join(res.Header.Values("X-Robots-Tag"), ", ")
	b, err := io.ReadAll(res.Body)
	return b, res.StatusCode, err
}

func fetch(ctx context.Context, httpc *http.Client, u string) ([]byte, int, error) {
	var p page
	return fetchPage(ctx, httpc, u, &p)
}
