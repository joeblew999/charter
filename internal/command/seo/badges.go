package seo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/badge"
	"github.com/joeblew999/charter/internal/seokit/pagespeed"
	"github.com/joeblew999/charter/site/seo/audit"
)

// The README lines between these two are charter seo badges' to write.
const scoresStart, scoresEnd = "<!-- seo scores -->", "<!-- /seo scores -->"

// categories are Lighthouse's, in its order, with the names the badges and the table use.
var categories = []struct{ id, name, title string }{
	{"performance", "performance", "Performance"}, {"accessibility", "accessibility", "Accessibility"},
	{"best-practices", "best practices", "Best practices"}, {"seo", "seo", "SEO"},
}

// badges is charter seo badges: Lighthouse on every page and the audit, then an SVG badge per Lighthouse
// category into charter.toml's [seo.badges] dir (the lowest page's score: a badge never flatters) and one for the audit, and
// the README's scores block (charter.toml's [seo.badges] readme) rewritten with every page's scores. Run it after a production
// release, and commit what it wrote.
func (r repo) badges(args []string) {
	parse(flags("badges"), args, 0)
	c := r.conf()
	ctx := context.Background()
	site := r.siteOf(c, "")
	auth := pagespeedAuth(ctx)
	pages := r.pages(ctx, c, site, nil)
	reports, errs := lighthouseAll(ctx, auth, pages, c.Lighthouse.Strategy)
	for i, err := range errs {
		if err != nil {
			fail(fmt.Errorf("%s: %w", pages[i], err))
		}
	}
	dir := filepath.Join(r.dir, filepath.FromSlash(c.Badges.Dir))
	for _, cat := range categories {
		low := 100
		for _, rep := range reports {
			low = min(low, score(rep, cat.id))
		}
		writeFile(filepath.Join(dir, "lighthouse-"+strings.ReplaceAll(cat.name, " ", "-")+".svg"),
			[]byte(badge.SVG(cat.name, fmt.Sprint(low), badge.ForScore(low))))
	}
	_, problems, err := audit.Audit(ctx, &http.Client{Timeout: time.Minute}, site)
	must(err)
	value, color := "passing", badge.Good
	if len(problems) > 0 {
		value, color = fmt.Sprintf("%d problems", len(problems)), badge.Poor
	}
	writeFile(filepath.Join(dir, "seo-audit.svg"), []byte(badge.SVG("seo audit", value, color)))
	stampScores(filepath.Join(r.dir, filepath.FromSlash(c.Badges.Readme)), reports, c.Lighthouse.Strategy)
}

// score is a report's score for category id, 0 without one.
func score(r pagespeed.Report, id string) int {
	for _, c := range r.Categories {
		if c.ID == id {
			return c.Score
		}
	}
	return 0
}

// stampScores rewrites what is between readme's scores markers: a table of every page's scores, and when and how
// they were measured. Without the markers it says where to put them.
func stampScores(readme string, reports []pagespeed.Report, strategy string) {
	b, err := os.ReadFile(readme)
	if err != nil {
		fail(err)
	}
	s := string(b)
	i, j := strings.Index(s, scoresStart), strings.Index(s, scoresEnd)
	if i < 0 || j < i {
		fmt.Printf("· %s has no %s … %s lines: put them where the scores table belongs\n", readme, scoresStart, scoresEnd)
		return
	}
	var t strings.Builder
	t.WriteString(scoresStart + "\n\n| Page |")
	for _, c := range categories {
		t.WriteString(" " + c.title + " |")
	}
	t.WriteString("\n|---|" + strings.Repeat("---:|", len(categories)) + "\n")
	for _, r := range reports {
		path := r.URL
		if u, err := url.Parse(r.URL); err == nil {
			path = u.Path
		}
		fmt.Fprintf(&t, "| [%s](%s) |", path, r.URL)
		for _, c := range categories {
			fmt.Fprintf(&t, " %d |", score(r, c.id))
		}
		t.WriteString("\n")
	}
	fmt.Fprintf(&t, "\nLighthouse %s on Google's servers (PageSpeed Insights), %s: `charter seo badges` writes this table and the badges.\n\n",
		strategy, time.Now().UTC().Format("2006-01-02"))
	writeFile(readme, []byte(s[:i]+t.String()+s[j:]))
}
