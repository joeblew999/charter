// Package serve serves a charter API the standard way (docs/self/standard.md), so a project
// gives its contract, its handlers and whom it trusts, and wires nothing itself:
//
//   - its operations under /api, each call checked by the contract's security (auth) and limited by
//     its rate limits (ratelimit);
//   - its specs at /api/openapi.json and /api/asyncapi.json, with the request's origin as their server;
//   - its operations as MCP tools at /api/mcp (go/lib/mcp);
//   - how to get in, at /.well-known/ (auth.WellKnown);
//   - a WebSocket channel's token as ?access_token=, on the channel only, for a client that cannot send
//     headers (a browser, a Worker);
//   - every refusal as RFC 9457 problem details (Huma's, and go/lib/problem's), as charter-lib answers.
//
// The same API gives the specs cmd/spec writes (OpenAPI, AsyncAPI) and the MCP check it runs (Tools).
// The platform carries the WebSocket (transport) and runs the handler (transport.Run).
package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/lib/asyncapi"
	"github.com/joeblew999/charter/go/lib/auth"
	"github.com/joeblew999/charter/go/lib/humaworkers"
	"github.com/joeblew999/charter/go/lib/mcp"
	"github.com/joeblew999/charter/go/lib/ratelimit"
	"github.com/joeblew999/charter/go/lib/transport"
)

// notJSON is what a body that does not parse is refused with (400), in Go and in charter-lib alike:
// encoding/json's own words differ from JavaScript's.
const notJSON = "the body is not valid JSON"

func init() {
	// Huma refuses a body that does not parse with its parser's error; the standard has one message.
	standard := huma.NewError
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		if status == http.StatusBadRequest && msg == "validation failed" {
			for _, err := range errs {
				if detail, ok := err.(*huma.ErrorDetail); ok && detail.Location == "body" {
					detail.Message = notJSON
				}
			}
		}
		return standard(status, msg, errs...)
	}
}

// The standard paths (docs/self/standard.md).
const (
	openAPIPath  = "/api/openapi.json"
	asyncAPIPath = "/api/asyncapi.json"
	mCPPath      = "/api/mcp"
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
	// Then the body's media type: one the operation does not take is 415, as in charter-lib.
	routes.UseMiddleware(mediaTypes(routes))
	tools := mcp.Handler(routes)
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
		// A request to a channel's address is the channel's, whatever its method, as in charter-lib: let
		// in or refused, then 426 if it is no upgrade. Only the transport's frames (MessageHeader) are the
		// channel's messages.
		if slices.Contains(channels, path) && r.Method != http.MethodGet && r.Header.Get(transport.MessageHeader) == "" {
			r = r.Clone(r.Context())
			r.Method = http.MethodGet
		}
		switch {
		case path == openAPIPath:
			a.write(w, a.OpenAPI, auth.Origin(r))
		case path == asyncAPIPath:
			a.write(w, a.AsyncAPI, auth.Origin(r))
		case path == mCPPath:
			tools.ServeHTTP(w, r)
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
	standardDocument(api.OpenAPI())
	return api
}

// The reason phrase a refusal is declared with.
var reasons = map[string]string{"400": "Bad Request", "401": "Unauthorized", "403": "Forbidden", "404": "Not Found", "409": "Conflict", "415": "Unsupported Media Type", "422": "Unprocessable Entity", "429": "Too Many Requests"}

// standardDocument writes the document as the standard has it (docs/self/standard.md), as
// charter-lib writes it from oRPC: a body inline, not a schema Huma names after its Go type
// (CreateInputBody); every refusal declared as problem details (401 when the operation needs
// credentials, 422 when it takes input, its 429 when it is limited, any other it declares, and a
// default); `explode` only on a parameter that is an array.
func standardDocument(doc *huma.OpenAPI) {
	schemas := doc.Components.Schemas.Map()
	inlined := map[string]bool{}
	inline := func(schema *huma.Schema) *huma.Schema {
		if schema == nil || !strings.HasPrefix(schema.Ref, "#/components/schemas/") {
			return schema
		}
		name := strings.TrimPrefix(schema.Ref, "#/components/schemas/")
		if def := schemas[name]; def != nil && strings.HasSuffix(name, "Body") {
			inlined[name] = true
			return def
		}
		return schema
	}
	problem := map[string]*huma.MediaType{"application/problem+json": {Schema: &huma.Schema{Ref: "#/components/schemas/ErrorModel"}}}
	for _, item := range doc.Paths {
		for _, op := range []*huma.Operation{item.Get, item.Put, item.Post, item.Delete, item.Patch, item.Head, item.Options, item.Trace} {
			if op == nil {
				continue
			}
			for _, param := range op.Parameters {
				if param.Schema == nil || param.Schema.Type != huma.TypeArray {
					param.Explode = nil
				}
			}
			if op.RequestBody != nil {
				for _, media := range op.RequestBody.Content {
					media.Schema = inline(media.Schema)
				}
			}
			security := op.Security
			if security == nil {
				security = doc.Security
			}
			refusals := map[string]*huma.Response{}
			if slices.ContainsFunc(security, func(requirement map[string][]string) bool { return len(requirement) > 0 }) {
				refusals["401"] = &huma.Response{Description: reasons["401"]}
			}
			if len(op.Parameters) > 0 || op.RequestBody != nil {
				refusals["422"] = &huma.Response{Description: reasons["422"]}
			}
			for status, response := range op.Responses {
				if strings.HasPrefix(status, "2") || strings.HasPrefix(status, "3") {
					for _, media := range response.Content {
						media.Schema = inline(media.Schema)
					}
					continue
				}
				delete(op.Responses, status)
				if len(status) == 3 && strings.HasPrefix(status, "4") {
					if reason := reasons[status]; reason != "" && status != "429" {
						response.Description = reason
					}
					refusals[status] = response
				}
			}
			refusals["default"] = &huma.Response{Description: "Error"}
			for status, response := range refusals {
				response.Content = problem
				op.Responses[status] = response
			}
		}
	}
	for name := range inlined {
		delete(schemas, name)
	}
	// A problem's status is Huma's `int`: 32 bits under TinyGo, so the Worker's own spec would say
	// int32 where the one cmd/spec writes natively says int64. The standard's is int64.
	if model := schemas["ErrorModel"]; model != nil && model.Properties["status"] != nil {
		model.Properties["status"].Format = "int64"
	}
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

// Tools is what keeps an operation from being the MCP tool it would be (mcp.Check): cmd/spec
// fails on it before it writes a spec.
func (a API) Tools() []error {
	return mcp.Check(humaworkers.New(a.Config(), a.Routes))
}

// mediaTypes refuses a body of a media type the operation does not take (415), as charter-lib does:
// Huma takes JSON for any operation, whatever its spec says.
func mediaTypes(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		op, given := ctx.Operation(), ctx.Header("Content-Type")
		if given == "" || op.RequestBody == nil || len(op.RequestBody.Content) == 0 {
			next(ctx)
			return
		}
		media, _, _ := strings.Cut(given, ";")
		if _, ok := op.RequestBody.Content[strings.ToLower(strings.TrimSpace(media))]; ok {
			next(ctx)
			return
		}
		body, _ := io.ReadAll(ctx.BodyReader())
		huma.WriteErr(api, ctx, http.StatusUnsupportedMediaType, "validation failed", &huma.ErrorDetail{Location: "body", Message: "unknown content type: " + given, Value: string(body)})
	}
}
