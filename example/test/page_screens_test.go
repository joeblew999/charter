package test

import (
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
)

// charter's screens, as the example has them around every page (pages/screens): what a reader does with them.

// The theme switch turns the page dark, and the page stays dark when it is loaded again: the reader's choice is
// kept, and is there before the first paint.
func TestTheThemeIsSwitchedAndKept(t *testing.T) {
	home := apptest.Browser(t)
	home.Open(at(pages.Home{}))
	was := strings.Contains(home.Attr("html", "class"), "dark")
	home.Click(`header [data-theme-toggle]`)
	if now := strings.Contains(home.Attr("html", "class"), "dark"); now == was {
		t.Fatalf("the switch left the page as it was (dark: %v)", now)
	}
	home.Open(at(pages.Home{}))
	if now := strings.Contains(home.Attr("html", "class"), "dark"); now == was {
		t.Errorf("the choice was not kept: the page is as before the switch (dark: %v)", now)
	}
	// The docs' pages are the same app: they have the choice too, and their own switch.
	home.Open(at(screens.DocsIndex{}))
	if now := strings.Contains(home.Attr("html", "class"), "dark"); now == was {
		t.Errorf("the docs do not have the reader's theme")
	}
	home.Click(`header [data-theme-toggle]`)
	if now := strings.Contains(home.Attr("html", "class"), "dark"); now != was {
		t.Errorf("the docs' switch did not switch back")
	}
}

// The time zone and clock dialog opens from the header, and its form arrives from its route in the page tree.
func TestTheTimeZoneDialog(t *testing.T) {
	home := apptest.Browser(t)
	home.Open(at(pages.Home{}))
	home.Click(`header [aria-haspopup=dialog]`)
	home.WaitText("#preferences", "Time zone")
	if action := home.Attr("#preferences form", "action"); !strings.HasSuffix(action, at(screens.PreferencesSave{})) {
		t.Errorf("the form goes to %q", action)
	}
}

// A reader whose browser asks for German is offered the page in German, in German, and is not sent there.
func TestThePageIsOfferedInTheReadersLanguage(t *testing.T) {
	html := apptest.Get(t, at(pages.Home{}), "Accept-Language", "de")
	if !strings.Contains(html, `<html lang="en"`) || !strings.Contains(html, "Diese Seite gibt es auch auf") || !strings.Contains(html, "Nein danke") || !strings.Contains(html, `href="/de/"`) {
		t.Errorf("no offer of the German page on the English one:\n%s", html)
	}
}
