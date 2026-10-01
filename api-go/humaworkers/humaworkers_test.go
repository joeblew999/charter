package humaworkers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

type thingInput struct {
	ID string `path:"id"`
}
type thingOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

// Two routes that count their registrations: a request must register only the one it matches.
func routes(registered map[string]int) []Route {
	route := func(method, path, id string, hidden bool) Route {
		return Route{Method: method, Path: path, Register: func(api huma.API) {
			registered[id]++
			huma.Register(api, huma.Operation{OperationID: id, Method: method, Path: path, Hidden: hidden},
				func(_ context.Context, in *thingInput) (*thingOutput, error) {
					out := &thingOutput{}
					out.Body.ID = id + ":" + in.ID
					return out, nil
				})
		}}
	}
	return []Route{
		route(http.MethodGet, "/things/{id}", "getThing", false),
		route(http.MethodDelete, "/things/{id}", "deleteThing", false),
		route(http.MethodGet, "/hidden/{id}", "hiddenThing", true),
	}
}

func get(api *API, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestARequestRegistersOnlyItsOwnRoute(t *testing.T) {
	registered := map[string]int{}
	api := New(Config("t", "1"), routes(registered))
	for range 2 {
		rec := get(api, "GET", "/things/42")
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":"getThing:42"`) {
			t.Fatalf("HTTP %d %s", rec.Code, rec.Body)
		}
	}
	if registered["getThing"] != 1 || registered["deleteThing"] != 0 || registered["hiddenThing"] != 0 {
		t.Fatalf("registered %v, want getThing once and nothing else", registered)
	}
}

func TestUnknownPathIs404AndUnknownMethodIs405(t *testing.T) {
	api := New(Config("t", "1"), routes(map[string]int{}))
	for path, want := range map[string]int{"/nope": 404, "/things": 404, "/things/": 404, "/things/1/2": 404} {
		if rec := get(api, "GET", path); rec.Code != want {
			t.Errorf("GET %s: HTTP %d, want %d", path, rec.Code, want)
		}
	}
	rec := get(api, "POST", "/things/1")
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, DELETE" {
		t.Errorf("POST /things/1: HTTP %d Allow %q, want 405 GET, DELETE", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestSpecsRegisterEveryRouteAndHiddenOnesStayOutOfOpenAPI(t *testing.T) {
	registered := map[string]int{}
	api := New(Config("t", "1"), routes(registered))
	if n := len(api.Operations()); n != 3 {
		t.Fatalf("%d operations, want 3 (hidden ones too)", n)
	}
	paths := api.OpenAPI().Paths
	if paths["/things/{id}"] == nil || paths["/things/{id}"].Get == nil || paths["/things/{id}"].Delete == nil || paths["/hidden/{id}"] != nil {
		t.Fatalf("OpenAPI paths: %v", paths)
	}
	for id, n := range registered {
		if n != 1 {
			t.Errorf("%s registered %d times", id, n)
		}
	}
}
