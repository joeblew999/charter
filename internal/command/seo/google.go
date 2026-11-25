package seo

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/seokit/pagespeed"
	"github.com/joeblew999/charter/internal/seokit/searchconsole"
	"github.com/joeblew999/charter/seo/config"
)

// The keys the commands that ask Google read: a service account's JSON key for Search Console (mise-seo setup makes
// it), and a PageSpeed Insights API key for Lighthouse, which falls back on the service account's.
const (
	consoleKey   = "GOOGLE_SEARCH_CONSOLE_KEY"
	pagespeedKey = "PAGESPEED_API_KEY"
)

// secret is the key name: from the environment when it's set there (set to "": none, and fnox isn't asked), else from
// fnox (fnox get), which every task that needs a key brings (its tools); "" for none. It's never printed.
func secret(name string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	if _, err := exec.LookPath("fnox"); err != nil {
		return ""
	}
	out, err := exec.Command("fnox", "get", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// noConsoleKey is what a command that asks Search Console says without the key.
var noConsoleKey = errors.New(consoleKey + " is neither in the environment nor in fnox: mise run seo:setup makes a service account and stores its key in fnox (once per person)")

// console is a Search Console client for the property of seo.toml's site, with the service account key's token, and
// the key; without the key it fails, saying how to make one.
func console(ctx context.Context, c config.Config, site string) (*searchconsole.Client, searchconsole.Key) {
	raw := secret(consoleKey)
	if raw == "" {
		fail(noConsoleKey)
	}
	key, err := searchconsole.ParseKey([]byte(raw))
	must(err)
	httpc := &http.Client{Timeout: time.Minute}
	token, err := key.Token(ctx, httpc, searchconsole.Scope)
	must(err)
	property := cmp.Or(c.Property, domainProperty(host(site)))
	fmt.Fprintf(os.Stderr, "mise-seo: Search Console property %s, as %s\n", property, key.ClientEmail)
	return &searchconsole.Client{HTTP: httpc, Token: token, Site: property}, key
}

// access reports whether the service account has Full access (or is an owner) to the client's property, among the
// properties it can use.
func access(sites []searchconsole.Site, property string) bool {
	for _, s := range sites {
		if s.SiteURL == property && (s.PermissionLevel == "siteOwner" || s.PermissionLevel == "siteFullUser") {
			return true
		}
	}
	return false
}

// noAccess is what a command says when the service account lacks Full access to the property.
func noAccess(key searchconsole.Key, property string) error {
	return fmt.Errorf("%s has no Full access to %s: Search Console → the property → Settings → Users and permissions → Add user (Full), then mise run seo:sites", key.ClientEmail, property)
}

// consoleWithAccess is console, failing unless the account has Full access to the property: every command that asks
// Search Console about the site needs it.
func consoleWithAccess(ctx context.Context, c config.Config, site string) *searchconsole.Client {
	sc, key := console(ctx, c, site)
	sites, err := sc.Sites(ctx)
	must(err)
	if !access(sites, sc.Site) {
		fail(noAccess(key, sc.Site))
	}
	return sc
}

// pagespeedAuth is how Lighthouse runs: with PAGESPEED_API_KEY when there is one (a free key is all it needs), else
// with the Search Console service account's token.
func pagespeedAuth(ctx context.Context) pagespeed.Auth {
	if k := secret(pagespeedKey); k != "" {
		return pagespeed.Auth{Key: k}
	}
	raw := secret(consoleKey)
	if raw == "" {
		fail(fmt.Errorf("Lighthouse needs %s (a free key: Google Cloud console → APIs & Services → Credentials → API key, "+
			"restricted to the PageSpeed Insights API; then fnox set %s) or %s (mise run seo:setup), in the environment or in fnox",
			pagespeedKey, pagespeedKey, consoleKey))
	}
	key, err := searchconsole.ParseKey([]byte(raw))
	must(err)
	token, err := key.Token(ctx, &http.Client{Timeout: time.Minute}, pagespeed.Scope)
	must(err)
	return pagespeed.Auth{Token: token}
}

// domainProperty is the Domain property of a host's registrable domain: app.example.com → sc-domain:example.com (its
// last two labels: good enough for .net and .com hosts; set property in seo.toml otherwise). An IP address or a
// single-label host (localhost) has none: "".
func domainProperty(host string) string {
	labels := strings.Split(host, ".")
	if len(labels) < 2 || net.ParseIP(host) != nil {
		return ""
	}
	return "sc-domain:" + strings.Join(labels[len(labels)-2:], ".")
}
