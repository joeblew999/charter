package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/joeblew999/charter/seo"
)

// Docs are an app's docs: the Markdown files of its docs/ folder, which charter build turns into Go once
// (docs/docs.built.go), so that no Markdown code runs on a Worker. An app has them by naming them in its App and
// mounting the docs screen (charter add screen:docs) in its tree, at /docs:
//
//	docs screens.Docs `route:"/docs Docs"`
//
// From that one value come the pages, their place in the sitemap, each page as Markdown (/docs/<page>.md), and what
// a language model reads: /llms.txt and /llms-full.txt.
type Docs struct {
	// Name is what the docs are of, and Summary one line about it: the heading of the pages, and of /llms.txt.
	Name, Summary string
	// Pages are the docs, as charter build wrote them: index.md first.
	Pages []Doc
	// Repo is the repository on GitHub (https://github.com/owner/name), and Dir the app's folder in it ("" at its
	// top): where the pages link to, and where "edit this page" goes.
	Repo, Dir string
	// Agents is what /llms.txt says to a coding agent, in Markdown: which plugin or skill to work with. It may be
	// empty.
	Agents string
	// More are sections of /llms.txt after the docs: the app's own pages, its API.
	More []seo.LLMsSection
}

// Doc is a page of the docs.
type Doc struct {
	// Slug is the page's place under /docs: "" for docs/index.md, "pages" for docs/03-pages.md.
	Slug string
	// Title is the page's first heading, and Description its first paragraph, as a search result shows them.
	Title, Description string
	// HTML is the page: trusted, written by charter build from the app's own Markdown, which Markdown is.
	HTML, Markdown string
	// Headings are the page's sections, for a table of contents.
	Headings []DocHeading
	// Source is the file, from the app's folder: docs/03-pages.md.
	Source string
	// Group is the group the docs' front page lists the page in (a line of docs/index.md that is only bold, above
	// the list that links to it), "" for a page it lists in none. What charter writes of the app itself is in
	// "Reference".
	Group string
}

// DocGroup is pages of the docs under one name, as the list at the side shows them.
type DocGroup struct {
	Name  string
	Pages []Doc
}

// Groups are the docs' pages as the list at the side has them: in their order, those of one group together under
// its name, and those of none (the front page among them) under no name.
func (d *Docs) Groups() []DocGroup {
	var groups []DocGroup
	for _, page := range d.Pages {
		if n := len(groups); n > 0 && groups[n-1].Name == page.Group {
			groups[n-1].Pages = append(groups[n-1].Pages, page)
			continue
		}
		groups = append(groups, DocGroup{Name: page.Group, Pages: []Doc{page}})
	}
	return groups
}

// Around is the page before a page and the one after it, in the order the docs are read: nil at either end.
func (d *Docs) Around(page Doc) (before, after *Doc) {
	for i := range d.Pages {
		if d.Pages[i].Slug != page.Slug {
			continue
		}
		if i > 0 {
			before = &d.Pages[i-1]
		}
		if i+1 < len(d.Pages) {
			after = &d.Pages[i+1]
		}
	}
	return before, after
}

// DocHeading is a section of a Doc: the id its heading has in the HTML, and how deep it is (2 or 3).
type DocHeading struct {
	ID, Title string
	Depth     int
}

// Path is the page's address, without a language: /docs/ or /docs/pages.
func (d Doc) Path() string { return "/docs/" + d.Slug }

// Find is the page with that slug.
func (d *Docs) Find(slug string) (Doc, bool) {
	for _, page := range d.Pages {
		if page.Slug == strings.Trim(slug, "/") {
			return page, true
		}
	}
	return Doc{}, false
}

// Edit is where a page is edited on GitHub, "" when the docs name no repository.
func (d *Docs) Edit(page Doc) string {
	if d.Repo == "" {
		return ""
	}
	return strings.TrimRight(d.Repo, "/") + "/edit/main/" + strings.Trim(strings.Trim(d.Dir, "/")+"/"+page.Source, "/")
}

// paths are the addresses of the pages but the first (/docs/, which the tree lists itself): each a page search
// engines may index.
func (d *Docs) paths() []string {
	var paths []string
	for _, page := range d.Pages {
		if page.Slug != "" {
			paths = append(paths, page.Path())
		}
	}
	return paths
}

// llms is /llms.txt (https://llmstxt.org): what the app is, what a coding agent should work with, and every page of
// the docs as Markdown, with what it is about.
func (d *Docs) llms(origin string) string {
	read := make([]seo.LLMsLink, len(d.Pages))
	for i, page := range d.Pages {
		read[i] = seo.LLMsLink{Title: page.Title, URL: origin + markdownPath(page), Note: page.Description}
	}
	details := d.Agents
	if details != "" {
		details += "\n\n"
	}
	details += "Every page of the docs is Markdown at its address with .md, and all of them are one file at " + origin + "/llms-full.txt."
	return seo.LLMsTxt(d.Name, d.Summary, details, append([]seo.LLMsSection{{Name: "Docs", Links: read}}, d.More...)...)
}

// full is /llms-full.txt: every page of the docs, one after the other, as it was written.
func (d *Docs) full(origin string) string {
	var b strings.Builder
	for _, page := range d.Pages {
		b.WriteString("<!-- " + origin + page.Path() + " -->\n\n" + strings.TrimSpace(page.Markdown) + "\n\n")
	}
	return b.String()
}

func markdownPath(page Doc) string {
	if page.Slug == "" {
		return "/docs/index.md"
	}
	return "/docs/" + page.Slug + ".md"
}

// text answers what a model reads of the docs: a page as Markdown, or all of them. ok is false for any other path.
func (d *Docs) text(w http.ResponseWriter, r *http.Request) (ok bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	var body string
	switch path := r.URL.Path; {
	case path == "/llms-full.txt":
		body = d.full(Origin(r))
	case strings.HasPrefix(path, "/docs/") && strings.HasSuffix(path, ".md"):
		slug := strings.TrimSuffix(strings.TrimPrefix(path, "/docs/"), ".md")
		if slug == "index" {
			slug = ""
		}
		page, found := d.Find(slug)
		if !found {
			return false
		}
		body = page.Markdown
	default:
		return false
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex") // the page itself is what a search engine lists
	w.Write([]byte(body))
	return true
}

// BuiltDocs are the pages charter build wrote from docs/*.md: what the app's docs package calls for its Pages. On a
// Worker they are beside the Wasm (built is empty, and they are read from where the Worker's entry put them), so a
// page that is edited is a new Worker without a new build of the Go; anywhere else built is the package's own copy.
func BuiltDocs(built string) []Doc {
	if built == "" {
		built = besideTheWasm("charterDocs")
	}
	var pages []Doc
	if built != "" {
		if err := json.Unmarshal([]byte(built), &pages); err != nil {
			panic("app: the docs charter build wrote are not what this charter reads: " + err.Error())
		}
	}
	return pages
}
