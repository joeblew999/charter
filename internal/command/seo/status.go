package seo

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/joeblew999/charter/internal/seokit/searchconsole"
	"github.com/joeblew999/charter/site/seo/audit"
)

// JSON of charter seo status --json.
type (
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
)

// status is charter seo status (seo:status): the submitted sitemap's status in Search Console, and Google's index
// status of every URL the live sitemap lists (its coverage, last crawl and canonical), printed as each is in.
func (r repo) status(args []string) {
	f := flags("status")
	jsonOut := f.Bool("json", false, "one JSON document on stdout")
	parse(f, args, 0)
	c := r.conf()
	site := r.siteOf(c, "")
	ctx := context.Background()
	sc := consoleWithAccess(ctx, c, site)
	sitemap := site + "/sitemap.xml"
	rep := statusReport{Counts: map[string]int{}, URLs: []urlStatus{}}
	if s, err := sc.GetSitemap(ctx, sitemap); err != nil {
		rep.SitemapError = err.Error()
	} else {
		rep.Sitemap = &s
	}
	if !*jsonOut {
		if rep.Sitemap == nil {
			fmt.Printf("sitemap %s: %s\n", sitemap, rep.SitemapError)
		} else {
			s := rep.Sitemap
			fmt.Printf("sitemap %s: submitted %s, downloaded %s, pending %v, errors %s, warnings %s\n",
				s.Path, or(s.LastSubmitted, "-"), or(s.LastDownloaded, "never"), s.IsPending, or(s.Errors, "0"), or(s.Warnings, "0"))
			for _, ct := range s.Contents {
				fmt.Printf("  %s: %s submitted, %s indexed\n", ct.Type, ct.Submitted, or(ct.Indexed, "?"))
			}
		}
	}
	urls, err := audit.SitemapURLs(ctx, &http.Client{Timeout: time.Minute}, sitemap)
	must(err)
	// The URL Inspection API takes seconds a URL: a few at once (its quota: 600 a minute, 2,000 a day) and, as text,
	// each row printed as soon as it and the rows before it are in.
	rows := make([]chan urlStatus, len(urls))
	sem := make(chan struct{}, 6)
	for i, u := range urls {
		rows[i] = make(chan urlStatus, 1)
		go func(u string, out chan<- urlStatus) {
			sem <- struct{}{}
			defer func() { <-sem }()
			in, err := sc.Inspect(ctx, u, "en")
			if err != nil {
				out <- urlStatus{URL: u, Error: err.Error()}
				return
			}
			out <- urlStatus{URL: u, Verdict: in.Verdict, Coverage: in.CoverageState, LastCrawl: in.LastCrawlTime,
				GoogleCanonical: in.GoogleCanonical, UserCanonical: in.UserCanonical, Link: in.Link}
		}(u, rows[i])
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.FilterHTML)
	if !*jsonOut {
		fmt.Fprintln(w, "URL\tVERDICT\tCOVERAGE\tLAST CRAWL\tCANONICAL")
	}
	problems := 0
	for _, ch := range rows {
		u := <-ch
		rep.URLs = append(rep.URLs, u)
		state := u.Coverage
		if u.Error != "" {
			state = "error"
		}
		rep.Counts[state]++
		canonical := "ok"
		if u.Error != "" || u.GoogleCanonical != "" && u.GoogleCanonical != u.URL {
			problems++
			canonical = "Google chose " + u.GoogleCanonical
		}
		if !*jsonOut {
			if u.Error != "" {
				fmt.Fprintf(w, "%s\tERROR\t%s\t\t\n", u.URL, u.Error)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", u.URL, u.Verdict, u.Coverage, or(u.LastCrawl, "-"), canonical)
			}
			w.Flush()
		}
	}
	if *jsonOut {
		emit(rep)
		return
	}
	for state, n := range rep.Counts {
		fmt.Printf("  %3d  %s\n", n, state)
	}
	fmt.Printf("%d URLs inspected (Google's index, not a live fetch); %d with errors or a different canonical\n", len(urls), problems)
}
