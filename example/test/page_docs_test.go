package test

import (
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
)

// The docs: Markdown files of docs/, each a page.

// In another language the docs keep the reader's language in their links, and say they are English.
func TestTheDocsInAnotherLanguage(t *testing.T) {
	html := apptest.Get(t, "/de/docs/how-it-is-built")
	for _, want := range []string{`<html lang="de"`, `lang="en" dir="ltr"`, `href="/de/docs/"`} {
		if !strings.Contains(html, want) {
			t.Errorf("/de/docs/how-it-is-built has no %s", want)
		}
	}
}
