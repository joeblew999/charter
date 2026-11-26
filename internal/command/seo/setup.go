package seo

import (
	"cmp"
	"context"
	"fmt"
	"github.com/joeblew999/charter/internal/tools"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/seokit/searchconsole"
	"github.com/joeblew999/charter/seo/config"
)

// setup is charter run seo:search setup, once per person, not per site: one service account serves every site it is
// added to, and Lighthouse when there's no PAGESPEED_API_KEY. With the person's gcloud login (gcloud auth login), it
// makes or checks the Google Cloud project (SEO_PROJECT, else the key's own, else <the site's first label>-search),
// enables the Search Console and PageSpeed Insights APIs there, and makes the service account and stores its key in
// fnox (GOOGLE_SEARCH_CONSOLE_KEY, in the keychain) unless one is there. Then it says what's left: the account's Full
// access to the property, which only Search Console gives. The task brings gcloud, the Python its installer needs, and
// fnox, and asks first: it writes to Google.
func (r repo) setup(args []string) {
	parse(flags("setup"), args, 0)
	c := r.conf()
	site := r.siteOf(c, "")
	// gcloud's update checks and usage reporting stall every command for minutes on some machines.
	for _, s := range []string{"component_manager/disable_update_check", "core/disable_usage_reporting", "survey/disable_prompts"} {
		gcloudOut("config", "set", s, "true", "--quiet")
	}
	account := strings.TrimSpace(gcloudOut("auth", "list", "--filter=status:ACTIVE", "--format=value(account)"))
	if account == "" {
		fail(fmt.Errorf("no gcloud login: run gcloud auth login first"))
	}
	raw := secret(consoleKey)
	var sa, project string
	if raw != "" {
		key, err := searchconsole.ParseKey([]byte(raw))
		must(err)
		sa = key.ClientEmail
		project = cmp.Or(os.Getenv("SEO_PROJECT"), keyProject(sa))
	} else {
		project = cmp.Or(os.Getenv("SEO_PROJECT"), defaultProject(site))
		sa = "search-console@" + project + ".iam.gserviceaccount.com"
	}
	fmt.Printf("· gcloud account %s, project %s\n", account, project)
	if _, err := gcloud("projects", "describe", project, "--quiet"); err != nil {
		mustGcloud("projects", "create", project, "--name="+project, "--quiet")
	}
	mustGcloud("services", "enable", "searchconsole.googleapis.com", "pagespeedonline.googleapis.com", "--project", project, "--quiet")
	fmt.Printf("✓ project %s: the Search Console and PageSpeed Insights APIs\n", project)
	if raw != "" {
		fmt.Printf("✓ %s is there: service account %s\n", consoleKey, sa)
	} else {
		if _, err := gcloud("iam", "service-accounts", "describe", sa, "--project", project, "--quiet"); err != nil {
			mustGcloud("iam", "service-accounts", "create", "search-console", "--project", project, "--display-name=Search Console", "--quiet")
		}
		dir, err := os.MkdirTemp("", "charter run seo:search-key")
		must(err)
		defer os.RemoveAll(dir)
		file := filepath.Join(dir, "key.json")
		mustGcloud("iam", "service-accounts", "keys", "create", file, "--iam-account", sa, "--project", project, "--quiet")
		fnox, err := tools.Fnox()
		must(err)
		store := exec.Command(fnox, "set", "-g", consoleKey, "--from-file", file, "-p", "keychain", "-d", "Search Console API key ("+sa+")")
		store.Stderr = os.Stderr
		if err := store.Run(); err != nil {
			os.RemoveAll(dir)
			fail(fmt.Errorf("fnox set %s: %w", consoleKey, err))
		}
		fmt.Printf("✓ service account %s, its key in fnox as %s\n", sa, consoleKey)
	}
	if hasAccess(c, site) {
		fmt.Printf("✓ %s has Full access to the property\n", sa)
		return
	}
	fmt.Printf("Next: Search Console → the property → Settings → Users and permissions → Add user %s (Full), then charter run seo:search sites\n", sa)
}

// hasAccess reports whether the service account in fnox has Full access to the property of the site, asking quietly:
// no key, or no answer, is no access.
func hasAccess(c config.Config, site string) bool {
	key, err := searchconsole.ParseKey([]byte(secret(consoleKey)))
	if err != nil {
		return false
	}
	ctx := context.Background()
	httpc := &http.Client{Timeout: time.Minute}
	token, err := key.Token(ctx, httpc, searchconsole.Scope)
	if err != nil {
		return false
	}
	sc := &searchconsole.Client{HTTP: httpc, Token: token, Site: cmp.Or(c.Property, domainProperty(host(site)))}
	sites, err := sc.Sites(ctx)
	return err == nil && access(sites, sc.Site)
}

// keyProject is the Google Cloud project of a service account's address: …@<project>.iam.gserviceaccount.com.
func keyProject(email string) string {
	m := regexp.MustCompile(`^[^@]*@([^.]+)\.iam\.gserviceaccount\.com$`).FindStringSubmatch(email)
	if m == nil {
		return ""
	}
	return m[1]
}

// defaultProject is the project setup makes without SEO_PROJECT or a key: the site's first label, then -search
// (go-htmx4.example.net → go-htmx4-search), as Google Cloud's project ids are written: lower case, digits and dashes,
// starting with a letter.
func defaultProject(site string) string {
	label, _, _ := strings.Cut(host(site), ".")
	label = strings.Trim(regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(strings.ToLower(label), "-"), "-")
	if label == "" || label[0] < 'a' || label[0] > 'z' {
		label = "site-" + label
	}
	return strings.TrimRight(label[:min(len(label), 23)], "-") + "-search"
}

// gcloud runs gcloud with args, its output returned and its errors shown.
func gcloud(args ...string) (string, error) {
	cmd := exec.Command("gcloud", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

// gcloudOut is gcloud's output, "" when it fails.
func gcloudOut(args ...string) string {
	out, _ := exec.Command("gcloud", args...).Output()
	return string(out)
}

// mustGcloud runs gcloud with args, its output shown, and fails when it does.
func mustGcloud(args ...string) {
	cmd := exec.Command("gcloud", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("gcloud %s: %w", strings.Join(args, " "), err))
	}
}
