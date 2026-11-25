package seo

import (
	"context"
	"fmt"

	"github.com/joeblew999/charter/internal/seokit/searchconsole"
)

// inspectReport is charter seo inspect --json.
type inspectReport struct {
	URL string `json:"url"`
	searchconsole.Inspection
	Link string `json:"link"`
}

// inspect is charter seo inspect (seo:inspect): one URL, the home page or the one named (a path, /de/, or a full URL),
// as Google's index has it: its verdict, coverage, last crawl, Google's canonical and the app's, and its Search
// Console page (--open opens it), where Test live URL and Request indexing are.
func (r repo) inspect(args []string) {
	f := flags("inspect")
	jsonOut := f.Bool("json", false, "one JSON document on stdout")
	open := f.Bool("open", false, "open its Search Console page")
	page := append(parse(f, args, 1), "/")[0]
	c := r.conf()
	site := r.siteOf(c, "")
	ctx := context.Background()
	sc := consoleWithAccess(ctx, c, site)
	u := absolute(site, page)
	in, err := sc.Inspect(ctx, u, "en")
	must(err)
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
}
