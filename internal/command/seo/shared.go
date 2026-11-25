package seo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joeblew999/charter/seo/config"
)

// conf is the repo's seo.toml, found up from its folder, its defaults filled in; every default without one.
func (r repo) conf() config.Config {
	c, err := config.Find(r.dir)
	if err != nil {
		fail(fmt.Errorf("%s: %w", config.File, err))
	}
	return c
}

// siteOf is the site a command works on, without its trailing /: flagSite when it's given (--site), else seo.toml's.
// It says which, and where that came from, on stderr before anything else: a run on the wrong site must be obvious.
// None fails, saying how to set it.
func (r repo) siteOf(c config.Config, flagSite string) string {
	site, from := c.Site, c.Path
	if flagSite != "" {
		site, from = flagSite, "--site"
	}
	if site == "" {
		fail(fmt.Errorf("no site: set site in %s (charter seo init --site https://your.site), or pass --site", config.File))
	}
	site = strings.TrimRight(site, "/")
	fmt.Fprintf(os.Stderr, "charter seo: %s (from %s)\n", site, from)
	return site
}

// absolute is page (a path, /de/, or a full URL) as a full URL on site.
func absolute(site, page string) string {
	if strings.HasPrefix(page, "/") {
		return site + page
	}
	return page
}

// host is a URL's host name, "" for none.
func host(site string) string {
	u, err := url.Parse(site)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// emit writes v as indented JSON on stdout.
func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	must(enc.Encode(v))
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// openInBrowser opens links on macOS (open), Linux (xdg-open) and Windows (rundll32's URL handler).
func openInBrowser(links []string) {
	for _, l := range links {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", l)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", l)
		default:
			cmd = exec.Command("xdg-open", l)
		}
		_ = cmd.Run()
	}
}

// exists reports whether the repo has the file p (slash-separated).
func (r repo) exists(p string) bool {
	_, err := os.Stat(filepath.Join(r.dir, filepath.FromSlash(p)))
	return err == nil
}

func must(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "charter seo:", err)
	os.Exit(1)
}

// writeFile writes b to path unless it is that already, and says which.
func writeFile(path string, b []byte) {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		fmt.Printf("✓ %s is up to date\n", path)
		return
	}
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	must(os.WriteFile(path, b, 0o644))
	fmt.Println("wrote", path)
}
