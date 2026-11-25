package seo

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joeblew999/charter/site/seo/audit"
)

// auditReport is charter seo audit --json. Its field names are the stable interface; the text renders the same values.
type auditReport struct {
	Site     string   `json:"site"`
	URLs     int      `json:"urls"`
	OK       bool     `json:"ok"`
	Problems []string `json:"problems"`
}

// audit is charter seo audit (seo:audit): robots.txt, the sitemap and every URL in it fetched live as Googlebot, and
// every problem that would keep a page out of Google's index or look wrong in a result, one a line (audit.Audit); exit
// 1 on any. seo.toml's site, or --site's: a local build, a preview. It needs no Google account.
func (r repo) audit(args []string) {
	f := flags("audit")
	jsonOut := f.Bool("json", false, "one JSON document on stdout")
	site := f.String("site", "", "another server than seo.toml's site: a local build, a preview")
	parse(f, args, 0)
	base := r.siteOf(r.conf(), *site)
	urls, problems, err := audit.Audit(context.Background(), &http.Client{Timeout: time.Minute}, base)
	must(err)
	rep := auditReport{Site: base, URLs: urls, OK: len(problems) == 0, Problems: append([]string{}, problems...)}
	switch {
	case *jsonOut:
		emit(rep)
	case rep.OK:
		fmt.Printf("✓ robots.txt and all %d sitemap URLs pass (fetched as Googlebot: 200, indexable, self-canonical, lang, "+
			"title, description, one h1, reciprocal hreflang with x-default, icon, Open Graph; a missing page is 404, "+
			"http moves to https, the home page names the site)\n", rep.URLs)
	default:
		for _, p := range rep.Problems {
			fmt.Println("✗ " + p)
		}
		fmt.Printf("%d problem(s) across robots.txt and %d sitemap URLs (fetched as Googlebot)\n", len(rep.Problems), rep.URLs)
	}
	if !rep.OK {
		os.Exit(1)
	}
}
