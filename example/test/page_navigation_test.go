package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/env"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// A link to another page of the app shows it with no page load: the document stays, and the next page's body takes
// the place of this one's (app.Body). What must be a page load still is one.

// marked puts a mark on a page's document, and sameDocument says whether it is still there: a page load makes a new
// document, which has none.
func marked(p *apptest.Page) { p.JS(`(window.__sameDocument = true, "marked")`) }

func sameDocument(p *apptest.Page) bool {
	return p.JS(`String(window.__sameDocument === true)`) == "true"
}

// arrives waits until a page is at path, and fails with where it is when it does not get there.
func arrives(t *testing.T, p *apptest.Page, path string) {
	t.Helper()
	for start := time.Now(); time.Since(start) < 10*time.Second; time.Sleep(100 * time.Millisecond) {
		if p.Path() == path {
			return
		}
	}
	t.Fatalf("the page is at %s, want %s", p.Path(), path)
}

// shown is the link to path that the page shows: the header has each way to a page twice, in itself and in its
// menu's drawer for a window with no room, and main may have one too.
func shown(path string) string {
	return fmt.Sprintf(`:is([data-layout-links], main) a[href=%q]`, path)
}

// follow clicks the link to path that the page shows.
func follow(p *apptest.Page, path string) {
	p.Click(shown(path))
}

func TestALinkShowsTheNextPageWithNoPageLoad(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open("/")
	marked(page)
	home := page.JS(`document.title`)

	// To another page of the same layout: the same document, the new page's heading and title, the header's mark
	// on the page it is now.
	built := at(pages.BuiltIndex{})
	follow(page, built)
	arrives(t, page, built)
	if !sameDocument(page) {
		t.Fatal("a link to another page of the app loaded a new document")
	}
	// The page's own title comes with it, once it is in place.
	title := home
	for start := time.Now(); time.Since(start) < 5*time.Second && title == home; time.Sleep(100 * time.Millisecond) {
		title = page.JS(`document.title`)
	}
	if title == home || title == "" {
		t.Errorf("the title after the link is %q, as on the page before", title)
	}
	if current := page.JS(fmt.Sprintf(`document.querySelector('[data-layout-links] a[href=%q]')?.dataset.variant ?? ""`, built)); current != "secondary" {
		t.Errorf("the header marks the page it is on as %q, want secondary", current)
	}

	// Back shows the page that was there, in the same document.
	page.JS(`(history.back(), "back")`)
	arrives(t, page, "/")
	if !sameDocument(page) {
		t.Error("Back loaded a new document")
	}
	page.WaitText("h1", "")

	// To a page of another layout of the app (the docs'), still no page load.
	docs := at(screens.DocsIndex{})
	follow(page, docs)
	arrives(t, page, docs)
	page.WaitText("#doc h1", "")
	if !sameDocument(page) {
		t.Error("a link to the docs loaded a new document")
	}
}

// A page's list keeps itself current when the page was arrived at by a link, as when it was loaded: its stream is
// opened as the page is put in place.
func TestAPageArrivedAtByALinkIsLive(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open(at(pages.BuiltIndex{}))
	marked(page)
	follow(page, at(pages.Home{}))
	arrives(t, page, at(pages.Home{}))
	if !sameDocument(page) {
		t.Fatal("the link home loaded a new document")
	}
	body := "after a link at " + time.Now().Format(time.RFC3339Nano)
	write(t, sdk(t, option.WithToken(apptest.Secret(t, env.WRITE_TOKEN))), body)
	page.WaitText(el(pages.Home.List), body)
}

// What must be a page load is one: another language (<html lang> changes), and the sign-in screen, whose script
// runs as its document loads.
func TestWhatMustBeAPageLoadIsOne(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open("/")
	marked(page)
	signIn := at(screens.SignInPage{})
	follow(page, signIn)
	arrives(t, page, signIn)
	if sameDocument(page) {
		t.Error("a link to the sign-in page kept the document: its script would not have run")
	}

	// Last: the language chosen is kept, and every link after it is that language's.
	page.Open("/")
	marked(page)
	page.Click(`footer a[hreflang=de]`)
	arrives(t, page, "/de/")
	if sameDocument(page) {
		t.Error("a link to another language kept the document: <html lang> would be the old one")
	}
	if lang := page.Attr("html", "lang"); lang != "de" {
		t.Errorf("<html lang> is %q after the link to German", lang)
	}
}

// asked is how many times a page has asked for path since it loaded, as the browser counts its requests.
func askedFor(p *apptest.Page, path string) string {
	return p.JS(fmt.Sprintf(`String(performance.getEntriesByType("resource").filter(e => new URL(e.name).pathname === %q).length)`, path))
}

// press presses a link without letting go: what a pointer does before a click is done.
func press(p *apptest.Page, selector string) {
	p.JS(fmt.Sprintf(`(document.querySelector(%q).dispatchEvent(new MouseEvent("mousedown", {bubbles: true, cancelable: true, button: 0})), "pressed")`, selector))
}

// The next page is asked for when its link is pressed, before the click is done, and the click uses that answer:
// one request, not two. A link to another language is not asked for on a press: following one chooses the language.
func TestAPressedLinkIsAskedForOnce(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open("/")
	built := at(pages.BuiltIndex{})
	link := shown(built)

	press(page, `footer a[hreflang=de]`)
	press(page, link)
	// Until the answer to the press is in: how long that is is the Worker's to say (a new instance takes a second).
	n := askedFor(page, built)
	for start := time.Now(); n == "0" && time.Since(start) < 20*time.Second; n = askedFor(page, built) {
		time.Sleep(100 * time.Millisecond)
	}
	if n != "1" {
		t.Fatalf("pressing the link asked for %s %s times, want once", built, n)
	}
	if n := askedFor(page, "/de/"); n != "0" {
		t.Errorf("pressing the link to German asked for it %s times: it must not be asked for until it is followed", n)
	}
	if page.Path() != "/" {
		t.Fatalf("a press alone went to %s", page.Path())
	}
	page.Click(link)
	arrives(t, page, built)
	time.Sleep(500 * time.Millisecond)
	if n := askedFor(page, built); n != "1" {
		t.Errorf("after the click %s was asked for %s times, want the once of the press", built, n)
	}
}
