// Package pages is the site's pages, as a tree structpages reads: every route is a field of Pages, with its route in
// the field's tag.
//
//   - A page is a type with a Page component, and Props for its data. Props declares what it needs (the store, the
//     bindings' state) and structpages hands it over: nothing here opens a database or reads a binding.
//   - A region of a page that changes by itself is a component of that page (Home's Form), with an id and a target
//     from the tree ({ Home.Form |> id }, { Home.Form |> target }), never a string.
//   - What a page calls is a type with ServeHTTP that returns an error: it never writes an error or a redirect
//     itself (app.Status, app.Redirect), and answers with a component (structpages.RenderComponent).
//   - Links come from the tree too ({ Bindings{} |> url }, { PostNote{} |> action }).
package pages

import (
	"bytes"
	"context"
	"math"
	"net/http"
	"strings"

	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/app/docs"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/bindings"
)

// Pages is the tree.
type Pages struct {
	home     Home       `route:"GET /{$} Home"`
	bindings Bindings   `route:"GET /bindings Bindings"`
	docs     docs.Pages `route:"/docs Docs"`

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

func (Home) Props(r *http.Request, store api.Store) (HomeProps, error) {
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
// page's Form again, empty, or with what was typed, refused. The note itself reaches this page as it reaches every
// other: by the stream.
type PostNote struct{}

// The longest note the form takes.
const maxBody = 500

func (PostNote) ServeHTTP(w http.ResponseWriter, r *http.Request, store api.Store) error {
	body := strings.TrimSpace(r.PostFormValue("body"))
	if body == "" || len(body) > maxBody || strings.Contains(body, api.END) {
		// htmx 4 swaps a 422 too: the form comes back with what was typed.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return structpages.RenderComponent(Home{}.Form(body, true))
	}
	if _, err := api.Post(r.Context(), store, body); err != nil {
		return err
	}
	return structpages.RenderComponent(Home{}.Form("", false))
}

// NotesStream is the page's side of the feed (api.Notes): every new note as Home's Item, over server-sent events,
// which htmx 4's SSE extension puts at the top of the list. A stream writes as it goes, so this is the one handler
// that returns no error.
type NotesStream struct{}

func (NotesStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.Notes.Fragments(w, r, func(ctx context.Context, note api.Note) (string, error) {
		var html bytes.Buffer
		err := Home{}.Item(note).Render(ctx, &html)
		return strings.TrimSpace(html.String()), err
	})
}

// Bindings is the page of what Cloudflare gives the Worker, one card each (package bindings). Their state is the
// app's request arg: reading it uses every binding once.
type Bindings struct{}

func (Bindings) Props(state bindings.State) (bindings.State, error) { return state, nil }

// BindingsAction is the bindings page's forms (/bindings/note, /bindings/queue, /bindings/email): it does what the
// form asks, then sends the reader back to the page, in their language.
type BindingsAction struct{}

func (BindingsAction) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	if err := bindings.Post(r); err != nil {
		return err
	}
	back, err := app.URL(r.Context(), Bindings{})
	if err != nil {
		return err
	}
	return app.Redirect{To: back}
}
