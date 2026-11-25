package seo

import (
	"context"
	"fmt"
	"os"

	"github.com/joeblew999/charter/internal/seokit/searchconsole"
)

// sitesReport is charter run search sites --json.
type sitesReport struct {
	Property string               `json:"property"`
	Access   bool                 `json:"access"` // Full or Owner on Property
	Sites    []searchconsole.Site `json:"sites"`
}

// sites is charter run search sites: the Search Console properties the service account can use, and whether it
// has Full access to [seo] property (its property, else its site's Domain property); exit 1 without it, saying
// how to add the account there.
func (r repo) sites(args []string) {
	f := flags("sites")
	jsonOut := f.Bool("json", false, "one JSON document on stdout")
	parse(f, args, 0)
	c := r.conf()
	ctx := context.Background()
	sc, key := console(ctx, c, r.siteOf(c, ""))
	list, err := sc.Sites(ctx)
	must(err)
	rep := sitesReport{Property: sc.Site, Access: access(list, sc.Site), Sites: append([]searchconsole.Site{}, list...)}
	if *jsonOut {
		emit(rep)
	} else {
		for _, s := range rep.Sites {
			fmt.Printf("%s\t%s\n", s.SiteURL, s.PermissionLevel)
		}
		if rep.Access {
			fmt.Printf("✓ %s has Full access to %s\n", key.ClientEmail, sc.Site)
		} else {
			fmt.Println("✗ " + noAccess(key, sc.Site).Error())
		}
	}
	if !rep.Access {
		os.Exit(1)
	}
}
