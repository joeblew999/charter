// Package audit reads a live site as Googlebot does, robots.txt, the sitemap and every page in it, and returns what
// would keep a page out of Google's index or look wrong in a result: what charter seo audit runs, and what an app's own
// tests run on a local build (Audit), so a page Google couldn't index fails before it ships.
package audit

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joeblew999/charter/seo"

	"golang.org/x/net/html"
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
	icon        bool              // <link rel="icon">: the site's icon beside its results
	ogTitle     string
	ogDesc      string
	ogURL       string
	webSiteName string // the name in WebSite structured data (JSON-LD), the site name Google shows
	location    string // a redirect's target
}

// Audit fetches robots.txt, the sitemap at base/sitemap.xml and every URL it lists as Googlebot, and returns how many
// URLs the sitemap lists and every problem that would keep a page out of Google's index, one per line (none: the site
// is clean). err is for a sitemap it can't read at all.
func Audit(ctx context.Context, httpc *http.Client, base string) (urls int, problems []string, err error) {
	sitemap := strings.TrimRight(base, "/") + "/sitemap.xml"
	list, err := SitemapURLs(ctx, httpc, sitemap)
	if err != nil {
		return 0, nil, err
	}
	return len(list), audit(ctx, httpc, base, sitemap, list), nil
}

// audit checks robots.txt and the sitemap's urls; one problem per line.
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

	// llms.txt is optional; when the site has one, it has to be right.
	if llms, status, err := fetch(ctx, httpc, strings.TrimRight(base, "/")+"/llms.txt"); err == nil && status == 200 {
		for _, p := range llmsProblems(ctx, httpc, base, string(llms)) {
			add("llms.txt: %s", p)
		}
	}

	site, err := url.Parse(base)
	if err != nil {
		site = &url.URL{}
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

	for _, msg := range siteProblems(ctx, httpc, site, pages) {
		add("%s", msg)
	}

	inSitemap := map[string]bool{}
	for _, u := range urls {
		inSitemap[u] = true
	}
	byURL := map[string]page{}
	for _, p := range pages {
		byURL[p.url] = p
	}
	for _, p := range pages {
		for _, msg := range pageProblems(p, site) {
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

// invisible reports whether r is a character that shows nothing but changes text: bidi marks and isolates, the zero
// width ones, a byte order mark.
func invisible(r rune) bool {
	return r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\u200e' || r == '\u200f' ||
		r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069' || r == '\ufeff'
}

// pageProblems checks one page against what Google needs to index it as its own canonical URL.
func pageProblems(p page, site *url.URL) []string {
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
	switch n := seo.SnippetWidth(p.title); {
	case n == 0:
		out = append(out, "no <title>")
	case n > seo.MaxTitleWidth:
		out = append(out, fmt.Sprintf("title is %d characters wide (Google shows about %d)", n, seo.MaxTitleWidth))
	}
	switch n := seo.SnippetWidth(p.description); {
	case n == 0:
		out = append(out, "no meta description")
	case n > seo.MaxDescriptionWidth:
		out = append(out, fmt.Sprintf("meta description is %d characters wide (Google shows about %d)", n, seo.MaxDescriptionWidth))
	}
	if p.h1 != 1 {
		out = append(out, fmt.Sprintf("%d <h1> elements (want exactly 1)", p.h1))
	}
	for what, s := range map[string]string{"title": p.title, "meta description": p.description, "og:title": p.ogTitle} {
		if strings.ContainsFunc(s, invisible) {
			out = append(out, fmt.Sprintf("%s %q has invisible characters (bidi marks, a BOM): they show in results and previews", what, s))
		}
	}
	if !p.icon {
		out = append(out, `no <link rel="icon">: Google shows a generic icon beside the site's results`)
	}
	switch {
	case p.ogTitle == "" || p.ogDesc == "":
		out = append(out, "no og:title or og:description: a shared link has no title or text")
	case p.ogURL != p.url:
		out = append(out, fmt.Sprintf("og:url %q is not the page's own URL", p.ogURL))
	}
	if len(p.alternates) > 0 {
		if _, ok := p.alternates["x-default"]; !ok {
			out = append(out, "hreflang set has no x-default")
		}
		if !hasAlternate(p, p.url) {
			out = append(out, "hreflang set does not include the page itself")
		}
		for lang, href := range p.alternates {
			if u, err := url.Parse(href); err != nil || u.Scheme != site.Scheme || u.Host != site.Host {
				out = append(out, fmt.Sprintf("hreflang %s → %q is not an absolute URL on %s://%s", lang, href, site.Scheme, site.Host))
			}
		}
	}
	return out
}

// llmsLink is a Markdown link's absolute URL.
var llmsLink = regexp.MustCompile(`\]\((https?://[^)\s]+)\)`)

// llmsProblems checks an llms.txt as llmstxt.org lays it out: a "# " title first, a "> " summary, and every link to
// this site answering 200 (links elsewhere aren't fetched).
func llmsProblems(ctx context.Context, httpc *http.Client, base, body string) []string {
	var out []string
	if !strings.HasPrefix(body, "# ") {
		out = append(out, `doesn't start with a "# " title`)
	}
	if !strings.Contains(body, "\n> ") {
		out = append(out, `no "> " summary`)
	}
	site, err := url.Parse(base)
	if err != nil {
		return out
	}
	for _, m := range llmsLink.FindAllStringSubmatch(body, -1) {
		if u, err := url.Parse(m[1]); err != nil || u.Host != site.Host {
			continue
		}
		if _, status, err := fetch(ctx, httpc, m[1]); err != nil {
			out = append(out, fmt.Sprintf("%s: %v", m[1], err))
		} else if status != 200 {
			out = append(out, fmt.Sprintf("%s: HTTP %d", m[1], status))
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
		case "script":
			if attr(n, "type") == "application/ld+json" && n.FirstChild != nil {
				var ld struct {
					Type string `json:"@type"`
					Name string `json:"name"`
				}
				if json.Unmarshal([]byte(n.FirstChild.Data), &ld) == nil && ld.Type == "WebSite" {
					p.webSiteName = ld.Name
				}
			}
		}
		if n.Data == "link" && slices.Contains(strings.Fields(attr(n, "rel")), "icon") {
			p.icon = true
		}
		if n.Data == "meta" {
			switch attr(n, "property") {
			case "og:title":
				p.ogTitle = attr(n, "content")
			case "og:description":
				p.ogDesc = attr(n, "content")
			case "og:url":
				p.ogURL = attr(n, "content")
			}
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
	p.location = res.Header.Get("Location")
	b, err := io.ReadAll(res.Body)
	return b, res.StatusCode, err
}

func fetch(ctx context.Context, httpc *http.Client, u string) ([]byte, int, error) {
	var p page
	return fetchPage(ctx, httpc, u, &p)
}

// SitemapURLs fetches a live sitemap and returns its <loc> URLs.
func SitemapURLs(ctx context.Context, httpc *http.Client, sitemap string) ([]string, error) {
	entries, err := sitemapEntries(ctx, httpc, sitemap)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Loc)
	}
	return out, err
}

// SitemapPages fetches a live sitemap and returns one URL per page: the x-default of each entry's language versions
// (its hreflang links), else the entry itself, each once and in order. A site in fourteen languages has fourteen
// entries per page and one URL here.
func SitemapPages(ctx context.Context, httpc *http.Client, sitemap string) ([]string, error) {
	entries, err := sitemapEntries(ctx, httpc, sitemap)
	var out []string
	seen := map[string]bool{}
	for _, e := range entries {
		page := e.Loc
		for _, l := range e.Links {
			if l.Rel == "alternate" && l.Hreflang == "x-default" {
				page = l.Href
			}
		}
		if !seen[page] {
			seen[page] = true
			out = append(out, page)
		}
	}
	return out, err
}

// sitemapEntry is one <url> of a sitemap, with its hreflang links (xhtml:link).
type sitemapEntry struct {
	Loc   string `xml:"loc"`
	Links []struct {
		Rel      string `xml:"rel,attr"`
		Hreflang string `xml:"hreflang,attr"`
		Href     string `xml:"href,attr"`
	} `xml:"link"`
}

func sitemapEntries(ctx context.Context, httpc *http.Client, sitemap string) ([]sitemapEntry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", sitemap, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: %s", sitemap, res.Status)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var set struct {
		URLs []sitemapEntry `xml:"url"`
	}
	if err := xml.Unmarshal(b, &set); err != nil {
		return nil, fmt.Errorf("%s: %w", sitemap, err)
	}
	return set.URLs, nil
}

// siteProblems checks the site as a whole: a page that doesn't exist answers 404 (a "soft 404" that answers 200
// makes Google index nonsense); plain http moves to https; the home page names the site (WebSite structured data).
func siteProblems(ctx context.Context, httpc *http.Client, site *url.URL, pages []page) []string {
	var out []string
	home := site.Scheme + "://" + site.Host + "/"
	missing := home + "seo-audit-no-such-page-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if _, status, err := fetch(ctx, httpc, missing); err == nil && status != 404 && status != 410 {
		out = append(out, fmt.Sprintf("%s: HTTP %d for a page that doesn't exist (want 404: a soft 404 gets indexed)", missing, status))
	}
	if site.Scheme == "https" && site.Port() == "" { // plain http is port 80; a test server's own port has none
		var p page
		_, status, err := fetchPage(ctx, httpc, "http://"+site.Host+"/", &p)
		if err == nil && (status < 300 || status > 399 || !strings.HasPrefix(p.location, "https://"+site.Host+"/")) {
			out = append(out, fmt.Sprintf("http://%s/: HTTP %d, not a redirect to https: every page has two addresses", site.Host, status))
		}
	}
	for _, p := range pages {
		if p.url == home && p.status == 200 && p.webSiteName == "" {
			out = append(out, home+`: no WebSite structured data with a name (the site name Google shows beside results)`)
		}
	}
	return out
}
