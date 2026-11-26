package pages

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/bindings"
	"github.com/joeblew999/charter/example/db"
	"github.com/joeblew999/charter/intl/intltest"
)

// The pages, tested as plain Go: a page declares what it needs, so a test hands it a store of its own. No Worker, no
// database, no browser: those are test/'s, against the real thing.

func TestMain(m *testing.M) { os.Exit(intltest.Main(m)) }

// notes is a store in memory.
type notes struct{ all []api.Note }

func (n *notes) Create(_ context.Context, body string) (api.Note, error) {
	note := api.Note{NotesRow: db.NotesRow{ID: int64(len(n.all) + 1), Body: body, CreatedAt: "2026-10-01 12:00:00"}}
	n.all = append(n.all, note)
	return note, nil
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
func (n *notes) Latest(context.Context) (int64, error)                 { return int64(len(n.all)), nil }

// The home page's data is the newest notes first, and where its stream starts: after the newest.
func TestHomeProps(t *testing.T) {
	store := &notes{}
	store.Create(context.Background(), "first")
	store.Create(context.Background(), "second")
	props, err := Home{}.Props(httptest.NewRequest("GET", "/", nil), store)
	if err != nil || len(props.Notes) != 2 || props.Notes[0].Body != "second" || props.After != 2 {
		t.Fatalf("%+v %v", props, err)
	}
}

// The tree as a server, with the test's own store where the Worker has D1.
func serve(t *testing.T, store api.Store) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	_, err := Mount(mux,
		structpages.WithRequestArgs(func(*http.Request) (api.Store, error) { return store, nil }, func(*http.Request) (bindings.State, error) { return bindings.State{}, nil }),
		structpages.WithTargetSelector(structpages.HTMXv4RenderTarget))
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

// The page has its form and its notes, each under the id the tree gives it.
func TestHomePage(t *testing.T) {
	store := &notes{}
	store.Create(context.Background(), "a note <b>as typed</b>")
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
	if err := (Home{}).Item(note).Render(sp.PageContext(context.Background()), &html); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html.String(), `<li id="home-item--7"`) || !strings.Contains(html.String(), "seven") {
		t.Errorf("%s", html.String())
	}
}
