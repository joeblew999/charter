package test

import (
	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/site/pages"
)

// The tests ask the site's own tree for every address, as its pages do.
var tree = func() *structpages.StructPages {
	sp, err := structpages.Parse(pages.Pages{}, "/", "charter")
	if err != nil {
		panic(err)
	}
	return sp
}()

// at is a route's address: at(pages.Home{}).
func at(page any, params ...any) string {
	path, err := tree.URLFor(page, params...)
	if err != nil {
		panic(err)
	}
	return path
}
