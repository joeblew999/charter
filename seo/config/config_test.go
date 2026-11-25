package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joeblew999/charter/seo/config"
)

// TestFind: [seo] of the charter.toml at or above the folder; its site is charter.toml's url unless it names another;
// without a file, every default.
func TestFind(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	os.MkdirAll(sub, 0o755)
	c, err := config.Find(sub)
	if err != nil || c.Path != "" || c.Site != "" || c.Lighthouse.Strategy != "mobile" || c.Lighthouse.Min != 90 {
		t.Fatalf("no file: %+v, %v", c, err)
	}
	os.WriteFile(filepath.Join(root, config.File), []byte("url = \"https://app.example.org\"\n[seo.lighthouse]\nmin = 80\n"), 0o644)
	c, err = config.Find(sub)
	if err != nil || c.Site != "https://app.example.org" || c.Lighthouse.Min != 80 || c.Badges.Dir != "docs/badges" {
		t.Fatalf("url: %+v, %v", c, err)
	}
	os.WriteFile(filepath.Join(root, config.File), []byte("url = \"https://app.example.org\"\n[seo]\nsite = \"https://other.example.org\"\n"), 0o644)
	if c, err = config.Find(sub); err != nil || c.Site != "https://other.example.org" {
		t.Fatalf("site: %+v, %v", c, err)
	}
}
