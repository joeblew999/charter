package test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
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
	writer.Type(`#post input[name=body]`, body)
	writer.Click(`#post button[type=submit]`)
	reader.WaitText("#notes", body)
	writer.WaitText("#notes", body)
	if left := writer.Attr(`#post input[name=body]`, "value"); left != "" {
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
	home.WaitText("#notes", body)
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
	res, err := http.PostForm(apptest.Base(t)+"/notes", url.Values{"body": {"  "}})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(b), `aria-invalid="true"`) {
		t.Fatalf("HTTP %d\n%s", res.StatusCode, b)
	}
}
