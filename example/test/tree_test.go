package test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/pages"
)

// The tests ask the app's own tree for every address and every element id, as its pages do: a route or a component
// that is renamed breaks them when they are compiled, not when a selector quietly matches nothing.
var tree = func() *structpages.StructPages {
	sp, err := structpages.Parse(pages.Pages{}, "/", "charter-example")
	if err != nil {
		panic(err)
	}
	return sp
}()

// at is a route's address: at(pages.BuiltIndex{}), at(pages.AttachedFile{}, "file", 1).
func at(page any, params ...any) string {
	path, err := tree.URLFor(page, params...)
	if err != nil {
		panic(err)
	}
	return path
}

// el is the CSS selector of a page's component: el(pages.Home.Form) is "#home-form".
func el(component any) string {
	id, err := tree.IDTarget(component)
	if err != nil {
		panic(err)
	}
	return id
}

// row is the CSS selector of a note's row on the home page, by the note's id: the row's own id, from the tree. A
// test finds its own note by it, wherever in the list that is, since other tests write notes while it runs.
func row(note int64) string {
	id, err := tree.KeyTarget(pages.Home.Item, strconv.FormatInt(note, 10))
	if err != nil {
		panic(err)
	}
	return id
}

// rowWith is the CSS selector of the row on the open home page that has this text: for a note a test typed into
// the page's form, whose id it was never told.
func rowWith(t *testing.T, page *apptest.Page, text string) string {
	t.Helper()
	id := page.JS(fmt.Sprintf(`[...document.querySelectorAll(%q)].find(li => li.textContent.includes(%q))?.id ?? ""`, el(pages.Home.List)+" li", text))
	if id == "" {
		t.Fatalf("the page has no row with %q", text)
	}
	return "#" + id
}
