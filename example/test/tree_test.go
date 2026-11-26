package test

import (
	"github.com/jackielii/structpages"

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
