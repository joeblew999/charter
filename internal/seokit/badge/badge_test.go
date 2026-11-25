package badge_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/joeblew999/charter/internal/seokit/badge"
)

// TestSVG: a well-formed SVG that names itself for screen readers, wider for a longer label, escaped.
func TestSVG(t *testing.T) {
	short, long := badge.SVG("seo", "100", badge.Good), badge.SVG("best practices & more", "96", badge.ForScore(96))
	for _, s := range []string{short, long} {
		if err := xml.Unmarshal([]byte(s), new(struct{})); err != nil {
			t.Fatalf("not XML: %v\n%s", err, s)
		}
	}
	if !strings.Contains(short, `aria-label="seo: 100"`) || !strings.Contains(long, "best practices &amp; more") {
		t.Errorf("label: %s", long)
	}
	if !(len(long) > len(short)) || !strings.Contains(short, badge.Good) {
		t.Errorf("size or color: %s", short)
	}
	for score, want := range map[int]string{100: badge.Good, 90: badge.Good, 89: badge.Fair, 50: badge.Fair, 49: badge.Poor} {
		if got := badge.ForScore(score); got != want {
			t.Errorf("ForScore(%d) = %s, want %s", score, got, want)
		}
	}
}
