package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
)

// The docs: Markdown files of docs/, each a page.

// A reader gets from the home page to the docs, and from one page of them to another by the list at the side.
func TestTheDocsAreReadPageByPage(t *testing.T) {
	page := apptest.Browser(t)
	page.Open("/")
	page.Click(`header a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	if path := page.Path(); path != "/docs/" {
		t.Errorf("the docs are at %s", path)
	}
	page.Click(`#docs-nav a[href$="/docs/database"]`)
	page.WaitText("#doc h1", "The database")
	// Its sections are listed, and its code is there as written.
	page.WaitText("#docs-toc", "Changing it")
	page.WaitText("#doc pre", "CREATE TABLE notes")
	// A link in the prose, written to another page's file, goes to that page.
	page.Open("/docs/")
	page.Click(`#doc a[href="getting-started"]`)
	page.WaitText("#doc h1", "Getting started")
}

// In another language the app around the docs is translated, the docs say they are in English, and their links keep
// the reader's language.
func TestTheDocsInAnotherLanguage(t *testing.T) {
	html := apptest.Get(t, "/de/docs/pages")
	for _, want := range []string{`<html lang="de"`, "Die Dokumentation ist auf Englisch verfasst.", `<article id="doc" lang="en"`, `href="/de/docs/database"`} {
		if !strings.Contains(html, want) {
			t.Errorf("/de/docs/pages has no %s", want)
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

// /llms.txt lists the docs for a language model, each with what it is about, and every page of them is in the
// sitemap.
func TestTheDocsAreListedForAgentsAndSearchEngines(t *testing.T) {
	llms := apptest.Get(t, "/llms.txt")
	for _, want := range []string{"# charter", "## Docs", "/docs/database", "The database", "## Live"} {
		if !strings.Contains(llms, want) {
			t.Errorf("/llms.txt has no %q:\n%s", want, llms)
		}
	}
	sitemap := apptest.Get(t, "/sitemap.xml")
	for _, want := range []string{"/docs/</loc>", "/docs/getting-started</loc>", "/de/docs/tests</loc>"} {
		if !strings.Contains(sitemap, want) {
			t.Errorf("the sitemap has no %s", want)
		}
	}
}
