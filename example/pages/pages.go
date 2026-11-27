// Package pages is the site's pages, as a tree structpages reads: every route is a field of Pages, with its route in
// the field's tag.
//
//   - A page is a type with a Page component, and Props for its data. Props declares what it needs (the store, the
//     caller) and structpages hands it over: nothing here opens a database or reads a binding.
//   - A region of a page that changes by itself is a component of that page (Home's Form), with an id and a target
//     from the tree ({ Home.Form |> id }, { Home.Form |> target }), never a string.
//   - What a page calls is a type with ServeHTTP that returns an error: it never writes an error or a redirect
//     itself (app.Status, app.Redirect), and answers with a component (structpages.RenderComponent).
//   - Links and forms' targets come from the tree too ({ BuiltIndex{} |> url }, { PostNote{} |> url }), in the
//     reader's language.
//   - What every app has (the language list, the docs' pages, the theme switch) is charter's screens, copied into
//     pages/screens (charter add screen:<name>) and built from this app's own gsxui components.
package pages

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackielii/structpages"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/built"
	"github.com/joeblew999/charter/example/pages/screens"
	"github.com/joeblew999/charter/live"
)

// Pages is the tree.
type Pages struct {
	home  Home         `route:"GET /{$} Home"`
	built Built        `route:"/built Built"`
	docs  screens.Docs `route:"/docs Docs"`

	// The app's own people: signing in with a passkey, and what is someone's own (charter's sign-in screen).
	signIn  screens.SignIn  `route:"/sign-in SignIn"`
	account screens.Account `route:"/account Account"`

	// What an admin sees of the system (charter's admin screen): the app at a glance, its bindings, its database.
	admin screens.Admin `route:"/admin Admin"`

	// The time zone and clock dialog's form, and where it is sent.
	preferences screens.Preferences `route:"/preferences Preferences"`

	// What the pages call: routes, not pages.
	post   PostNote   `route:"POST /notes PostNote"`
	remove RemoveNote `route:"POST /notes/{note}/remove RemoveNote"`
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

// HomeProps is what Home shows: the notes, newest first.
type HomeProps struct {
	Notes []api.Note
	// MayWrite is whether the reader may write a note: the form, or the way to sign in. It is what PostNote
	// needs, asked of the same caller.
	MayWrite bool
	// Caller is the reader: each note's row asks whether they may take it away.
	Caller charter.Caller
	// Notice is the line for everyone (package built: KV keeps it), empty when there is none.
	Notice built.Notice
}

// Open says whether a note's row shows when it was written in full: what the row's own button asks for.
type Open bool

// Removable says whether the reader may take a note away (api.MayRemove): whether its row has the button.
type Removable bool

// Props loads what the request wants, as structpages has it. A row's button asks for that row alone (its target is
// Home's Item, with the note's id as its key): then one note is read, and structpages renders Item with it. The
// list asks for itself when its stream says the notes changed (its target is Home's List): then the newest notes
// are read, and structpages renders List with them, for this reader. Anything else is the page. Each component
// takes its share of the results by their types.
func (p Home) Props(r *http.Request, store api.Store, caller charter.Caller, target structpages.RenderTarget) (HomeProps, []api.Note, charter.Caller, api.Note, Open, Removable, error) {
	if target.Is(p.Item) {
		id, err := strconv.ParseInt(structpages.KeyOf(target), 10, 64)
		if err != nil {
			return HomeProps{}, nil, caller, api.Note{}, false, false, app.NotFound("no such note")
		}
		note, err := store.Get(r.Context(), id)
		if err != nil {
			return HomeProps{}, nil, caller, api.Note{}, false, false, app.NotFound("no such note")
		}
		return HomeProps{}, nil, caller, note, r.URL.Query().Get("open") != "", Removable(api.MayRemove(caller, note)), nil
	}
	notes, err := store.Before(r.Context(), math.MaxInt64, shown)
	if err != nil {
		return HomeProps{}, nil, caller, api.Note{}, false, false, err
	}
	if target.Is(p.List) {
		return HomeProps{}, notes, caller, api.Note{}, false, false, nil
	}
	props := HomeProps{Notes: notes, MayWrite: caller.May(PostNote{}.Needs()...), Caller: caller}
	// One read of KV, for every reader: what it is for.
	if props.Notice, err = built.NoticeNow(r.Context()); err != nil {
		return HomeProps{}, nil, caller, api.Note{}, false, false, err
	}
	return props, notes, caller, api.Note{}, false, false, nil
}

// current is how a link in the header looks: marked when the page being shown is that page, or under it
// (structpages.Within). A page that is renamed fails the render here, instead of never being marked.
func current(ctx context.Context, page any) (string, error) {
	within, err := structpages.Within(ctx, page)
	if within {
		return "secondary", err
	}
	return "ghost", err
}

// PostNote is the form's target: it writes the note as the API's createNote does (api.Post), and answers with the
// page's Form again, empty, or with what was typed, refused. The note itself reaches this page as it reaches every
// other: by the stream.
type PostNote struct{}

// Needs is what writing a note needs, on the page as through the API: the contract's createNote says the same
// (charter.Needs("write")). Whoever is not signed in is sent to sign in.
func (PostNote) Needs() []string { return []string{"write"} }

// The longest note the form takes.
const maxBody = 500

func (PostNote) ServeHTTP(w http.ResponseWriter, r *http.Request, store api.Store, caller charter.Caller) error {
	body := strings.TrimSpace(r.PostFormValue("body"))
	if body == "" || len(body) > maxBody || strings.Contains(body, api.END) {
		// htmx 4 swaps a 422 too: the form comes back with what was typed.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return structpages.RenderComponent(Home{}.Form(body, true))
	}
	if _, err := api.Post(r.Context(), store, caller, body); err != nil {
		return err
	}
	return structpages.RenderComponent(Home{}.Form("", false))
}

// RemoveNote is a row's remove button: it takes the note away if the reader may (api.Remove, the rule the API's
// deleteNote is under too), and answers with nothing, which is what the row becomes.
type RemoveNote struct{}

// Needs is what it needs before the rule is asked: someone who may write.
func (RemoveNote) Needs() []string { return []string{"write"} }

func (RemoveNote) ServeHTTP(w http.ResponseWriter, r *http.Request, store api.Store, caller charter.Caller) error {
	id, err := strconv.ParseInt(r.PathValue("note"), 10, 64)
	if err != nil {
		return app.NotFound("no such note")
	}
	return api.Remove(r.Context(), store, caller, id)
}

// again is a page's own address as the part of it that shows a feed asks it when the feed says something changed
// (live.Again): the same page, in the reader's language, read where the database is newest.
func again(ctx context.Context, page any) (string, error) {
	address, err := structpages.URLFor(ctx, page)
	return live.Again(address), err
}
