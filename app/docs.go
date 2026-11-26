package app

import "strings"

// Doc is a page of an app's docs: a Markdown file of its docs/ folder, which charter build turns into HTML once, so
// that no Markdown is read on a Worker (docs/docs.built.go has them all, in the files' order). An app shows them with
// pages of its own, in its own components; charter gives it the content, the links and the list.
type Doc struct {
	// Slug is the page's place under /docs: "" for docs/index.md, "pages" for docs/03-pages.md.
	Slug string
	// Title is the page's first heading, and Description its first paragraph, as a search result shows them.
	Title, Description string
	// HTML is the page: trusted, written by charter build from the app's own Markdown.
	HTML string
	// Headings are the page's sections, for a table of contents.
	Headings []DocHeading
	// Source is the file, from the app's folder: docs/03-pages.md.
	Source string
}

// DocHeading is a section of a Doc: the id its heading has in the HTML, and how deep it is (2 or 3).
type DocHeading struct {
	ID, Title string
	Depth     int
}

// Path is the page's address, without a language: /docs/ or /docs/pages.
func (d Doc) Path() string { return "/docs/" + d.Slug }

// FindDoc is the page with that slug.
func FindDoc(docs []Doc, slug string) (Doc, bool) {
	for _, d := range docs {
		if d.Slug == strings.Trim(slug, "/") {
			return d, true
		}
	}
	return Doc{}, false
}

// DocPaths are the pages' addresses, for a page type's Paths method: what makes each a page search engines may
// index, in the sitemap.
func DocPaths(docs []Doc) []string {
	paths := make([]string, 0, len(docs))
	for _, d := range docs {
		if d.Slug != "" {
			paths = append(paths, d.Path())
		}
	}
	return paths
}
