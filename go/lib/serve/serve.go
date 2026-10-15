// Package serve serves a charter API the standard way (docs/reference/standard.md), so a project
// gives its contract, its handlers and whom it trusts, and wires nothing itself:
//
//   - its operations under /api, each call checked by the contract's security (auth) and limited by
//     its rate limits (ratelimit);
//   - its specs at /api/openapi.json and /api/asyncapi.json, with the request's origin as their server;
//   - its operations as MCP tools at /api/mcp (humamcp);
//   - how to get in, at /.well-known/ (auth.WellKnown);
//   - a WebSocket channel's token as ?access_token=, on the channel only, for a client that cannot send
//     headers (a browser, a Worker).
//
// The same API gives the specs cmd/spec writes (OpenAPI, AsyncAPI) and the MCP check it runs (Tools).
// The platform carries the WebSocket (transport) and runs the handler (transport.Run).
package serve

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/asyncapi"
	"github.com/joeblew999/charter/go/auth"
	"github.com/joeblew999/charter/go/humamcp"
	"github.com/joeblew999/charter/go/humaworkers"
	"github.com/joeblew999/charter/go/ratelimit"
)

// The standard paths (docs/reference/standard.md).
const (
	OpenAPIPath  = "/api/openapi.json"
	AsyncAPIPath = "/api/asyncapi.json"
	MCPPath      = "/api/mcp"
)

// API is what a project gives; the rest is the standard.
type API struct {
	// Config is the document around the operations: its title, version and description, its
	// security schemes (auth.Scheme, ...), its rate limits (ratelimit.Declare), its extensions.
	Config func() huma.Config
	// Routes are the operations, bound to the request's environment (to an empty one, for the specs).
	Routes []humaworkers.Route
	// LiveTitle is the AsyncAPI document's title: the API's channels.
	LiveTitle string
	// Setting looks a secret or variable up (the Worker's environment; natively, os.Getenv).
	Setting func(name string) string
	// Trusted are the ways in the API takes (auth); none, and it checks no credentials.
	Trusted []auth.Trusted
	// Document adds what the document says that no route does (the webhooks), to the API it gives.
	Document func(api *humaworkers.API)
}

// Handler serves the API the standard way.
func (a API) Handler() http.Handler {
	routes := humaworkers.New(a.Config(), a.Routes)
	if len(a.Trusted) > 0 {
		routes.UseMiddleware(auth.Middleware(routes, a.Setting, a.Trusted...))
	}
	// After auth: a limit counts per caller.
	routes.UseMiddleware(ratelimit.Middleware(routes, nil))
	mcp := humamcp.Handler(routes)
	wellKnown := auth.WellKnown(routes, a.Setting, a.Trusted...)
	// The channels' paths: where a token may come as ?access_token=.
	var channels []string
	for _, op := range routes.Operations() {
		if asyncapi.Is(op) && !slices.Contains(channels, op.Path) {
			channels = append(channels, op.Path)
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.URL.Query().Get("access_token"); token != "" && r.Header.Get("Authorization") == "" && slices.Contains(channels, r.URL.Path) {
			r = r.Clone(r.Context())
			r.Header.Set("Authorization", "Bearer "+token)
		}
		path := r.URL.Path
		switch {
		case path == OpenAPIPath:
			a.write(w, a.OpenAPI, Origin(r))
		case path == AsyncAPIPath:
			a.write(w, a.AsyncAPI, Origin(r))
		case path == MCPPath:
			mcp.ServeHTTP(w, r)
		case len(a.Trusted) > 0 && strings.HasPrefix(path, "/.well-known/"):
			// An API that takes credentials says how to get in; one that takes none has nothing to say.
			wellKnown.ServeHTTP(w, r)
		default:
			routes.ServeHTTP(w, r)
		}
	})
}

func (a API) write(w http.ResponseWriter, spec func(server string) ([]byte, error), server string) {
	body, err := spec(server)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// document is the whole API registered, with server as its server, and what Document adds.
func (a API) document(server string) *humaworkers.API {
	config := a.Config()
	if server != "" {
		config.Servers = []*huma.Server{{URL: server}}
	}
	api := humaworkers.New(config, a.Routes)
	if a.Document != nil {
		a.Document(api)
	}
	return api
}

// OpenAPI is the API's OpenAPI spec, with server as its server: what cmd/spec writes for Fern and
// what the Worker serves at /api/openapi.json.
func (a API) OpenAPI(server string) ([]byte, error) {
	return json.Marshal(a.document(server).OpenAPI())
}

// AsyncAPI is the AsyncAPI spec of the API's channels (/api/asyncapi.json).
func (a API) AsyncAPI(server string) ([]byte, error) {
	api := a.document(server)
	doc, err := asyncapi.Generate(api.Operations(), api.OpenAPI().Components.Schemas, asyncapi.Info{Title: a.LiveTitle, Version: api.OpenAPI().Info.Version}, server)
	if err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}

// Tools is what keeps an operation from being the MCP tool it would be (humamcp.Check): cmd/spec
// fails on it before it writes a spec.
func (a API) Tools() []error {
	return humamcp.Check(humaworkers.New(a.Config(), a.Routes))
}

// Origin is a request's scheme and host: absolute in its URL on Workers, from Host natively, https
// when it came over TLS or a proxy says so.
func Origin(r *http.Request) string {
	if r.URL.Scheme != "" && r.URL.Host != "" {
		return r.URL.Scheme + "://" + r.URL.Host
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + r.Host
}
