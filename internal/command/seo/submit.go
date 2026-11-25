package seo

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/joeblew999/charter/seo/audit"
)

// submit is charter run search submit: the site's sitemap.xml submitted (or submitted again) to Search Console,
// once the account has Full access to the property and the live audit passes: Google crawls what it lists. The task
// asks first: it writes to Google.
func (r repo) submit(args []string) {
	parse(flags("submit"), args, 0)
	c := r.conf()
	site := r.siteOf(c, "")
	ctx := context.Background()
	sc := consoleWithAccess(ctx, c, site)
	if _, problems, err := audit.Audit(ctx, &http.Client{Timeout: time.Minute}, site); err != nil || len(problems) > 0 {
		fail(fmt.Errorf("the audit doesn't pass (%v, %d problems): charter run audit says what, and the sitemap goes to Google once it passes", err, len(problems)))
	}
	sitemap := site + "/sitemap.xml"
	must(sc.SubmitSitemap(ctx, sitemap))
	fmt.Printf("✓ the audit passes, and %s is submitted to %s\n", sitemap, sc.Site)
}
