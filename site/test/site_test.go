package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/site/pages"
)

// charter's own site: a front page, and the docs, shown by the pages every charter app's docs get.

// A reader gets from the front page to the docs, through them by the list at the side, and from any page of them to
// the repository and to that page's own file.
func TestFromTheFrontPageThroughTheDocs(t *testing.T) {
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	page.WaitText("h1", "Go apps on Cloudflare Workers")
	if repo := page.Attr("#github", "href"); repo != pages.Repo {
		t.Errorf("the front page links the repository as %q", repo)
	}
	page.Click(`main a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	page.Click(`#docs-nav a[href$="/docs/getting-started"]`)
	page.WaitText("#doc h1", "Getting started")
	page.WaitText("#doc pre", "go tool charter dev")
	if repo, edit := page.Attr("#docs-repo", "href"), page.Attr("#docs-edit", "href"); repo != pages.Repo || edit != pages.Repo+"/edit/main/site/docs/01-getting-started.md" {
		t.Errorf("repository %q, edit %q", repo, edit)
	}
	page.Click(`#docs-nav a[href$="/docs/agents"]`)
	page.WaitText("#doc h1", "For your coding agent")
	page.WaitText("#doc", "/plugin marketplace add joeblew999/charter")
}

// A page of the docs that does not exist is a 404.
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

// What a language model reads: /llms.txt opens with the plugins an agent works with, then lists every page as
// Markdown and the example running; each page is Markdown at its address with .md; /llms-full.txt is all of them.
func TestTheDocsForAgentsAndSearchEngines(t *testing.T) {
	llms := apptest.Get(t, "/llms.txt")
	for _, want := range []string{"# charter", "marketplace joeblew999/charter", "## Docs", "/docs/agents.md", "/docs/database.md", "/llms-full.txt", "## The example, running"} {
		if !strings.Contains(llms, want) {
			t.Errorf("/llms.txt has no %q:\n%s", want, llms)
		}
	}
	if md := apptest.Get(t, "/docs/agents.md"); !strings.HasPrefix(md, "# For your coding agent") {
		t.Errorf("/docs/agents.md is not the page's Markdown:\n%s", md)
	}
	if full := apptest.Get(t, "/llms-full.txt"); !strings.Contains(full, "# Getting started") || !strings.Contains(full, "# Deploy") {
		t.Errorf("/llms-full.txt is not every page")
	}
	sitemap := apptest.Get(t, "/sitemap.xml")
	for _, want := range []string{"/docs/</loc>", "/docs/agents</loc>", "/docs/deploy</loc>"} {
		if !strings.Contains(sitemap, want) {
			t.Errorf("the sitemap has no %s", want)
		}
	}
}
