package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/asyncapi"
	"github.com/joeblew999/charter/go/auth"
	"github.com/joeblew999/charter/go/humaworkers"
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
				huma.Register(api, huma.Operation{OperationID: "listThings", Method: "GET", Path: "/api/things", Security: auth.Public()}, read)
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
	rec := get(t, h, "GET", "https://things.example"+OpenAPIPath)
	if json.Unmarshal(rec.Body.Bytes(), &spec); rec.Code != 200 || len(spec.Servers) != 1 || spec.Servers[0].URL != "https://things.example" || spec.Paths["/api/things"] == nil {
		t.Errorf("the OpenAPI spec: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "GET", "https://things.example"+AsyncAPIPath); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"liveThings"`) || !strings.Contains(rec.Body.String(), `"Things live"`) {
		t.Errorf("the AsyncAPI spec: %d %s", rec.Code, rec.Body)
	}
	req := httptest.NewRequest("POST", "https://things.example"+MCPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
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
