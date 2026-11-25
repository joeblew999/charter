package seo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeblew999/charter/internal/seokit/pagespeed"
)

// TestStampScores: only what is between the markers changes, into a table of every page's scores; without the
// markers the README is left alone.
func TestStampScores(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	os.WriteFile(path, []byte("# App\n\n"+scoresStart+"\nold\n"+scoresEnd+"\n\nThe rest.\n"), 0o644)
	r := pagespeed.Report{URL: "https://app.example.com/de/board", Categories: []pagespeed.Category{
		{ID: "performance", Score: 99}, {ID: "accessibility", Score: 84}, {ID: "best-practices", Score: 100}, {ID: "seo", Score: 100}}}
	stampScores(path, []pagespeed.Report{r}, "mobile")
	b, _ := os.ReadFile(path)
	s := string(b)
	for _, want := range []string{"# App\n\n" + scoresStart + "\n", "| Page | Performance | Accessibility | Best practices | SEO |",
		"| [/de/board](https://app.example.com/de/board) | 99 | 84 | 100 | 100 |", scoresEnd + "\n\nThe rest.\n"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "old") {
		t.Errorf("the old block is still there:\n%s", s)
	}
	plain := filepath.Join(t.TempDir(), "README.md")
	os.WriteFile(plain, []byte("# App\n"), 0o644)
	stampScores(plain, []pagespeed.Report{r}, "mobile")
	if b, _ := os.ReadFile(plain); string(b) != "# App\n" {
		t.Errorf("a README without markers changed: %q", b)
	}
}
