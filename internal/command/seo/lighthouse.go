package seo

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joeblew999/charter/internal/seokit/pagespeed"
	"github.com/joeblew999/charter/site/seo/audit"
	"github.com/joeblew999/charter/site/seo/config"
)

// lighthouse is charter seo lighthouse (seo:lighthouse): Lighthouse on the live site through PageSpeed Insights, on
// Google's servers, so no Node and no local Chrome: every page of the sitemap (one per page, whatever its languages),
// [seo] pages, or the one named. It prints each page's four scores, what fails and on which elements, and real
// users' Core Web Vitals, and exits 1 when a category of any page scores under [seo] min. Mobile, what Google
// indexes, or --desktop.
func (r repo) lighthouse(args []string) {
	f := flags("lighthouse")
	jsonOut := f.Bool("json", false, "one JSON document on stdout")
	desktop := f.Bool("desktop", false, "score the desktop page instead of the mobile one Google indexes")
	pages := parse(f, args, 1)
	c := r.conf()
	strategy := c.Lighthouse.Strategy
	if *desktop {
		strategy = "desktop"
	}
	ctx := context.Background()
	site := r.siteOf(c, "")
	auth := pagespeedAuth(ctx)
	reports, errs := lighthouseAll(ctx, auth, r.pages(ctx, c, site, pages), strategy)
	low := false
	for i, rep := range reports {
		if errs[i] != nil {
			low = true
			if !*jsonOut {
				fmt.Printf("✗ %s: %v\n", rep.URL, errs[i])
			}
			continue
		}
		for _, cat := range rep.Categories {
			low = low || cat.Score < c.Lighthouse.Min
		}
		if !*jsonOut {
			printLighthouse(rep, c.Lighthouse.Min)
		}
	}
	if *jsonOut {
		emit(reports)
	}
	if low {
		os.Exit(1)
	}
}

// pages are the pages Lighthouse scores, as full URLs: the one named, else charter.toml's [seo]'s, else one per page of the live
// sitemap.
func (r repo) pages(ctx context.Context, c config.Config, site string, named []string) []string {
	pages := c.Lighthouse.Pages
	if len(named) > 0 {
		pages = named
	}
	if len(pages) == 0 {
		var err error
		pages, err = audit.SitemapPages(ctx, &http.Client{Timeout: time.Minute}, site+"/sitemap.xml")
		must(err)
	}
	var out []string
	for _, p := range pages {
		out = append(out, absolute(site, p))
	}
	return out
}

// lighthouseAll runs Lighthouse on every page, four at a time; a page's report has its URL even when its run failed.
func lighthouseAll(ctx context.Context, auth pagespeed.Auth, pages []string, strategy string) ([]pagespeed.Report, []error) {
	httpc := &http.Client{Timeout: 3 * time.Minute}
	reports, errs := make([]pagespeed.Report, len(pages)), make([]error, len(pages))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, page := range pages {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			reports[i], errs[i] = pagespeed.Run(ctx, httpc, auth, page, strategy)
			reports[i].URL = page
		})
	}
	wg.Wait()
	return reports, errs
}

func printLighthouse(r pagespeed.Report, min int) {
	fmt.Printf("%s (%s, Lighthouse on Google's servers)\n", r.URL, r.Strategy)
	for _, c := range r.Categories {
		mark := "✓"
		if c.Score < min {
			mark = "✗"
		}
		fmt.Printf("%s %-15s %3d\n", mark, c.Title, c.Score)
		for _, a := range c.Failed {
			fmt.Printf("      %3d  %s  %s  (%s)\n", a.Score, a.Title, a.Display, a.ID)
			for _, it := range a.Items {
				fmt.Printf("             %.160s\n", it)
			}
		}
	}
	if len(r.Field) == 0 {
		fmt.Println("real users: too few visits for Chrome's field data yet")
	}
	for _, m := range r.Field {
		fmt.Printf("real users: %-40s p75 %6d  %s\n", m.ID, m.Percentile, m.Category)
	}
	fmt.Println()
}
