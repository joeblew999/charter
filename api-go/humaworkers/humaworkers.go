// Package humaworkers runs a Huma API (github.com/danielgtaylor/huma) on Cloudflare Workers through
// workers-go and TinyGo. It exists for three reasons, each
// measured under workerd (docs/findings.md, "Go on workers-go"):
//
//   - workers-go starts a fresh Go runtime for every request, so registering every operation at
//     start-up would be paid on every request. Here only the operation a request matches is registered.
//     Specs register them all (Operations, OpenAPI).
//   - TinyGo's http.ServeMux has no "GET /path/{id}" patterns, which Huma's own net/http adapter
//     needs. Routes are matched here.
//   - Huma's default config installs a hook that calls reflect.StructOf, which TinyGo does not have.
//     Config leaves it out.
//
// Build with `tinygo build -target wasm -stack-size=256kb`: Huma overflows TinyGo's default stack.
package humaworkers

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// Route is one operation: where it is served, and how to register it with Huma (a huma.Register call).
type Route struct {
	Method   string
	Path     string // Huma's form, e.g. /api/notes/{id}
	Register func(api huma.API)
}

// Upstream: tinygo-org/tinygo#3599 (when fixed: keep huma.DefaultConfig's CreateHooks, the schema links work)
// Config is huma.DefaultConfig without what does not run or is not wanted on Workers: no
// schema-link hook (reflect.StructOf), and no built-in /openapi, /docs and /schemas routes (serve
// the specs yourself, with the request's origin as their server).
func Config(title, version string) huma.Config {
	config := huma.DefaultConfig(title, version)
	config.CreateHooks = nil
	config.OpenAPIPath, config.DocsPath, config.SchemasPath = "", "", ""
	return config
}

// API is a Huma API whose routes are registered when first needed.
type API struct {
	huma.API
	routes   []Route
	done     []bool
	handlers map[string]handler // "GET /api/notes/{id}"
	ops      []*huma.Operation
}

type handler struct {
	op  *huma.Operation
	run func(huma.Context)
}

// New makes the API. Nothing is registered yet.
func New(config huma.Config, routes []Route) *API {
	a := &API{routes: routes, done: make([]bool, len(routes)), handlers: map[string]handler{}}
	a.API = huma.NewAPI(config, adapter{a})
	return a
}

// Operations registers every route and returns all operations, hidden ones too (those are not in
// OpenAPI().Paths; the AsyncAPI generator reads them from here).
func (a *API) Operations() []*huma.Operation {
	for i := range a.routes {
		a.register(i)
	}
	return a.ops
}

// OpenAPI registers every route and returns the document.
func (a *API) OpenAPI() *huma.OpenAPI {
	a.Operations()
	return a.API.OpenAPI()
}

func (a *API) register(i int) {
	if !a.done[i] {
		a.done[i] = true
		a.routes[i].Register(a.API)
	}
}

// ServeHTTP registers the one route the request matches and runs it. An unknown path is 404; a
// known path with another method is 405.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var allow []string
	for i, route := range a.routes {
		values, ok := match(route.Path, r.URL.Path)
		if !ok {
			continue
		}
		if route.Method != r.Method {
			allow = append(allow, route.Method)
			continue
		}
		a.register(i)
		h, ok := a.handlers[route.Method+" "+route.Path]
		if !ok {
			http.Error(w, "route "+route.Method+" "+route.Path+" registered no such operation", http.StatusInternalServerError)
			return
		}
		for name, value := range values {
			r.SetPathValue(name, value)
		}
		h.run(humago.NewContext(h.op, r, w))
		return
	}
	if len(allow) > 0 {
		w.Header().Set("Allow", strings.Join(allow, ", "))
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// match compares a Huma path with a request path, segment by segment; {name} takes one segment.
func match(pattern, path string) (map[string]string, bool) {
	want, got := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(want) != len(got) {
		return nil, false
	}
	var values map[string]string
	for i, segment := range want {
		if len(segment) > 2 && segment[0] == '{' && segment[len(segment)-1] == '}' {
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

// adapter is what Huma registers operations with.
type adapter struct{ api *API }

func (ad adapter) Handle(op *huma.Operation, run func(huma.Context)) {
	ad.api.handlers[op.Method+" "+op.Path] = handler{op, run}
	ad.api.ops = append(ad.api.ops, op)
}

func (ad adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) { ad.api.ServeHTTP(w, r) }
