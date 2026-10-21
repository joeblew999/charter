package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/joeblew999/charter/go/lib/searchconsole"
)

// finding is a sitemap URL Google hasn't indexed as its own page. Only problems need a person.
type finding struct {
	URL     string `json:"url"`
	State   string `json:"state"`
	Note    string `json:"note"`
	Link    string `json:"link,omitempty"`
	Problem bool   `json:"problem"`
}

// todo inspects every sitemap URL in Google's index. It returns the number indexed and the rest as findings.
func todo(ctx context.Context, c *searchconsole.Client, urls []string) (indexed int, findings []finding) {
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

// openInBrowser opens links on macOS (open) or Linux (xdg-open); elsewhere the printed links are enough.
func openInBrowser(links []string) {
	cmd := map[string]string{"darwin": "open", "linux": "xdg-open"}[runtime.GOOS]
	if cmd == "" {
		return
	}
	for _, l := range links {
		_ = exec.Command(cmd, l).Run()
	}
}
