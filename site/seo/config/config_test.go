package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/site/seo/config"
)

// TestFind: the nearest seo.toml above the working directory, with what it doesn't say at its default; without one,
// every default, and no site: never one from the environment.
func TestFind(t *testing.T) {
	t.Setenv("APP_DOMAIN", "app.example.com")
	root := t.TempDir()
	c, err := config.Find(root)
	if err != nil || c.Path != "" || c.Site != "" || c.Lighthouse.Strategy != "mobile" || c.Lighthouse.Min != 90 {
		t.Fatalf("no seo.toml: %+v, %v", c, err)
	}
	os.WriteFile(filepath.Join(root, config.File), []byte(`site = "https://other.example.org"
[lighthouse]
pages = ["/", "/about"]
min = 80
`), 0o644)
	sub := filepath.Join(root, "app", "views")
	os.MkdirAll(sub, 0o755)
	if c, err = config.Find(sub); err != nil || c.Site != "https://other.example.org" || !slices.Equal(c.Lighthouse.Pages, []string{"/", "/about"}) ||
		c.Lighthouse.Min != 80 || c.Lighthouse.Strategy != "mobile" {
		t.Fatalf("seo.toml: %+v, %v", c, err)
	}
	// The template parses, and says nothing but the defaults.
	os.WriteFile(filepath.Join(root, config.File), []byte(config.Template), 0o644)
	if c, err = config.Find(root); err != nil || c.Site != "" || len(c.Lighthouse.Pages) != 0 {
		t.Fatalf("template: %+v, %v", c, err)
	}
	// init --site writes the site in, once: a site of the file's own stays.
	b, ok := config.SetSite(config.Template, "https://shop.example.com/")
	os.WriteFile(filepath.Join(root, config.File), []byte(b), 0o644)
	if c, err = config.Find(root); !ok || err != nil || c.Site != "https://shop.example.com" {
		t.Fatalf("SetSite: %v %+v, %v", ok, c, err)
	}
	if again, ok := config.SetSite(b, "https://other.example.com"); ok || again != b {
		t.Fatalf("SetSite on a file with a site of its own changed it:\n%s", again)
	}
	if b, ok := config.SetSite("[lighthouse]\nmin = 80\n", "https://shop.example.com"); !ok || !strings.HasPrefix(b, `site = "https://shop.example.com"`) {
		t.Fatalf("SetSite on a file with no site line:\n%s", b)
	}
}
