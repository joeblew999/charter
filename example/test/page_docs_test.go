package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
)

// The docs: Markdown files of docs/, each a page.

// A reader gets from the home page to the docs, and from one page of them to another by the list at the side.
func TestTheDocsAreReadPageByPage(t *testing.T) {
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	page.Click(`header a[href$="/docs/"]`)
	page.WaitText("#doc h1", "The example")
	if path := page.Path(); path != "/docs/" {
		t.Errorf("the docs are at %s", path)
	}
	page.Click(`#docs-nav a[href$="/docs/how-it-is-built"]`)
	page.WaitText("#doc h1", "How it is built")
	// Its sections are listed, and its code is there as written.
	page.WaitText("#docs-toc", "A page declares what it needs")
	page.WaitText("#doc pre", "store api.Store")
	// A link in the prose, written to another page's file, goes to that page.
	page.Click(`#doc a[href="./"]`)
	page.WaitText("#doc h1", "The example")
	// Where it comes from: the repository, and the page's own file to edit.
	if repo, edit := page.Attr("#docs-repo", "href"), page.Attr("#docs-edit", "href"); !strings.HasPrefix(repo, "https://") || !strings.HasPrefix(edit, repo+"/edit/main/") || !strings.HasSuffix(edit, "/docs/index.md") {
		t.Errorf("repository %q, edit %q", repo, edit)
	}
}

// In another language the docs keep the reader's language in their links, and say they are English.
func TestTheDocsInAnotherLanguage(t *testing.T) {
	html := apptest.Get(t, "/de/docs/how-it-is-built")
	for _, want := range []string{`<html lang="de"`, `lang="en" dir="ltr"`, `href="/de/docs/"`} {
		if !strings.Contains(html, want) {
			t.Errorf("/de/docs/how-it-is-built has no %s", want)
		}
	}
}

// A page of the docs that does not exist is a 404, not an empty page.
func TestADocsPageThatIsNotThere(t *testing.T) {
	res, err := http.Get(apptest.Base(t) + "/docs/no-such-page")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("HTTP %d, want 404", res.StatusCode)
	}
}

// What a language model reads: /llms.txt says which plugins an agent works with and lists every page as Markdown;
// each page is Markdown at its address with .md; /llms-full.txt is all of them. Every page is in the sitemap.
func TestTheDocsForAgentsAndSearchEngines(t *testing.T) {
	llms := apptest.Get(t, "/llms.txt")
	for _, want := range []string{"# charter-example", "marketplace joeblew999/charter", "## Docs", "/docs/how-it-is-built.md", "/llms-full.txt"} {
		if !strings.Contains(llms, want) {
			t.Errorf("/llms.txt has no %q:\n%s", want, llms)
		}
	}
	if md := apptest.Get(t, "/docs/how-it-is-built.md"); !strings.HasPrefix(md, "# How it is built") {
		t.Errorf("/docs/how-it-is-built.md is not the page's Markdown:\n%s", md)
	}
	if full := apptest.Get(t, "/llms-full.txt"); !strings.Contains(full, "# The example") || !strings.Contains(full, "# How it is built") {
		t.Errorf("/llms-full.txt is not every page")
	}
	sitemap := apptest.Get(t, "/sitemap.xml")
	for _, want := range []string{"/docs/</loc>", "/docs/how-it-is-built</loc>", "/de/docs/how-it-is-built</loc>"} {
		if !strings.Contains(sitemap, want) {
			t.Errorf("the sitemap has no %s", want)
		}
	}
}

// The docs are searched by the titles of their pages and sections: the search opens, a few letters leave what
// matches, and choosing a section goes to it.
func TestTheDocsAreSearched(t *testing.T) {
	page := apptest.Browser(t)
	page.Open(at(screens.DocsIndex{}))
	page.Click(`#docs-search`)
	page.Type(`[data-gsxui-slot-command-input]`, "declares")
	page.WaitText(`[data-gsxui-slot-command-list]`, "A page declares what it needs")
	page.Click(`[data-gsxui-slot-command-item]:not([hidden])`)
	page.WaitText("#doc h1", "How it is built")
	if path := page.Path(); path != at(screens.DocsPage{}, "slug", "how-it-is-built") {
		t.Errorf("the search went to %s", path)
	}
}

// Code in the docs is marked up by what each word is, for the theme to colour.
func TestTheDocsCodeIsMarkedUp(t *testing.T) {
	html := apptest.Get(t, at(screens.DocsPage{}, "slug", "how-it-is-built"))
	if !strings.Contains(html, `<pre class="chroma">`) || !strings.Contains(html, `<span class="kd">func</span>`) {
		t.Errorf("the code is plain:\n%s", html)
	}
}
