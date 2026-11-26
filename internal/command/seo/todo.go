package seo

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joeblew999/charter/internal/seokit/searchconsole"
	"github.com/joeblew999/charter/seo/audit"
)

// todoReport is charter run seo:search todo --json.
type todoReport struct {
	Total    int       `json:"total"`
	Indexed  int       `json:"indexed"`
	Problems int       `json:"problems"`
	Findings []finding `json:"findings"`
}

// todo is charter run seo:search todo: Google's verdict for every sitemap URL, from its index (not a live fetch):
// indexed as its own page, waiting for a crawl, a duplicate of another, or a problem, each with its Search Console
// link; exit 1 on a problem, and --open opens each problem's page there (Test live URL, Request indexing, which have no
// API).
func (r repo) todo(args []string) {
	f := flags("todo")
	jsonOut := f.Bool("json", false, "one JSON document on stdout")
	open := f.Bool("open", false, "open Google's Search Console page of each problem")
	parse(f, args, 0)
	c := r.conf()
	site := r.siteOf(c, "")
	ctx := context.Background()
	sc := consoleWithAccess(ctx, c, site)
	urls, err := audit.SitemapURLs(ctx, &http.Client{Timeout: time.Minute}, site+"/sitemap.xml")
	must(err)
	indexed, findings := inspectAll(ctx, sc, urls)
	rep := todoReport{Total: len(urls), Indexed: indexed, Findings: append([]finding{}, findings...)}
	for _, fd := range findings {
		if fd.Problem {
			rep.Problems++
		}
	}
	if *jsonOut {
		emit(rep)
	} else {
		links := printTodo(rep.Total, rep.Indexed, rep.Findings)
		if rep.Problems > 0 {
			fmt.Printf("%d problem(s) Google reports\n", rep.Problems)
			if *open {
				openInBrowser(links)
			}
		}
	}
	if rep.Problems > 0 {
		os.Exit(1)
	}
}

// finding is a sitemap URL Google hasn't indexed as its own page. Only problems need a person.
type finding struct {
	URL     string `json:"url"`
	State   string `json:"state"`
	Note    string `json:"note"`
	Link    string `json:"link,omitempty"`
	Problem bool   `json:"problem"`
}

// inspectAll inspects every sitemap URL in Google's index, six at a time. It returns the number indexed and the rest as
// findings.
func inspectAll(ctx context.Context, c *searchconsole.Client, urls []string) (indexed int, findings []finding) {
	results := make([]searchconsole.Inspection, len(urls))
	errs := make([]error, len(urls))
	sem := make(chan struct{}, 6)
	done := make(chan struct{})
	for i, u := range urls {
		go func() {
			sem <- struct{}{}
			defer func() { <-sem; done <- struct{}{} }()
			results[i], errs[i] = c.Inspect(ctx, u, "en")
		}()
	}
	for range urls {
		<-done
	}
	for i, u := range urls {
		if errs[i] != nil {
			findings = append(findings, finding{u, "inspection failed", errs[i].Error(), "", true})
			continue
		}
		if f, ok := classify(u, results[i]); ok {
			findings = append(findings, f)
		} else {
			indexed++
		}
	}
	return indexed, findings
}

// classify sorts one URL's inspection: ok=false means indexed as its own canonical page. Waiting for a crawl and a
// same-content duplicate are expected; anything else Google reports (404, noindex, blocked, a FAIL verdict) is a problem.
func classify(u string, in searchconsole.Inspection) (finding, bool) {
	fail := in.Verdict == "FAIL"
	if in.GoogleCanonical != "" && in.GoogleCanonical != u {
		return finding{u, in.CoverageState, "Google indexes " + in.GoogleCanonical + " instead (same content, e.g. a " +
			"same-language regional copy; hreflang still sends those users here)", in.Link, fail}, true
	}
	switch in.CoverageState {
	case "Submitted and indexed", "Indexed, not submitted in sitemap":
		if !fail {
			return finding{}, false
		}
	case "Discovered - currently not indexed", "URL is unknown to Google":
		if !fail {
			return finding{u, in.CoverageState, "waiting for Google to crawl it from the sitemap: nothing to do", in.Link, false}, true
		}
	}
	return finding{u, in.CoverageState, "Google reports a problem: open the link, fix the cause, then Request indexing", in.Link, true}, true
}

// printTodo prints the summary and returns the problems' links.
func printTodo(total, indexed int, findings []finding) (problemLinks []string) {
	fmt.Printf("✓ %d of %d sitemap URLs indexed as their own page\n", indexed, total)
	for _, f := range findings {
		if !f.Problem {
			fmt.Printf("· %s: %s: %s\n", f.URL, f.State, f.Note)
		}
	}
	for _, f := range findings {
		if f.Problem {
			link := f.Link
			if link == "" {
				link = "(no link from Google: Search Console → URL inspection → paste the URL)"
			} else {
				problemLinks = append(problemLinks, link)
			}
			fmt.Printf("✗ %s: %s\n    %s\n    %s\n", f.URL, f.State, f.Note, link)
		}
	}
	return problemLinks
}
