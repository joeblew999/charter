package test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/site/pages"
	"github.com/joeblew999/charter/site/pages/screens"
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

// gsxui's theme editor is in the app, with gsxui's own controls, and the app's own front page is its live preview.
// What a reader chooses there (a theme, a radius, a font: gsxui's preset) the preview shows at once; the browser
// keeps it and every page of the app has it from then on, in the light and in the dark; the page's address carries
// it to someone else; a style the app is not built in is said to be another's; and the app's own theme is one
// press away. Everything that chooses and resolves is gsxui's: what is tested here is that the app has it.
func TestAReaderChoosesATheme(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	variable := func(name string) string {
		return page.JS(`getComputedStyle(document.documentElement).getPropertyValue("--` + name + `").trim()`)
	}
	if page.JS(`String(document.documentElement.classList.contains("dark"))`) == "true" {
		page.Click("[data-theme-toggle]")
	}
	own := variable("primary")
	page.Click(`header a[href$="` + at(screens.Theme{}) + `"]`)
	page.WaitText("main h1", "Theme editor")
	page.WaitText("[data-theme-preview-status]", "Live")
	// In the preview: the app's own front page, as gsxui's preview script has dressed it.
	previewed := func(name string) string {
		return page.JS(`document.querySelector("[data-theme-preview-frame]").contentDocument.documentElement.style.getPropertyValue("--` + name + `").trim()`)
	}
	preset := func() string { return page.JS(`new URL(location.href).searchParams.get("preset") ?? ""`) }
	if title := page.JS(`document.querySelector("[data-theme-preview-frame]").contentDocument.title`); !strings.Contains(title, "charter") {
		t.Errorf("the preview is %q, want the app's front page", title)
	}
	start, plain := preset(), previewed("primary")
	if !strings.HasPrefix(start, "gsxui:") || plain == "" {
		t.Fatalf("the editor starts at %q, with --primary %q in the preview", start, plain)
	}
	// gsxui's own picker for each part: its list is opened, and a choice in it taken.
	pick := func(part, choice string) {
		t.Helper()
		page.Click(`[data-theme-picker="` + part + `"] [data-theme-picker-trigger]`)
		page.Click(`[data-theme-picker="` + part + `"] input[value="` + choice + `"]`)
	}
	changes := func(what string, read func() string, from string) string {
		t.Helper()
		for start := time.Now(); read() == from && time.Since(start) < 5*time.Second; {
			time.Sleep(50 * time.Millisecond)
		}
		if read() == from {
			t.Fatalf("%s is still %q", what, from)
		}
		return read()
	}
	pick("theme", "blue")
	blue := changes("the preview's --primary, after choosing the theme Blue", func() string { return previewed("primary") }, plain)
	radius, font := previewed("radius"), previewed("font-sans")
	pick("radius", "large")
	changes("the preview's --radius, after choosing the radius Large", func() string { return previewed("radius") }, radius)
	pick("fontSans", "inter")
	if font := changes("the preview's --font-sans, after choosing Inter", func() string { return previewed("font-sans") }, font); !strings.Contains(font, "Inter") {
		t.Errorf("the preview's body font is %q", font)
	}
	chosen := changes("the address's preset", preset, start)
	// Every page of the app has it from then on, in the light and in the dark.
	page.Open(at(pages.Home{}))
	if got := variable("primary"); got != blue {
		t.Errorf("on the front page, --primary is %q, want the chosen theme's %q", got, blue)
	}
	if !strings.Contains(variable("font-sans"), "Inter") {
		t.Errorf("on the front page, the body font is %q", variable("font-sans"))
	}
	page.Click("[data-theme-toggle]")
	if got := variable("primary"); got == blue || got == own || got == "" {
		t.Errorf("the chosen theme in the dark: --primary %q (its light is %q, the app's own %q)", got, blue, own)
	}
	page.Click("[data-theme-toggle]")
	// The editor comes back to what was chosen, and its address gives it to another browser.
	page.Open(at(screens.Theme{}))
	page.WaitText("[data-theme-preview-status]", "Live")
	if got := preset(); got != chosen {
		t.Errorf("the editor came back at %q, want what was chosen: %q", got, chosen)
	}
	other := apptest.Browser(t)
	other.Open(at(screens.Theme{}) + "?preset=" + chosen)
	other.WaitText("[data-theme-preview-status]", "Live")
	other.Open(at(pages.Home{}))
	if got := other.JS(`getComputedStyle(document.documentElement).getPropertyValue("--primary").trim()`); got != blue && other.JS(`String(document.documentElement.classList.contains("dark"))`) != "true" {
		t.Errorf("another browser, given the address: --primary %q, want %q", got, blue)
	}
	// A style is the components' own markup, where the rest is CSS: choosing Lyra, the preview is asked for again
	// and its button is Lyra's, and every page of the app is from then on.
	button := `document.querySelector("main a[data-variant]").className`
	drawn := func() string {
		return page.JS(`document.querySelector("[data-theme-preview-frame]").contentDocument?.querySelector("main a[data-variant]")?.className ?? ""`)
	}
	nova := drawn()
	if nova == "" {
		t.Fatal("the preview has no button on the front page to tell a style by")
	}
	page.Click(`[data-theme-style="lyra"]`)
	changes("the address's preset, after choosing the style Lyra", preset, chosen)
	lyra := changes("the button in the preview, after choosing the style Lyra", func() string {
		if now := drawn(); now != "" {
			return now
		}
		return nova // the preview is between two pages
	}, nova)
	page.WaitText("[data-theme-preview-status]", "Live")
	if got := previewed("primary"); got != blue {
		t.Errorf("in the preview in Lyra, --primary is %q, want the chosen theme's still: %q", got, blue)
	}
	page.Open(at(pages.Home{}))
	if got := page.JS(button); got != lyra {
		t.Errorf("on the front page after choosing Lyra, the button is drawn %q, want Lyra's %q", got, lyra)
	}
	if link := page.Attr(`header a[href$="`+at(screens.Theme{})+`"]`, "class"); link == "" {
		t.Error("the header has no way back to the theme editor")
	}
	page.Open(at(screens.Theme{}))
	page.WaitText("[data-theme-preview-status]", "Live")
	// And the app's own again.
	page.Click("[data-charter-own]")
	page.WaitText("[data-theme-preview-status]", "Live")
	page.Open(at(pages.Home{}))
	if got := variable("primary"); got != own {
		t.Errorf("after choosing the app's own theme: --primary %q, want %q", got, own)
	}
	if got := page.JS(button); got != nova {
		t.Errorf("after choosing the app's own theme, the button is drawn %q, want the app's own style's %q", got, nova)
	}
}
