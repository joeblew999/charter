// Package humaworkers runs a Huma Huma (github.com/danielgtaylor/huma) on Cloudflare Workers:
//
//   - A new Go runtime pays for everything registered at start-up, so only the operation a request matches is
//     registered (ServeHTTP by path, Operation by id). Specs register them all (Operations, OpenAPI).
//   - Routes are matched here: the most specific one wins, whatever their order.
//   - HumaConfig is Huma's default without its built-in /openapi, /docs and /schemas routes: charter/serve serves the
//     specs, with the request's origin as their server.
package api

import (
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

func init() {
	// Workers have no disk, and Huma's adapter keeps only 8 KB of an uploaded file in memory before
	// it writes the rest to a temporary file. Keep whole uploads in memory, up to this size.
	humago.MultipartMaxMemory = 32 << 20
}

// Route is one operation: where it is served, and how to register it with Huma (a huma.Register call).
type Route struct {
	Method string
	Path   string // Huma's form, e.g. /api/notes/{id}
	// OperationID is the id Register gives the operation. Optional: it lets Operation find one
	// operation by id without registering the others.
	OperationID string
	Register    func(api huma.API)
}

// HumaConfig is huma.DefaultConfig without its schema-link hook and its built-in /openapi, /docs and /schemas routes
// (serve the specs yourself, with the request's origin as their server).
func HumaConfig(title, version string) huma.Config {
	config := huma.DefaultConfig(title, version)
	config.CreateHooks = nil
	config.OpenAPIPath, config.DocsPath, config.SchemasPath = "", "", ""
	return config
}

// Huma is a Huma Huma whose routes are registered when first needed.
type Huma struct {
	huma.API
	// registering guards what follows: on Workers a runtime serves one request, but the native build
	// serves many at once, and the first ones register routes.
	registering sync.Mutex
	routes      []Route
	done        []bool
	handlers    map[string]handler  // "GET /api/notes/{id}"
	ops         [][]*huma.Operation // what each route registered
	current     int                 // the route being registered
}

type handler struct {
	op  *huma.Operation
	run func(huma.Context)
}

// NewHuma makes the Huma. Nothing is registered yet.
func NewHuma(config huma.Config, routes []Route) *Huma {
	a := &Huma{routes: routes, done: make([]bool, len(routes)), handlers: map[string]handler{}, ops: make([][]*huma.Operation, len(routes))}
	a.API = huma.NewAPI(config, adapter{a})
	return a
}

// Operations registers every route and returns all operations, hidden ones too (those are not in
// OpenAPI().Paths; the AsyncAPI generator reads them from here). They are in the routes' order,
// whatever requests came before: a process that lives on (the native build) registers routes as
// requests need them.
func (a *Huma) Operations() []*huma.Operation {
	a.registering.Lock()
	defer a.registering.Unlock()
	var all []*huma.Operation
	for i := range a.routes {
		a.register(i)
		all = append(all, a.ops[i]...)
	}
	return all
}

// Operation returns the operation with this id, or nil. It registers as little as it can: the route
// that names the id, or, among routes that name none, one after another until the id turns up.
func (a *Huma) Operation(id string) *huma.Operation {
	a.registering.Lock()
	defer a.registering.Unlock()
	find := func() *huma.Operation {
		for _, ops := range a.ops {
			for _, op := range ops {
				if op.OperationID == id {
					return op
				}
			}
		}
		return nil
	}
	if op := find(); op != nil {
		return op
	}
	for i, route := range a.routes {
		if a.done[i] || (route.OperationID != "" && route.OperationID != id) {
			continue
		}
		a.register(i)
		if op := find(); op != nil {
			return op
		}
	}
	return nil
}

// Spec is the document as far as routes are registered: its settings (security, servers) without
// registering anything, which OpenAPI does.
func (a *Huma) Spec() *huma.OpenAPI { return a.API.OpenAPI() }

// OpenAPI registers every route and returns the document.
func (a *Huma) OpenAPI() *huma.OpenAPI {
	a.Operations()
	return a.API.OpenAPI()
}

// register registers route i, once. The caller holds a.registering.
func (a *Huma) register(i int) {
	if !a.done[i] {
		a.done[i], a.current = true, i
		a.routes[i].Register(a.API)
	}
}

// ServeHTTP registers the one route the request matches and runs it. An unknown path is 404; a
// known path with another method is 405, with the methods it has (Allow, sorted); both as problem
// details. Of several routes that match, the most specific one runs,
// whatever their order: /things/watch before /things/{id} for a request for /things/watch.
func (a *Huma) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var allow []string
	var values map[string]string
	best := -1
	for i, route := range a.routes {
		found, ok := match(route.Path, r.URL.Path)
		if !ok {
			continue
		}
		if route.Method != r.Method {
			allow = append(allow, route.Method)
		} else if best < 0 || moreSpecific(route.Path, a.routes[best].Path) {
			best, values = i, found
		}
	}
	if best < 0 {
		if len(allow) > 0 {
			slices.Sort(allow)
			w.Header().Set("Allow", strings.Join(slices.Compact(allow), ", "))
			WriteProblem(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		WriteProblem(w, http.StatusNotFound, "not found")
		return
	}
	route := a.routes[best]
	a.registering.Lock()
	a.register(best)
	h, ok := a.handlers[route.Method+" "+route.Path]
	a.registering.Unlock()
	if !ok {
		http.Error(w, "route "+route.Method+" "+route.Path+" registered no such operation", http.StatusInternalServerError)
		return
	}
	for name, value := range values {
		r.SetPathValue(name, value)
	}
	h.run(humago.NewContext(h.op, r, w))
}

// moreSpecific reports whether path a is more specific than path b, both matching one request: at
// the first segment where one is literal and the other a {name}, the literal one is.
func moreSpecific(a, b string) bool {
	one, other := strings.Split(a, "/"), strings.Split(b, "/")
	for i := range one {
		if isParam(one[i]) != isParam(other[i]) {
			return isParam(other[i])
		}
	}
	return false
}

func isParam(segment string) bool {
	return len(segment) > 2 && segment[0] == '{' && segment[len(segment)-1] == '}'
}

// match compares a Huma path with a request path, segment by segment; {name} takes one segment.
func match(pattern, path string) (map[string]string, bool) {
	want, got := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(want) != len(got) {
		return nil, false
	}
	var values map[string]string
	for i, segment := range want {
		if isParam(segment) {
			if got[i] == "" {
				return nil, false
			}
			if values == nil {
				values = map[string]string{}
			}
			values[segment[1:len(segment)-1]] = got[i]
		} else if segment != got[i] {
			return nil, false
		}
	}
	return values, true
}

// jsonOnly takes application/octet-stream out of op's request body when it is there only because
// the input has a RawBody []byte next to its typed Body (the request as posted, kept beside the
// validated one). Huma 2.39.1 then documents both content types, though only the Body's is read,
// and Fern's Go generator takes the binary one: the method's request becomes an io.Reader. op is
// the one in the OpenAPI document. An input with a RawBody alone keeps it: it is its only content.
// Upstream: danielgtaylor/huma#1132 (when fixed: delete jsonOnly)
func jsonOnly(op *huma.Operation) {
	const raw = "application/octet-stream"
	if op.RequestBody == nil || len(op.RequestBody.Content) < 2 {
		return
	}
	if media := op.RequestBody.Content[raw]; media != nil && media.Schema != nil && media.Schema.Type == huma.TypeString && media.Schema.Format == "binary" {
		delete(op.RequestBody.Content, raw)
	}
}

// adapter is what Huma registers operations with.
type adapter struct{ api *Huma }

func (ad adapter) Handle(op *huma.Operation, run func(huma.Context)) {
	jsonOnly(op)
	ad.api.handlers[op.Method+" "+op.Path] = handler{op, run}
	ad.api.ops[ad.api.current] = append(ad.api.ops[ad.api.current], op)
}

func (ad adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) { ad.api.ServeHTTP(w, r) }

// ResolveSchema follows a schema that only refers to a registry schema (its $ref) to the one it names; a
// reference to none stays as it is.
func ResolveSchema(schema *huma.Schema, registry huma.Registry) *huma.Schema {
	for schema != nil && schema.Ref != "" {
		target := registry.SchemaFromRef(schema.Ref)
		if target == nil {
			break
		}
		schema = target
	}
	return schema
}
