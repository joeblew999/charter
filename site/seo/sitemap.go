package seo

import (
	"html"
	"strings"
)

// Sitemap writes the sitemap of a site in one language: each of paths ("/", "/about") at origin
// ("https://app.example.com"), as Site's Sitemap serves it. A site in several languages has each page in every one,
// with its other-language versions: with intl, intl.Sitemap writes that.
func Sitemap(origin string, paths []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range paths {
		b.WriteString("<url><loc>" + html.EscapeString(origin+p) + "</loc></url>\n")
	}
	b.WriteString("</urlset>\n")
	return b.String()
}
