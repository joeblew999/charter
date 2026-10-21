// Command searchconsole reports and nudges how Google indexes a site, through the Search Console API (package
// searchconsole), and audits it live as Googlebot sees it:
//
//	fnox exec -- go tool searchconsole sites                # properties the service account can use; exit 1 without Full access
//	fnox exec -- go tool searchconsole submit               # submit <base>/sitemap.xml
//	fnox exec -- go tool searchconsole status               # sitemap status + index status of every sitemap URL
//	fnox exec -- go tool searchconsole [-open] todo         # what Google hasn't indexed; exit 1 (and -open) on problems
//	fnox exec -- go tool searchconsole [-open] inspect /de/ # one URL: Google's verdict + its Search Console page
//	go tool searchconsole audit                             # live fetch as Googlebot: robots.txt + every sitemap URL
//
// -json prints one JSON document on stdout instead of text (same data, for agents, CI and jq) and never opens a browser.
// Exit codes are the contract between tasks: 0 ok, 1 problems (or no access), 2 usage.
//
// The key is GOOGLE_SEARCH_CONSOLE_KEY (a service account JSON key, e.g. from fnox); audit needs none. -base defaults
// to https://$APP_DOMAIN and -site to the Domain property of its registrable domain (app.example.com →
// sc-domain:example.com). Request indexing has no API: todo prints Google's link for each problem (-open opens only those).
package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/joeblew999/charter/go/lib/searchconsole"
)

// JSON reports (-json). Field names are the stable interface; text output renders the same values.
type (
	auditReport struct {
		Base     string   `json:"base"`
		URLs     int      `json:"urls"`
		OK       bool     `json:"ok"`
		Problems []string `json:"problems"`
	}
	sitesReport struct {
		Site   string               `json:"site"`
		Access bool                 `json:"access"` // Full or Owner on Site
		Sites  []searchconsole.Site `json:"sites"`
	}
	submitReport struct {
		Site    string `json:"site"`
		Sitemap string `json:"sitemap"`
	}
	urlStatus struct {
		URL             string `json:"url"`
		Verdict         string `json:"verdict,omitempty"`
		Coverage        string `json:"coverage,omitempty"`
		LastCrawl       string `json:"lastCrawl,omitempty"`
		GoogleCanonical string `json:"googleCanonical,omitempty"`
		UserCanonical   string `json:"userCanonical,omitempty"`
		Link            string `json:"link,omitempty"`
		Error           string `json:"error,omitempty"`
	}
	statusReport struct {
		Sitemap      *searchconsole.Sitemap `json:"sitemap"`
		SitemapError string                 `json:"sitemapError,omitempty"`
		URLs         []urlStatus            `json:"urls"`
		Counts       map[string]int         `json:"counts"`
	}
	inspectReport struct {
		URL string `json:"url"`
		searchconsole.Inspection
		Link string `json:"link"`
	}
	todoReport struct {
		Total    int       `json:"total"`
		Indexed  int       `json:"indexed"`
		Problems int       `json:"problems"`
		Findings []finding `json:"findings"`
	}
)

func main() {
	domain := os.Getenv("APP_DOMAIN")
	site := flag.String("site", domainProperty(domain), "Search Console property (sc-domain:… or a URL prefix)")
	base := flag.String("base", "https://"+domain, "the app's base URL (sitemap at <base>/sitemap.xml)")
	open := flag.Bool("open", false, "todo, inspect: open Google's Search Console page (todo: only for problem URLs)")
	jsonOut := flag.Bool("json", false, "print one JSON document on stdout instead of text; never opens a browser")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchconsole [-site sc-domain:example.com] [-base https://app.example.com] [-open] [-json] sites|submit|status|todo|audit|inspect <path or URL>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 && !(flag.NArg() == 2 && flag.Arg(0) == "inspect") || domain == "" && (*site == "" || *base == "https://") {
		flag.Usage()
		os.Exit(2)
	}
	if *jsonOut {
		*open = false
	}
	ctx := context.Background()
	httpc := &http.Client{Timeout: 60 * time.Second}
	sitemap := strings.TrimRight(*base, "/") + "/sitemap.xml"
	if flag.Arg(0) == "audit" {
		urls, err := sitemapURLs(ctx, httpc, sitemap)
		if err != nil {
			fail(err)
		}
		r := auditReport{Base: *base, URLs: len(urls), Problems: audit(ctx, httpc, *base, sitemap, urls)}
		r.OK = len(r.Problems) == 0
		if r.Problems == nil {
			r.Problems = []string{}
		}
		if *jsonOut {
			emit(r)
		} else if r.OK {
			fmt.Printf("✓ robots.txt and all %d sitemap URLs pass (fetched as Googlebot: 200, indexable, self-canonical, lang, "+
				"title, description, one h1, reciprocal hreflang with x-default)\n", r.URLs)
		} else {
			for _, p := range r.Problems {
				fmt.Println("✗ " + p)
			}
			fmt.Printf("%d problem(s) across robots.txt and %d sitemap URLs (fetched as Googlebot)\n", len(r.Problems), r.URLs)
		}
		if !r.OK {
			os.Exit(1)
		}
		return
	}
	raw := os.Getenv("GOOGLE_SEARCH_CONSOLE_KEY")
	if raw == "" {
		fail(fmt.Errorf("GOOGLE_SEARCH_CONSOLE_KEY is not set (run through fnox exec)"))
	}
	key, err := searchconsole.ParseKey([]byte(raw))
	if err != nil {
		fail(err)
	}
	token, err := key.Token(ctx, httpc, searchconsole.Scope)
	if err != nil {
		fail(err)
	}
	c := &searchconsole.Client{HTTP: httpc, Token: token, Site: *site}

	switch flag.Arg(0) {
	case "sites":
		sites, err := c.Sites(ctx)
		if err != nil {
			fail(err)
		}
		r := sitesReport{Site: *site, Sites: sites}
		if r.Sites == nil {
			r.Sites = []searchconsole.Site{}
		}
		for _, s := range sites {
			if s.SiteURL == *site && (s.PermissionLevel == "siteOwner" || s.PermissionLevel == "siteFullUser") {
				r.Access = true
			}
		}
		if *jsonOut {
			emit(r)
		} else {
			for _, s := range sites {
				fmt.Printf("%s\t%s\n", s.SiteURL, s.PermissionLevel)
			}
			if !r.Access {
				fmt.Printf("✗ %s has no Full access to %s: Search Console → Settings → Users and permissions → Add user (Full)\n", key.ClientEmail, *site)
			}
		}
		if !r.Access {
			os.Exit(1)
		}
	case "submit":
		if err := c.SubmitSitemap(ctx, sitemap); err != nil {
			fail(err)
		}
		if *jsonOut {
			emit(submitReport{Site: *site, Sitemap: sitemap})
		} else {
			fmt.Printf("✓ submitted %s to %s\n", sitemap, *site)
		}
	case "status":
		status(ctx, c, httpc, sitemap, *jsonOut)
	case "inspect":
		u := flag.Arg(1)
		if strings.HasPrefix(u, "/") {
			u = strings.TrimRight(*base, "/") + u
		}
		in, err := c.Inspect(ctx, u, "en")
		if err != nil {
			fail(err)
		}
		if *jsonOut {
			emit(inspectReport{URL: u, Inspection: in, Link: in.Link})
			return
		}
		fmt.Printf("%s\n  verdict    %s\n  coverage   %s\n  last crawl %s as %s (fetch %s, robots.txt %s)\n  canonical  Google: %s, yours: %s\n  %s\n",
			u, in.Verdict, in.CoverageState, or(in.LastCrawlTime, "never"), or(in.CrawledAs, "-"), or(in.PageFetchState, "-"),
			or(in.RobotsTxtState, "-"), or(in.GoogleCanonical, "-"), or(in.UserCanonical, "-"), in.Link)
		fmt.Println("  On that page: TEST LIVE URL (Google fetches the page now), then REQUEST INDEXING if it isn't indexed.")
		if *open && in.Link != "" {
			openInBrowser([]string{in.Link})
		}
	case "todo":
		urls, err := sitemapURLs(ctx, httpc, sitemap)
		if err != nil {
			fail(err)
		}
		indexed, findings := todo(ctx, c, urls)
		r := todoReport{Total: len(urls), Indexed: indexed, Findings: findings}
		if r.Findings == nil {
			r.Findings = []finding{}
		}
		for _, f := range findings {
			if f.Problem {
				r.Problems++
			}
		}
		if *jsonOut {
			emit(r)
		} else {
			links := printTodo(r.Total, r.Indexed, r.Findings)
			if r.Problems > 0 {
				fmt.Printf("%d problem(s) Google reports\n", r.Problems)
				if *open {
					openInBrowser(links)
				}
			}
		}
		if r.Problems > 0 {
			os.Exit(1)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}
}

// emit writes v as indented JSON on stdout.
func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fail(err)
	}
}

func status(ctx context.Context, c *searchconsole.Client, httpc *http.Client, sitemap string, jsonOut bool) {
	r := statusReport{Counts: map[string]int{}}
	if s, err := c.GetSitemap(ctx, sitemap); err != nil {
		r.SitemapError = err.Error()
	} else {
		r.Sitemap = &s
	}
	if !jsonOut {
		if r.Sitemap == nil {
			fmt.Printf("sitemap %s: %s\n", sitemap, r.SitemapError)
		} else {
			s := r.Sitemap
			fmt.Printf("sitemap %s: submitted %s, downloaded %s, pending %v, errors %s, warnings %s\n",
				s.Path, or(s.LastSubmitted, "-"), or(s.LastDownloaded, "never"), s.IsPending, or(s.Errors, "0"), or(s.Warnings, "0"))
			for _, ct := range s.Contents {
				fmt.Printf("  %s: %s submitted, %s indexed\n", ct.Type, ct.Submitted, or(ct.Indexed, "?"))
			}
		}
	}
	urls, err := sitemapURLs(ctx, httpc, sitemap)
	if err != nil {
		fail(err)
	}
	// The URL Inspection API takes seconds per URL; run a few at once (quota: 600 per minute, 2,000 per day) and, as text,
	// print each row as soon as it and the rows before it are done.
	rows := make([]chan urlStatus, len(urls))
	sem := make(chan struct{}, 6)
	for i, u := range urls {
		rows[i] = make(chan urlStatus, 1)
		go func(u string, out chan<- urlStatus) {
			sem <- struct{}{}
			defer func() { <-sem }()
			in, err := c.Inspect(ctx, u, "en")
			if err != nil {
				out <- urlStatus{URL: u, Error: err.Error()}
				return
			}
			out <- urlStatus{URL: u, Verdict: in.Verdict, Coverage: in.CoverageState, LastCrawl: in.LastCrawlTime,
				GoogleCanonical: in.GoogleCanonical, UserCanonical: in.UserCanonical, Link: in.Link}
		}(u, rows[i])
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.FilterHTML)
	if !jsonOut {
		fmt.Fprintln(w, "URL\tVERDICT\tCOVERAGE\tLAST CRAWL\tCANONICAL")
	}
	problems := 0
	for _, ch := range rows {
		u := <-ch
		r.URLs = append(r.URLs, u)
		state := u.Coverage
		if u.Error != "" {
			state = "error"
		}
		r.Counts[state]++
		canonical := "ok"
		if u.Error != "" || u.GoogleCanonical != "" && u.GoogleCanonical != u.URL {
			problems++
			canonical = "Google chose " + u.GoogleCanonical
		}
		if !jsonOut {
			if u.Error != "" {
				fmt.Fprintf(w, "%s\tERROR\t%s\t\t\n", u.URL, u.Error)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", u.URL, u.Verdict, u.Coverage, or(u.LastCrawl, "-"), canonical)
			}
			w.Flush()
		}
	}
	if jsonOut {
		emit(r)
		return
	}
	for state, n := range r.Counts {
		fmt.Printf("  %3d  %s\n", n, state)
	}
	fmt.Printf("%d URLs inspected (Google's index, not a live fetch); %d with errors or a different canonical\n", len(urls), problems)
}

// sitemapURLs fetches the live sitemap and returns its <loc> URLs.
func sitemapURLs(ctx context.Context, httpc *http.Client, sitemap string) ([]string, error) {
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
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(b, &set); err != nil {
		return nil, fmt.Errorf("%s: %w", sitemap, err)
	}
	out := make([]string, 0, len(set.URLs))
	for _, u := range set.URLs {
		out = append(out, u.Loc)
	}
	return out, nil
}

// domainProperty is the Domain property of a host's registrable domain: app.example.com →
// sc-domain:example.com (last two labels; good enough for .net/.com hosts, pass -site otherwise).
func domainProperty(host string) string {
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return ""
	}
	return "sc-domain:" + strings.Join(labels[len(labels)-2:], ".")
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "searchconsole:", err)
	os.Exit(1)
}
