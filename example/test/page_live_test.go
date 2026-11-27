package test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
)

// Two open pages keep up with each other: what one adds and takes away, the other shows, as it is, in order, once.

// shownIn is the notes of this test that a page's list shows, top to bottom, joined by "|".
func shownIn(p *apptest.Page, stamp string) string {
	return p.JS(fmt.Sprintf(`[...document.querySelectorAll(%q)].map(li => li.querySelector("span[dir]")?.textContent ?? "").filter(text => text.startsWith(%q)).map(text => text.slice(%d)).join("|")`,
		el(pages.Home.List)+" > li", stamp, len(stamp)))
}

// settles waits until a page's list shows want, and fails with what it shows when it does not.
func settles(t *testing.T, who string, p *apptest.Page, stamp, want string) {
	t.Helper()
	var got string
	for start := time.Now(); time.Since(start) < 15*time.Second; time.Sleep(150 * time.Millisecond) {
		if got = shownIn(p, stamp); got == want {
			return
		}
	}
	t.Fatalf("the %s's page shows %q, want %q", who, got, want)
}

func TestTwoOpenPagesKeepUp(t *testing.T) {
	writer := apptest.Browser(t)
	reader := writer.Tab()
	writer.SignUp(at(screens.SignInPage{}), "Writer")
	writer.Open("/")
	reader.Open("/")
	stamp := "keeps up " + time.Now().Format("15:04:05.000") + " #"
	form := el(pages.Home.Form)

	// Five notes, one straight after the other, as fast as the page takes them: all at once, as someone's posts and
	// someone else's arrive together.
	writer.JS(fmt.Sprintf(`(() => {
		const post = document.querySelector(%q).getAttribute("hx-post");
		for (let i = 1; i <= 5; i++) fetch(post, {method: "POST", headers: {"HX-Request": "true"}, body: new URLSearchParams({body: %q + i})});
		return "sent";
	})()`, form, stamp))
	for who, page := range map[string]*apptest.Page{"reader": reader, "writer": writer} {
		var got string
		for start := time.Now(); time.Since(start) < 15*time.Second; time.Sleep(150 * time.Millisecond) {
			if got = shownIn(page, stamp); len(strings.Split(got, "|")) == 5 {
				break
			}
		}
		// Each once, whatever order they were written in.
		seen := strings.Split(got, "|")
		for i := 1; i <= 5; i++ {
			if n := strings.Count("|"+got+"|", fmt.Sprintf("|%d|", i)); n != 1 {
				t.Errorf("the %s's page shows note %d %d times: %q", who, i, n, got)
			}
		}
		if len(seen) != 5 {
			t.Fatalf("the %s's page shows %d of the 5 notes: %q", who, len(seen), got)
		}
	}
	// Both show them in the same order: the newest first.
	order := shownIn(writer, stamp)
	settles(t, "reader", reader, stamp, order)

	// The writer takes two away: they go from the reader's page too.
	kept := strings.Split(order, "|")
	for _, gone := range []string{kept[1], kept[3]} {
		writer.JS(fmt.Sprintf(`(() => {
			const row = [...document.querySelectorAll(%q)].find(li => (li.querySelector("span[dir]")?.textContent ?? "") === %q);
			row.querySelector("[data-remove]").click();
			return "clicked";
		})()`, el(pages.Home.List)+" > li", stamp+gone))
	}
	want := strings.Join([]string{kept[0], kept[2], kept[4]}, "|")
	settles(t, "writer", writer, stamp, want)
	settles(t, "reader", reader, stamp, want)

	// And one more, typed the ordinary way: it shows on both, at the top.
	writer.Type(form+` input[name=body]`, stamp+"6")
	writer.Click(form + ` button[type=submit]`)
	settles(t, "reader", reader, stamp, "6|"+want)
	settles(t, "writer", writer, stamp, "6|"+want)

	// What is left is taken away, so the Worker is as it was.
	for range 4 {
		writer.JS(fmt.Sprintf(`(() => {
			const row = [...document.querySelectorAll(%q)].find(li => (li.querySelector("span[dir]")?.textContent ?? "").startsWith(%q));
			row?.querySelector("[data-remove]").click();
			return "clicked";
		})()`, el(pages.Home.List)+" > li", stamp))
		time.Sleep(300 * time.Millisecond)
	}
	settles(t, "reader", reader, stamp, "")
}
