package seo

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joeblew999/charter/seo/audit"
)

// auditReport is charter run seo:audit --json. Its field names are the stable interface; the text renders the same values.
type auditReport struct {
	Site     string   `json:"site"`
	URLs     int      `json:"urls"`
	OK       bool     `json:"ok"`
	Problems []string `json:"problems"`
}

// audit is charter run seo:audit: robots.txt, the sitemap and every URL in it fetched live as Googlebot, and
// every problem that would keep a page out of Google's index or look wrong in a result, one a line (audit.Audit); exit
// 1 on any. [seo] site, or --site's: a local build, a preview. It needs no Google account.
func (r repo) audit(args []string) {
	f := flags("audit")
	jsonOut := f.Bool("json", false, "one JSON document on stdout")
	site := f.String("site", "", "another server than [seo] site: a local build, a preview")
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
			"http moves to https, the home page names the site; and every link they have to this site leads somewhere)\n", rep.URLs)
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

// Audit is the audit of a server, for a command that has more to do after it (charter test, which still has a local
// Worker to stop): every problem said, and an error when there is one.
func Audit(base string) error {
	urls, problems, err := audit.Audit(context.Background(), &http.Client{Timeout: time.Minute}, base)
	if err != nil {
		return err
	}
	if len(problems) == 0 {
		fmt.Printf("✓ robots.txt and all %d sitemap URLs pass (fetched as Googlebot), and every link they have to this site leads somewhere\n", urls)
		return nil
	}
	for _, p := range problems {
		fmt.Println("✗ " + p)
	}
	return fmt.Errorf("the search-engine audit found %d problem(s) across robots.txt and %d sitemap URLs", len(problems), urls)
}
