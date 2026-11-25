// Package badge draws a small two-part SVG badge, label and value, as README badges look (shields.io's flat
// style), with no service and no fonts to fetch: seo's Lighthouse and audit badges, and intl's languages badge. A
// module of its own, so neither of them depends on the other.
package badge

import (
	"fmt"
	"html"
	"strings"
)

// Colors for a score out of 100, as Lighthouse colors it: 90 and up good, 50 and up needs work, below poor.
const (
	Good = "#0cce6b"
	Fair = "#ffa400"
	Poor = "#ff4e42"
	Gray = "#555"
)

// ForScore is the color of a score out of 100.
func ForScore(score int) string {
	switch {
	case score >= 90:
		return Good
	case score >= 50:
		return Fair
	}
	return Poor
}

// SVG is a badge: label on gray, value on color.
func SVG(label, value, color string) string {
	lw, vw := width(label)+10, width(value)+10
	w := lw + vw
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">`+
		`<title>%s: %s</title>`+
		`<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>`+
		`<clipPath id="r"><rect width="%d" height="20" rx="3" fill="#fff"/></clipPath>`+
		`<g clip-path="url(#r)"><rect width="%d" height="20" fill="%s"/><rect x="%d" width="%d" height="20" fill="%s"/><rect width="%d" height="20" fill="url(#s)"/></g>`+
		`<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">`+
		`<text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text><text x="%d" y="14">%s</text>`+
		`<text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text><text x="%d" y="14">%s</text></g></svg>`+"\n",
		w, esc(label), esc(value), esc(label), esc(value),
		w, lw, Gray, lw, vw, color, w,
		lw/2, esc(label), lw/2, esc(label),
		lw+vw/2, esc(value), lw+vw/2, esc(value))
}

func esc(s string) string { return html.EscapeString(s) }

// width is s's width in Verdana 11px, near enough to size a badge: narrow letters, wide ones, the rest.
func width(s string) int {
	w := 0.0
	for _, r := range s {
		switch {
		case strings.ContainsRune("iljI.,:;!|' ", r):
			w += 3.5
		case strings.ContainsRune("mwMW@%", r):
			w += 10.5
		case r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			w += 7.5
		default:
			w += 6.6
		}
	}
	return int(w + 0.5)
}
