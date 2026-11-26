package pages

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackielii/structpages"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/bindings"
	"github.com/joeblew999/charter/example/db"
	"github.com/joeblew999/charter/intl/intltest"
)

// The pages, tested as plain Go: a page declares what it needs, so a test hands it a store of its own. No Worker, no
// database, no browser: those are test/'s, against the real thing.

func TestMain(m *testing.M) { os.Exit(intltest.Main(m)) }

// notes is a store in memory.
type notes struct {
	all  []api.Note
	next int64
}

func (n *notes) Create(_ context.Context, body, authorID, author string) (api.Note, error) {
	n.next++
	note := api.Note{NotesRow: db.NotesRow{ID: n.next, Body: body, AuthorID: authorID, Author: author, CreatedAt: "2026-10-01 12:00:00"}}
	n.all = append(n.all, note)
	return note, nil
}

func (n *notes) Remove(_ context.Context, id int64) error {
	n.all = slices.DeleteFunc(n.all, func(note api.Note) bool { return note.ID == id })
	return nil
}

func (n *notes) Get(_ context.Context, id int64) (api.Note, error) {
	for _, note := range n.all {
		if note.ID == id {
			return note, nil
		}
	}
	return api.Note{}, errors.New("no such note")
}

func (n *notes) Before(_ context.Context, before int64, limit int) (out []api.Note, err error) {
	for i := len(n.all) - 1; i >= 0 && len(out) < limit; i-- {
		if n.all[i].ID < before {
			out = append(out, n.all[i])
		}
	}
	return out, nil
}

func (n *notes) Since(context.Context, int64, int) ([]api.Note, error) { return nil, nil }
func (n *notes) Latest(context.Context) (int64, error)                 { return n.next, nil }

// The home page's data is the newest notes first, and where its stream starts: after the newest.
func TestHomeProps(t *testing.T) {
	store := &notes{}
	store.Create(context.Background(), "first", "u1", "Ada")
	store.Create(context.Background(), "second", "u1", "Ada")
	props, _, _, _, err := Home{}.Props(httptest.NewRequest("GET", "/", nil), store, writer, page{})
	if err != nil || len(props.Notes) != 2 || props.Notes[0].Body != "second" || props.After != 2 {
		t.Fatalf("%+v %v", props, err)
	}
}

// Who a test's request comes from: someone signed in who may write, or nobody.
var (
	writer = charter.Caller{Scheme: charter.SessionScheme, Subject: "u1", Person: "Ada", Scopes: []string{"read", "write"}}
	nobody = charter.Caller{}
)

// as is the app's Auth for a test: every request is this caller's. What a route needs is checked by the same
// rule the Worker's Auth has (Caller.May), so a test of who may do what is a test of the pages' own Needs.
type as charter.Caller

func (c as) Allow(_ *http.Request, needs ...string) error {
	switch who := charter.Caller(c); {
	case !who.SignedIn():
		return app.Status(http.StatusUnauthorized, "sign in to do this")
	case !who.May(needs...):
		return app.Status(http.StatusForbidden, "not allowed")
	}
	return nil
}

// The tree as a server, with the test's own store where the Worker has D1, and the test's own caller where it has
// sessions.
func serve(t *testing.T, store api.Store, who ...charter.Caller) http.Handler {
	t.Helper()
	caller := writer
	if len(who) > 0 {
		caller = who[0]
	}
	mux := http.NewServeMux()
	// The app's own options (its error handler, htmx 4's target selector, its check of what a route needs), with
	// the test's store and caller after them.
	options := append((&app.App{Docs: &app.Docs{}, Auth: as(caller)}).Options(false),
		structpages.WithRequestArgs(func(*http.Request) (api.Store, error) { return store, nil }, func(*http.Request) (bindings.State, error) { return bindings.State{}, nil },
			func(*http.Request) (charter.Caller, error) { return caller, nil }))
	_, err := Mount(mux, options...)
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

// The page has its form and its notes, each under the id the tree gives it.
func TestHomePage(t *testing.T) {
	store := &notes{}
	store.Create(context.Background(), "a note <b>as typed</b>", "u1", "Ada")
	rec := httptest.NewRecorder()
	serve(t, store).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	html := rec.Body.String()
	for _, want := range []string{`<form id="home-form"`, `hx-target="#home-form"`, `hx-post="/notes"`, `<ul id="home-list"`, `id="home-item--1"`, "a note &lt;b&gt;as typed&lt;/b&gt;", "/notes/stream?after=1"} {
		if !strings.Contains(html, want) {
			t.Errorf("the page has no %s", want)
		}
	}
	if rec.Code != 200 {
		t.Errorf("HTTP %d\n%s", rec.Code, html)
	}
}

// The form's target answers with the form alone: empty after a note, or as typed and marked when it is refused.
func TestPostNote(t *testing.T) {
	store := &notes{}
	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/notes", strings.NewReader(url.Values{"body": {body}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		serve(t, store).ServeHTTP(rec, r)
		return rec
	}
	if rec := post("kept"); rec.Code != 200 || len(store.all) != 1 || !strings.Contains(rec.Body.String(), `<form id="home-form"`) || strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("a note: HTTP %d, %d kept\n%s", rec.Code, len(store.all), rec.Body)
	}
	if rec := post("   "); rec.Code != 422 || len(store.all) != 1 || !strings.Contains(rec.Body.String(), `aria-invalid="true"`) {
		t.Errorf("an empty note: HTTP %d, %d kept\n%s", rec.Code, len(store.all), rec.Body)
	}
}

// A note renders the same alone (what the stream sends) as in the list.
func TestItemAlone(t *testing.T) {
	sp, err := structpages.Parse(Pages{}, "/", "charter-example")
	if err != nil {
		t.Fatal(err)
	}
	var html bytes.Buffer
	note := api.Note{NotesRow: db.NotesRow{ID: 7, Body: "seven", CreatedAt: "2026-10-01 12:00:00"}}
	if err := (Home{}).Item(note, false, false).Render(sp.PageContext(context.Background()), &html); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html.String(), `<li id="home-item--7"`) || !strings.Contains(html.String(), "seven") {
		t.Errorf("%s", html.String())
	}
}

// A row's button asks for that row alone: the answer is the one note, open, and nothing of the page around it.
func TestOneRowAlone(t *testing.T) {
	store := &notes{}
	store.Create(context.Background(), "first", "u1", "Ada")
	store.Create(context.Background(), "second", "u1", "Ada")
	r := httptest.NewRequest("GET", "/?open=1", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Target", "li#home-item--1")
	rec := httptest.NewRecorder()
	serve(t, store).ServeHTTP(rec, r)
	html := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(html, `<li id="home-item--1"`) || !strings.Contains(html, "first") || !strings.Contains(html, `aria-expanded="true"`) {
		t.Errorf("HTTP %d\n%s", rec.Code, html)
	}
	if strings.Contains(html, "second") || strings.Contains(html, "<html") {
		t.Errorf("more than the row came back:\n%s", html)
	}
}

// Every route of the tree has an address: a page that the tree cannot resolve (mounted twice, a parameter the test
// does not know) fails here, before a link to it fails in a browser. Parameters are filled with a sample.
func TestEveryRouteHasAnAddress(t *testing.T) {
	// Every route passes a middleware once as the tree is mounted: that is the list.
	var routes []*structpages.PageNode
	sp, err := Mount(http.NewServeMux(), append((&app.App{Docs: &app.Docs{}}).Options(false),
		structpages.WithRequestArgs(func(*http.Request) (api.Store, error) { return &notes{}, nil }, func(*http.Request) (bindings.State, error) { return bindings.State{}, nil },
			func(*http.Request) (charter.Caller, error) { return writer, nil }),
		structpages.WithArgs(charter.NewAuth(nil)),
		structpages.WithMiddlewares(func(next http.Handler, pn *structpages.PageNode) http.Handler {
			routes = append(routes, pn)
			return next
		}))...)
	if err != nil {
		t.Fatal(err)
	}
	sample := map[string]any{"slug": "how-it-is-built", "action": "note", "token": 1, "note": 1, "person": "u1"}
	for _, node := range routes {
		if _, err := sp.URLFor(node.Value.Interface(), sample); err != nil {
			t.Errorf("%s (%s): %v", node.Name, node.FullRoute(), err)
		}
	}
	if len(routes) < 8 {
		t.Errorf("only %d routes found in the tree", len(routes))
	}
}

// page is a request for the whole page: a target that is none of its components.
type page struct{}

func (page) Is(any) bool { return false }

// Who may write is one rule, said once (PostNote's Needs): nobody gets the way to sign in where the form would be,
// and is refused when they post anyway; someone signed in gets the form, and their note is kept.
func TestWritingNeedsSomeoneSignedIn(t *testing.T) {
	store := &notes{}
	get := func(who charter.Caller) string {
		rec := httptest.NewRecorder()
		serve(t, store, who).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		return rec.Body.String()
	}
	post := func(who charter.Caller) int {
		r := httptest.NewRequest("POST", "/notes", strings.NewReader(url.Values{"body": {"a note"}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		serve(t, store, who).ServeHTTP(rec, r)
		return rec.Code
	}
	if html := get(nobody); strings.Contains(html, `id="home-form"`) || !strings.Contains(html, `id="sign-in-to-write"`) {
		t.Errorf("nobody is offered the form, or not the way to sign in:\n%s", html)
	}
	if code := post(nobody); code != http.StatusUnauthorized || len(store.all) != 0 {
		t.Errorf("nobody posting: HTTP %d, %d kept", code, len(store.all))
	}
	reader := charter.Caller{Scheme: charter.SessionScheme, Subject: "u2", Person: "Bo", Scopes: []string{"read"}}
	if code := post(reader); code != http.StatusForbidden || len(store.all) != 0 {
		t.Errorf("someone who may only read, posting: HTTP %d, %d kept", code, len(store.all))
	}
	if html := get(writer); !strings.Contains(html, `id="home-form"`) {
		t.Errorf("someone who may write is not offered the form:\n%s", html)
	}
	if code := post(writer); code != http.StatusOK || len(store.all) != 1 {
		t.Errorf("someone who may write, posting: HTTP %d, %d kept", code, len(store.all))
	}
}

// Who may take a note away is one rule (api.MayRemove): whoever wrote it, and a moderator. The page shows the button
// to exactly those, and the route refuses everyone else.
func TestWhoMayRemoveANote(t *testing.T) {
	store := &notes{}
	note, _ := store.Create(context.Background(), "Ada's note", "u1", "Ada")
	other := charter.Caller{Scheme: charter.SessionScheme, Subject: "u2", Person: "Bo", Role: "member", Scopes: []string{"read", "write"}}
	moderator := charter.Caller{Scheme: charter.SessionScheme, Subject: "u3", Person: "Mo", Role: "moderator", Scopes: []string{"read", "write", "moderate"}}
	button := func(who charter.Caller) bool {
		rec := httptest.NewRecorder()
		serve(t, store, who).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		return strings.Contains(rec.Body.String(), "data-remove")
	}
	remove := func(who charter.Caller) int {
		rec := httptest.NewRecorder()
		serve(t, store, who).ServeHTTP(rec, httptest.NewRequest("POST", "/notes/1/remove", nil))
		return rec.Code
	}
	if button(nobody) || button(other) || !button(writer) || !button(moderator) {
		t.Errorf("the button: nobody %v, another member %v, the writer %v, a moderator %v", button(nobody), button(other), button(writer), button(moderator))
	}
	if code := remove(nobody); code != http.StatusUnauthorized {
		t.Errorf("nobody removing: HTTP %d", code)
	}
	if code := remove(other); code != http.StatusForbidden || len(store.all) != 1 {
		t.Errorf("another member removing: HTTP %d, %d left", code, len(store.all))
	}
	if code := remove(moderator); code != http.StatusOK || len(store.all) != 0 {
		t.Errorf("a moderator removing: HTTP %d, %d left", code, len(store.all))
	}
	note, _ = store.Create(context.Background(), "Ada's second", "u1", "Ada")
	rec := httptest.NewRecorder()
	serve(t, store, writer).ServeHTTP(rec, httptest.NewRequest("POST", "/notes/"+strconv.FormatInt(note.ID, 10)+"/remove", nil))
	if rec.Code != http.StatusOK || len(store.all) != 0 {
		t.Errorf("the writer removing their own: HTTP %d, %d left", rec.Code, len(store.all))
	}
}

// The seed and the tests are one thing: what fills a developer's database (api.Seed, with api.Seeded) fills this
// test's store by the same call, through the app's own Post. So the page is tested with the people and notes a
// developer sees on theirs: Lin, a member, may take away Lin's notes; Grace, a moderator, anyone's; nobody, none.
func TestThePageWithTheSeed(t *testing.T) {
	store := &notes{}
	if err := api.Seed(context.Background(), store, nil); err != nil {
		t.Fatal(err)
	}
	buttons := func(who charter.Caller) int {
		rec := httptest.NewRecorder()
		serve(t, store, who).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		html := rec.Body.String()
		for _, n := range api.Seeded.Notes {
			if !strings.Contains(html, n.By.Name) {
				t.Errorf("the page does not say %s wrote a note", n.By.Name)
			}
		}
		return strings.Count(html, "data-remove")
	}
	lins := 0
	for _, n := range api.Seeded.Notes {
		if n.By.ID == api.Seeded.Lin.ID {
			lins++
		}
	}
	if got := buttons(api.Seeded.Lin.Caller()); got != lins {
		t.Errorf("Lin, a member, has %d remove buttons, want %d: their own notes'", got, lins)
	}
	if got := buttons(api.Seeded.Grace.Caller()); got != len(api.Seeded.Notes) {
		t.Errorf("Grace, a moderator, has %d remove buttons, want %d: every note's", got, len(api.Seeded.Notes))
	}
	if got := buttons(nobody); got != 0 {
		t.Errorf("nobody has %d remove buttons", got)
	}
}
