package test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/pages"
	notes "github.com/joeblew999/charter/example/sdk/go"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// The home page, in a real browser: what a reader sees and does, with htmx, the stream and the scripts Vite built.

// A note typed into one open page shows at once in another: the form, the feed, htmx's SSE swap.
func TestANoteShowsOnEveryOpenPage(t *testing.T) {
	writer := apptest.Browser(t)
	reader := writer.Tab()
	writer.Open("/")
	reader.Open("/")
	body := "typed at " + time.Now().Format(time.RFC3339Nano)
	writer.Type(el(pages.Home.Form)+` input[name=body]`, body)
	writer.Click(el(pages.Home.Form) + ` button[type=submit]`)
	reader.WaitText(el(pages.Home.List), body)
	writer.WaitText(el(pages.Home.List), body)
	if left := writer.Attr(el(pages.Home.Form)+` input[name=body]`, "value"); left != "" {
		t.Errorf("the form still holds %q after posting", left)
	}
}

// A note made through the API, by its SDK, reaches the open page too: one feed behind both.
func TestAnAPINoteShowsOnTheOpenPage(t *testing.T) {
	home := apptest.Browser(t)
	home.Open("/")
	body := "from the SDK at " + time.Now().Format(time.RFC3339Nano)
	writer := sdk(t, option.WithToken(apptest.Secret(t, "WRITE_TOKEN")))
	if _, err := writer.Notes.Create(t.Context(), &notes.CreateNotesRequest{Body: body}); err != nil {
		t.Fatal(err)
	}
	home.WaitText(el(pages.Home.List), body)
}

// Each language has its own address, and Arabic is laid out right to left.
func TestTheLanguages(t *testing.T) {
	home := apptest.Browser(t)
	home.Open("/")
	if lang, dir := home.Attr("html", "lang"), home.Attr("html", "dir"); lang != "en" || dir != "ltr" {
		t.Errorf("/: lang %q dir %q", lang, dir)
	}
	home.Click(`footer a[hreflang=ar]`)
	home.WaitText("h1", "الملاحظات")
	if lang, dir, path := home.Attr("html", "lang"), home.Attr("html", "dir"), home.Path(); lang != "ar" || dir != "rtl" || path != "/ar/" {
		t.Errorf("Arabic: lang %q dir %q at %s", lang, dir, path)
	}
	// The link to the other page keeps the language.
	home.Click(`main a[href$="/bindings"]`)
	home.WaitText("h1", "Cloudflare")
	if path := home.Path(); path != "/ar/bindings" {
		t.Errorf("the Cloudflare page from Arabic is at %s", path)
	}
}

// What the form refuses comes back marked, as a 422 htmx swaps in.
func TestTheFormRefusesAnEmptyNote(t *testing.T) {
	res, err := http.PostForm(apptest.Base(t)+at(pages.PostNote{}), url.Values{"body": {"  "}})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(b), `aria-invalid="true"`) {
		t.Fatalf("HTTP %d\n%s", res.StatusCode, b)
	}
}

// A note's own button opens its row, and only its row: the row asks for itself (its id carries the note's), and the
// answer says when the note was written. The header marks the page being shown.
func TestANotesRowOpensByItself(t *testing.T) {
	home := apptest.Browser(t)
	home.Open(at(pages.Home{}))
	body := "opened at " + time.Now().Format(time.RFC3339Nano)
	home.Type(el(pages.Home.Form)+` input[name=body]`, body)
	home.Click(el(pages.Home.Form) + ` button[type=submit]`)
	home.WaitText(el(pages.Home.List), body)
	first := el(pages.Home.List) + ` li:first-child`
	if open := home.Attr(first+` button`, "aria-expanded"); open != "false" {
		t.Fatalf("the newest row starts %q", open)
	}
	home.Click(first + ` button`)
	home.WaitText(first+` p`, "20") // the year, in the full date
	if open := home.Attr(first+` button`, "aria-expanded"); open != "true" {
		t.Errorf("the row did not open: %q", open)
	}
	if !strings.Contains(home.Text(first), body) {
		t.Errorf("the opened row is another note: %q", home.Text(first))
	}
	if here := home.Attr(`header nav a[href="`+at(pages.Home{})+`"]`, "data-variant"); here != "secondary" {
		t.Errorf("the header does not mark the page being shown: %q", here)
	}
}
