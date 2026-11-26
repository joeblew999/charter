// Package pages is charter's site: a front page, and the docs, which are the Markdown of docs/ shown by charter's
// docs screen (pages/screens, from charter add screen:docs), the same every app's docs get.
package pages

import (
	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/site/pages/screens"
)

// Pages is the tree.
type Pages struct {
	home Home         `route:"GET /{$} Home"`
	docs screens.Docs `route:"/docs Docs"`
}

// Mount mounts the tree: what app.App's Pages is.
func Mount(mux structpages.Mux, options ...structpages.Option) (*structpages.StructPages, error) {
	return structpages.Mount(mux, Pages{}, "/", "charter", options...)
}

// Home is the front page.
type Home struct{}

// Where the site points: the repository, and the example running.
const (
	Repo    = "https://github.com/joeblew999/charter"
	Example = "https://charter-example.gedw99.workers.dev"
)

// start is the three commands that make a new app.
const start = "go run github.com/joeblew999/charter/cmd/charter@latest create my-app\ncd my-app\ngo tool charter dev"
