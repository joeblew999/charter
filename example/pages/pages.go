// Package pages is the site's pages, as a tree structpages reads: every route is a field of Pages, with its route in
// the field's tag. A page is a type with a Page component (and Props for its data); what a page calls is a type with
// ServeHTTP. Links and form targets come from the tree ({ Bindings{} |> url }, { PostNote{} |> action }), never a path
// written as a string, and the pages search engines may index are the tree's GET pages.
package pages

import (
	"bytes"
	"context"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/bindings"
)

// Pages is the tree.
type Pages struct {
	home     Home     `route:"GET /{$} Home"`
	bindings Bindings `route:"GET /bindings Cloud"`

	// What the pages call: routes, not pages.
	post   PostNote       `route:"POST /notes PostNote"`
	stream NotesStream    `route:"GET /notes/stream NotesStream"`
	act    BindingsAction `route:"POST /bindings/{action} BindingsAction"`
}

// Mount mounts the tree: what app.App's Pages is. The call is here so that structpages-lint finds the tree and can
// check every link against it.
func Mount(mux structpages.Mux, options ...structpages.Option) (*structpages.StructPages, error) {
	return structpages.Mount(mux, Pages{}, "/", "charter-example", options...)
}

// The newest notes the home page shows.
const shown = 50

// Home is the home page: the form, and the newest notes, which the feed keeps current.
type Home struct{}

// HomeProps is what Home shows: the notes, newest first, and the id of the newest, where its stream starts, so
// nothing between the two is lost.
type HomeProps struct {
	Notes []api.Note
	After int64
}

func (Home) Props(r *http.Request) (HomeProps, error) {
	store, err := api.Open()
	if err != nil {
		return HomeProps{}, err
	}
	notes, err := store.Before(r.Context(), math.MaxInt64, shown)
	if err != nil {
		return HomeProps{}, err
	}
	props := HomeProps{Notes: notes}
	if len(notes) > 0 {
		props.After = notes[0].ID
	}
	return props, nil
}

// PostNote is the form's target: it writes the note as the API's createNote does (api.Post), and answers with the
// form again, empty, or with what was typed and why it was refused. The note itself reaches this page as it reaches
// every other: by the stream.
type PostNote struct{}

// The longest note the form takes.
const maxBody = 500

func (PostNote) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body := strings.TrimSpace(r.PostFormValue("body"))
	switch {
	case body == "" || len(body) > maxBody || strings.Contains(body, api.END):
		// htmx 4 swaps a 422 too.
		render(w, r, http.StatusUnprocessableEntity, Form(body, true))
	default:
		if _, err := api.Post(r.Context(), body); err != nil {
			fail(w, err)
			return
		}
		render(w, r, http.StatusOK, Form("", false))
	}
}

// NotesStream is the page's side of the feed (api.Notes): every new note as the Item fragment, over server-sent
// events, which htmx 4's SSE extension puts at the top of the list.
type NotesStream struct{}

func (NotesStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.Notes.Fragments(w, r, func(ctx context.Context, note api.Note) (string, error) {
		var html bytes.Buffer
		err := Item(note).Render(ctx, &html)
		return strings.TrimSpace(html.String()), err
	})
}

// Cloud is the page of what Cloudflare gives the Worker, one card each (package bindings).
type Bindings struct{}

func (Bindings) Props(r *http.Request) (bindings.State, error) { return bindings.Read(r) }

// BindingsAction is the Cloudflare page's forms (/bindings/note, /bindings/queue, /bindings/email): plain posts, answered with
// a redirect back to the page.
type BindingsAction struct{}

func (BindingsAction) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := bindings.Post(r); err != nil {
		fail(w, err)
		return
	}
	// Back to the page, in the reader's language.
	back, err := app.URL(r.Context(), Bindings{})
	if err != nil {
		fail(w, err)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// render writes a component as the answer: whole, with its length, or a 500 if it fails half-way.
func render(w http.ResponseWriter, r *http.Request, status int, node gsx.Node) {
	var html bytes.Buffer
	if err := node.Render(r.Context(), &html); err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(html.Len()))
	w.WriteHeader(status)
	w.Write(html.Bytes())
}

func fail(w http.ResponseWriter, err error) {
	log.Printf("pages: %v", err)
	http.Error(w, "Something went wrong.", http.StatusInternalServerError)
}
