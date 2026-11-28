package test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/site/pages"
)

// charter's own site: a front page, and the docs, shown by the pages every charter app's docs get.

// A reader gets from the front page to the docs, through them by the list at the side, and from any page of them to
// the repository and to that page's own file.
func TestFromTheFrontPageThroughTheDocs(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	page.WaitText("h1", "Go apps on Cloudflare Workers")
	if repo := page.Attr("#github", "href"); repo != pages.Repo {
		t.Errorf("the front page links the repository as %q", repo)
	}
	page.Click(`main a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	page.Click(`[data-gsxui-slot-sidebar-desktop] a[href$="/docs/getting-started"]`)
	page.WaitText("#doc h1", "Getting started")
	page.WaitText("#doc pre", "go tool charter dev")
	if repo, edit := page.Attr("#docs-repo", "href"), page.Attr("#docs-edit", "href"); repo != pages.Repo || edit != pages.Repo+"/edit/main/site/docs/01-getting-started.md" {
		t.Errorf("repository %q, edit %q", repo, edit)
	}
	page.Click(`[data-gsxui-slot-sidebar-desktop] a[href$="/docs/agents"]`)
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

// The docs are searched by the titles of their pages and sections: the search opens, a few letters leave what
// matches, and choosing one goes to it. It is the docs screen every app has, tested here once.
func TestTheDocsAreSearched(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	page.Click(`main a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	page.Click(`#docs-search`)
	page.Type(`[data-gsxui-slot-command-input]`, "getting st")
	page.WaitText(`[data-gsxui-slot-command-list]`, "Getting started")
	page.Click(`[data-gsxui-slot-command-item]:not([hidden])`)
	page.WaitText("#doc h1", "Getting started")
}

// The Go that starts as the Worker's module loads is there for the first request: when it is not, the Worker says
// so to whoever asks which runtime answered. A local Worker allows while its module loads what Cloudflare refuses,
// so it is the deployed one this is for.
func TestTheWorkersGoStartedWithIt(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, apptest.Base(t)+at(pages.Home{}), nil)
	req.Header.Set("x-go-runtime", "?")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	said := res.Header.Get("x-go-runtime")
	if said == "" {
		t.Fatal("the Worker does not say which runtime answered (x-go-runtime)")
	}
	if strings.Contains(said, "failed") {
		t.Errorf("the Go that started with the Worker did not last: %s", said)
	}
}

// On a phone there is no room for the list of pages at the side: it slides in from a bar that stays at the top,
// in the groups the docs' front page has. A link in it shows its page with the list gone, and the list opens again
// from there. The page's own sections come down from the same bar, and go when one is chosen.
func TestTheDocsOnAPhone(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Phone()
	page.Open(at(pages.Home{}))
	page.Click(`main a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	// The list of pages and the page's sections are each a dialog of gsxui's: whether one is open.
	const list = "[data-gsxui-slot-sidebar-mobile-content]"
	open := func(selector string) string {
		return page.JS(fmt.Sprintf(`String(document.querySelector(%q).open)`, selector))
	}
	if got := open(list); got != "false" {
		t.Fatalf("before it is opened, the list of pages is open: %s", got)
	}
	page.Click("#docs-menu")
	page.WaitText(list, "Ship and watch")
	if got := open(list); got != "true" {
		t.Fatalf("opened, the list of pages is open: %s", got)
	}
	page.Click(list + ` a[href$="/docs/getting-started"]`)
	page.WaitText("#doc h1", "Getting started")
	if got := open(list); got != "false" {
		t.Errorf("after a link in it was followed, the list of pages is open: %s", got)
	}
	page.Click("#docs-menu")
	if got := open(list); got != "true" {
		t.Errorf("opened again on the next page, the list of pages is open: %s", got)
	}
	// The sections of the page, from a page loaded anew: chosen, the list is gone and the page is at the section.
	page.Open(at(pages.Home{}))
	page.Click(`main a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	page.Click("#docs-sections")
	if got := open("#docs-sections-list"); got != "true" {
		t.Fatalf("opened, the page's sections are open: %s", got)
	}
	page.Click(`#docs-sections-list a[href="#where-to-go"]`)
	// It goes as gsxui's sheets go, sliding away: until it has.
	for start := time.Now(); open("#docs-sections-list") == "true" && time.Since(start) < 5*time.Second; {
		time.Sleep(50 * time.Millisecond)
	}
	if got := open("#docs-sections-list"); got != "false" {
		t.Errorf("after a section was chosen, the sections are open: %s", got)
	}
	if hash := page.JS(`location.hash`); hash != "#where-to-go" {
		t.Errorf("choosing a section went to %q", hash)
	}
	// Nothing on a page is wider than a phone: a table or a sample that is scrolls by itself.
	if wider := page.JS(`String(document.documentElement.scrollWidth - window.innerWidth)`); wider != "0" {
		t.Errorf("the page is %s px wider than the window", wider)
	}
}

// The list at the side is in the groups the docs' front page names, and the docs are read from one end to the
// other by the links at the foot of each page, in that order, without going back to the list.
func TestTheDocsAreReadInOrder(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	page.Click(`main a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	for _, group := range []string{"Start", "Write", "Prove", "Ship and watch", "Reference"} {
		page.WaitText("[data-gsxui-slot-sidebar-desktop]", group)
	}
	if page.Has("#docs-previous") {
		t.Error("the first page has a page before it")
	}
	page.WaitText("#docs-next", "Getting started")
	page.Click("#docs-next")
	page.WaitText("#doc h1", "Getting started")
	page.WaitText("#docs-previous", "charter")
	page.WaitText("#docs-next", "The commands")
	// Where the page is among the others is said at the top: its group, and itself.
	page.WaitText("#docs-here", "Getting started")
	page.Click("#docs-next")
	page.WaitText("#doc h1", "The commands, and what each one proves")
	if here := page.Attr(`[data-gsxui-slot-sidebar-desktop] a[aria-current="page"]`, "href"); !strings.HasSuffix(here, "/docs/commands") {
		t.Errorf("the list marks %q as the page being read", here)
	}
}

// A reader chooses among gsxui's themes as the page runs: the page takes the theme's colours at once, in the light
// and in the dark, each its own; the choice is kept across a link and a page loaded anew; and the app's own theme
// is one press away. The themes are gsxui's, resolved by gsxui (web/gsxui/themes.css): what is tested here is that
// the page has them.
func TestAReaderChoosesATheme(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	page.Click(`main a[href$="/docs/"]`)
	page.WaitText("#doc h1", "charter")
	variable := func(name string) string {
		return page.JS(`getComputedStyle(document.documentElement).getPropertyValue("--` + name + `").trim()`)
	}
	theme := func() string { return page.JS(`document.documentElement.getAttribute("data-theme") ?? ""`) }
	dark := func() string { return page.JS(`String(document.documentElement.classList.contains("dark"))`) }
	if dark() == "true" {
		page.Click("[data-theme-toggle]")
	}
	own := variable("primary")
	page.Click("[data-theme-picker]")
	if n := page.JS(`String(document.querySelectorAll("[data-theme-choice]").length)`); n != "18" {
		t.Fatalf("the list has %s choices, want the app's own and gsxui's 17", n)
	}
	page.Click(`[data-theme-choice="blue"]`)
	blue := variable("primary")
	if theme() != "blue" || blue == own || blue == "" {
		t.Fatalf("after choosing blue: data-theme %q, --primary %q (the app's own is %q)", theme(), blue, own)
	}
	// The list marks the one in force, and only that one.
	marked := func(name string) string {
		return page.JS(`getComputedStyle(document.querySelector('[data-theme-choice="` + name + `"] [data-theme-chosen]')).visibility`)
	}
	if marked("blue") != "visible" || marked("rose") != "hidden" || marked("") != "hidden" {
		t.Errorf("the mark: blue %s, rose %s, the app's own %s", marked("blue"), marked("rose"), marked(""))
	}
	// A theme has its dark: another primary, and the dark's own text on what the light had dark text on.
	lightText := variable("secondary-foreground")
	page.Click("[data-theme-toggle]")
	if dark() != "true" || variable("primary") == blue || variable("secondary-foreground") == lightText {
		t.Errorf("blue in the dark: dark %s, --primary %q (light: %q), --secondary-foreground %q (light: %q)", dark(), variable("primary"), blue, variable("secondary-foreground"), lightText)
	}
	page.Click("[data-theme-toggle]")
	// Kept across a link, and across a page loaded anew.
	page.Click("#docs-next")
	page.WaitText("#doc h1", "Getting started")
	if theme() != "blue" || variable("primary") != blue {
		t.Errorf("after a link: data-theme %q, --primary %q", theme(), variable("primary"))
	}
	page.Open(at(pages.Home{}))
	if theme() != "blue" || variable("primary") != blue {
		t.Errorf("on a page loaded anew: data-theme %q, --primary %q", theme(), variable("primary"))
	}
	// And the app's own again.
	page.Click("[data-theme-picker]")
	page.Click(`[data-theme-choice=""]`)
	if theme() != "" || variable("primary") != own {
		t.Errorf("after choosing the app's own: data-theme %q, --primary %q, want %q", theme(), variable("primary"), own)
	}
}
