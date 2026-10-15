package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/lib/asyncapi"
	"github.com/joeblew999/charter/go/lib/auth"
	"github.com/joeblew999/charter/go/lib/humaworkers"
)

type thing struct {
	Name string `json:"name" example:"a thing"`
}

type thingOutput struct{ Body thing }

// An API with a public read, a write that needs the write scope, and a channel.
func things(trusted ...auth.Trusted) API {
	config := func() huma.Config {
		config := humaworkers.Config("Things", "1.2.3")
		auth.Scheme(config.OpenAPI)
		return config
	}
	read := func(context.Context, *struct{}) (*thingOutput, error) {
		return &thingOutput{Body: thing{Name: "one"}}, nil
	}
	return API{
		Config:    config,
		LiveTitle: "Things live",
		Setting:   func(name string) string { return map[string]string{"WRITE_TOKEN": "w"}[name] },
		Trusted:   trusted,
		Routes: []humaworkers.Route{
			{Method: "GET", Path: "/api/things", OperationID: "listThings", Register: func(api huma.API) {
				huma.Register(api, huma.Operation{OperationID: "listThings", Method: "GET", Path: "/api/things", Security: []map[string][]string{}}, read)
			}},
			{Method: "POST", Path: "/api/things", OperationID: "createThing", Register: func(api huma.API) {
				huma.Register(api, huma.Operation{OperationID: "createThing", Method: "POST", Path: "/api/things", Security: auth.Needs("write")}, read)
			}},
			{Method: "GET", Path: "/api/things/live", OperationID: "liveThings", Register: func(api huma.API) {
				huma.Register(api, asyncapi.Operation(huma.Operation{OperationID: "liveThings", Method: "GET", Path: "/api/things/live", Security: auth.Needs("write")}, asyncapi.Channel{Name: "liveThings", Payload: thing{}}), read)
			}},
		},
	}
}

func get(t *testing.T, h http.Handler, method, target string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Every standard route, from what the project gave.
func TestTheStandardRoutes(t *testing.T) {
	h := things(auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"write"}}).Handler()
	var spec struct {
		Servers []struct{ URL string } `json:"servers"`
		Paths   map[string]any         `json:"paths"`
	}
	rec := get(t, h, "GET", "https://things.example"+openAPIPath)
	if json.Unmarshal(rec.Body.Bytes(), &spec); rec.Code != 200 || len(spec.Servers) != 1 || spec.Servers[0].URL != "https://things.example" || spec.Paths["/api/things"] == nil {
		t.Errorf("the OpenAPI spec: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "GET", "https://things.example"+asyncAPIPath); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"liveThings"`) || !strings.Contains(rec.Body.String(), `"Things live"`) {
		t.Errorf("the AsyncAPI spec: %d %s", rec.Code, rec.Body)
	}
	req := httptest.NewRequest("POST", "https://things.example"+mCPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	mcp := httptest.NewRecorder()
	h.ServeHTTP(mcp, req)
	if mcp.Code != 200 || !strings.Contains(mcp.Body.String(), `"listThings"`) || !strings.Contains(mcp.Body.String(), `"createThing"`) {
		t.Errorf("MCP: %d %s", mcp.Code, mcp.Body)
	}
	if rec := get(t, h, "GET", "https://things.example/.well-known/oauth-protected-resource"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"resource":"https://things.example"`) {
		t.Errorf("the resource's metadata: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "GET", "https://things.example/api/things"); rec.Code != 200 {
		t.Errorf("a public read: %d %s", rec.Code, rec.Body)
	}
}

// Calls are checked by the contract's security, with the challenge; a token comes as ?access_token=
// on a channel only.
func TestCallsAreCheckedAndTheChannelTakesItsTokenInTheQuery(t *testing.T) {
	h := things(auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"write"}}).Handler()
	if rec := get(t, h, "POST", "https://things.example/api/things"); rec.Code != 401 || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "resource_metadata=") {
		t.Errorf("a write without credentials: %d %q", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}
	if rec := get(t, h, "POST", "https://things.example/api/things", "Authorization", "Bearer w"); rec.Code != 200 {
		t.Errorf("a write with the token: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "GET", "https://things.example/api/things/live?access_token=w"); rec.Code != 200 {
		t.Errorf("the channel with ?access_token=: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "POST", "https://things.example/api/things?access_token=w"); rec.Code != 401 {
		t.Errorf("?access_token= on what is no channel: %d, want 401", rec.Code)
	}
}

// An API that takes no credentials checks none, and has no sign-in documents.
func TestAnAPIThatTrustsNoOne(t *testing.T) {
	h := things().Handler()
	if rec := get(t, h, "POST", "https://things.example/api/things"); rec.Code != 200 {
		t.Errorf("a write with no auth at all: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "GET", "https://things.example/.well-known/oauth-protected-resource"); rec.Code != 404 {
		t.Errorf("sign-in documents with no one trusted: %d, want 404", rec.Code)
	}
}

// The refusals the document declares, by status, as problem details: 401 where an operation needs
// credentials, 422 where it takes input to check (Fern types an SDK's errors by status).
func TestTheDocumentDeclaresEveryRefusal(t *testing.T) {
	body, err := things().OpenAPI("https://things.example")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Description string
				Content     map[string]struct {
					Schema struct {
						Ref string `json:"$ref"`
					}
				}
			}
		}
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	statuses := func(path, method string) string {
		var all []string
		for status := range doc.Paths[path][method].Responses {
			all = append(all, status)
		}
		slices.Sort(all)
		return strings.Join(all, " ")
	}
	if got := statuses("/api/things", "get"); got != "200 default" {
		t.Errorf("a public read without input: %s, want 200 default", got)
	}
	if got := statuses("/api/things", "post"); got != "200 401 default" {
		t.Errorf("a write that needs credentials: %s, want 200 401 default", got)
	}
	refused := doc.Paths["/api/things"]["post"].Responses["401"]
	if media, ok := refused.Content["application/problem+json"]; refused.Description != "Unauthorized" || !ok || media.Schema.Ref != "#/components/schemas/ErrorModel" {
		t.Errorf("401: %+v", refused)
	}
}
