package humaworkers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestAnOperationIsFoundByIDRegisteringAsLittleAsItCan(t *testing.T) {
	// Routes that name their operation: only the one asked for is registered.
	registered := map[string]int{}
	named := routes(registered)
	for i, id := range []string{"getThing", "deleteThing", "hiddenThing"} {
		named[i].OperationID = id
	}
	api := New(Config("t", "1"), named)
	if op := api.Operation("deleteThing"); op == nil || op.Method != http.MethodDelete {
		t.Fatalf("deleteThing: %+v", op)
	}
	if api.Operation("nope") != nil || len(registered) != 1 || registered["deleteThing"] != 1 {
		t.Fatalf("registered %v, want deleteThing once and nothing else", registered)
	}
	// Routes that don't: one after another, until it is found.
	registered = map[string]int{}
	api = New(Config("t", "1"), routes(registered))
	if op := api.Operation("deleteThing"); op == nil || registered["getThing"] != 1 || registered["deleteThing"] != 1 || registered["hiddenThing"] != 0 {
		t.Fatalf("deleteThing: %+v, registered %v", op, registered)
	}
	if api.Operation("nope") != nil || api.Operation("hiddenThing") == nil || registered["hiddenThing"] != 1 || registered["getThing"] != 1 {
		t.Fatalf("registered %v", registered)
	}
}

// A process that lives on registers routes as requests come: the operations stay in the routes' order.
func TestOperationsAreInTheRoutesOrderWhateverWasServedFirst(t *testing.T) {
	api := New(Config("t", "1"), routes(map[string]int{}))
	get(api, "GET", "/hidden/1")
	get(api, "DELETE", "/things/1")
	var ids []string
	for _, op := range api.Operations() {
		ids = append(ids, op.OperationID)
	}
	if strings.Join(ids, " ") != "getThing deleteThing hiddenThing" {
		t.Fatalf("operations %v", ids)
	}
}

// The native build serves requests at once: the first ones may register routes together (run with -race).
func TestRoutesAreRegisteredSafelyByRequestsAtOnce(t *testing.T) {
	api := New(Config("t", "1"), routes(map[string]int{}))
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch i % 3 {
			case 0:
				get(api, "GET", "/things/1")
			case 1:
				get(api, "DELETE", "/things/1")
			default:
				api.Operations()
			}
		}()
	}
	wg.Wait()
	if n := len(api.Operations()); n != 3 {
		t.Fatalf("%d operations, want 3", n)
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

// The reporter's shape: the request as posted, kept next to the validated one.
type rawInput struct {
	ID   string `path:"id"`
	Body struct {
		Name string `json:"name" minLength:"2"`
	}
	RawBody []byte
}

func TestABodyWithARawBodyIsJSONOnlyInTheSpec(t *testing.T) {
	api := New(Config("t", "1"), []Route{
		{Method: http.MethodPost, Path: "/things/{id}", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{OperationID: "both", Method: http.MethodPost, Path: "/things/{id}"},
				func(_ context.Context, in *rawInput) (*thingOutput, error) {
					out := &thingOutput{}
					out.Body.ID = in.Body.Name + " " + string(in.RawBody)
					return out, nil
				})
		}},
		// A RawBody by itself is the operation's body: it stays.
		{Method: http.MethodPut, Path: "/blobs/{id}", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{OperationID: "rawOnly", Method: http.MethodPut, Path: "/blobs/{id}"},
				func(context.Context, *struct{ RawBody []byte }) (*struct{}, error) { return nil, nil })
		}},
	})
	doc := api.OpenAPI()
	content := doc.Paths["/things/{id}"].Post.RequestBody.Content
	if len(content) != 1 || content["application/json"] == nil || content["application/json"].Schema == nil {
		t.Errorf("Body + RawBody: request content %v, want application/json only", content)
	}
	if content := doc.Paths["/blobs/{id}"].Put.RequestBody.Content; len(content) != 1 || content["application/octet-stream"] == nil {
		t.Errorf("RawBody alone: request content %v, want application/octet-stream", content)
	}
	// The handler still gets both: the validated body and the bytes as posted.
	posted := `{ "name" :  "abc" }`
	req := httptest.NewRequest(http.MethodPost, "/things/1", strings.NewReader(posted))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if want, _ := json.Marshal("abc " + posted); rec.Code != 200 || !strings.Contains(rec.Body.String(), string(want)) {
		t.Errorf("HTTP %d %s", rec.Code, rec.Body)
	}
}
