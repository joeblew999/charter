package catalog

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joeblew999/charter/internal/badge"
)

// The languages badge ([intl] badge) for a README: how many languages the app ships and whether every one is
// translated, drawn like seo's Lighthouse badges. gen writes it from the catalogs, so it can't go stale; check says
// when the file isn't what gen would write.

// BadgeFile is [intl] badge, "" without one.
func (p *Project) BadgeFile() string {
	if p.Config.Badge == "" {
		return ""
	}
	return filepath.Join(p.Config.Root, p.Config.Badge)
}

func (p *Project) badgeBytes() []byte {
	behind := 0
	for _, l := range p.Locales {
		if !l.Current() {
			behind++
		}
	}
	value, color := fmt.Sprintf("%d, all translated", len(p.Locales)), badge.Good
	if behind > 0 {
		value, color = fmt.Sprintf("%d, %d not translated", len(p.Locales), behind), badge.Fair
	}
	return []byte(badge.SVG("languages", value, color))
}

// WriteBadge writes the badge, when charter.toml's [intl] names one.
func (p *Project) WriteBadge() error {
	f := p.BadgeFile()
	if f == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	return writeFile(f, p.badgeBytes())
}

// BadgeCurrent reports whether the badge on disk is what WriteBadge would write (true without one).
func (p *Project) BadgeCurrent() bool {
	f := p.BadgeFile()
	if f == "" {
		return true
	}
	b, err := os.ReadFile(f)
	return err == nil && string(b) == string(p.badgeBytes())
}
