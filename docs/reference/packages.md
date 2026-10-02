---
title: Go packages
nav_order: 3
parent: Reference
---

# Go packages: the library a Go project imports

The packages of the library (`go/`, the module `github.com/joeblew999/charter/go`): what each is for, its exported names, and one minimal use. The doc comments say more: `go doc github.com/joeblew999/charter/go/<name>`.

| Package | What it is |
|---|---|
| [`humaworkers`](#humaworkers) | A [Huma](https://huma.rocks) API on Cloudflare Workers |
| [`transport`](#transport) | What goes around the handler: the request bridge, and WebSockets |
| [`d1`](#d1) | Statements on a D1 database |
| [`hub`](#hub) | The live signal of a feed |
| [`follow`](#follow) | The gap-free feed over a log and a hub |
| [`humamcp`](#humamcp) | A Huma API as an MCP server |
| [`asyncapi`](#asyncapi) | An AsyncAPI document from a Huma API |
| [`specfile`](#specfile) | The body of a spec command |
| [The Worker glue](#the-worker-glue) | The JavaScript in `go/worker/` |

## humaworkers

Huma's own adapter needs `http.ServeMux` method patterns, which TinyGo lacks, and registering every operation at start-up would be paid each time a Go runtime starts. So this package routes itself and registers an operation when a request first matches it.

```go
type Route struct {
	Method, Path string             // Huma's form: /api/notes/{id}
	OperationID  string             // optional: lets Operation find one without registering the others
	Register     func(api huma.API) // a huma.Register call
}

func Config(title, version string) huma.Config // huma.DefaultConfig without what does not run on Workers
func WithForm(config huma.Config) huma.Config  // form-encoded bodies: contentType:"application/x-www-form-urlencoded"
func New(config huma.Config, routes []Route) *API

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) // registers the one route matched; 404, 405
func (a *API) Operation(id string) *huma.Operation              // registers as little as it can
func (a *API) Operations() []*huma.Operation                    // registers all; hidden ones too
func (a *API) OpenAPI() *huma.OpenAPI                           // registers all

const FormContentType = "application/x-www-form-urlencoded"
```

```go
api := humaworkers.New(humaworkers.Config("billing-api", "1.0.0"), []humaworkers.Route{
	{Method: http.MethodGet, Path: "/api/hello", OperationID: "hello", Register: func(api huma.API) {
		huma.Register(api, huma.Operation{OperationID: "hello", Method: http.MethodGet, Path: "/api/hello"}, hello)
	}},
})
```

- **`API` embeds `huma.API`:** `UseMiddleware` and the rest of Huma work as usual.
- **The most specific route wins,** whatever the order: `/things/watch` before `/things/{id}`.
- **422 and 401 are declared by status** where they apply, so Fern's SDKs get a typed error.
- **Uploads stay in memory,** up to 32 MB. A form's values are strings.

## transport

```go
func Run(h http.Handler)
func Serve(h http.Handler) http.Handler

const MessagesHeader = "X-Websocket-Messages"
const MessagesPost = "post"
```

```go
func main() { transport.Run(api.Handler(env())) }
```

| | On Cloudflare (Wasm) | Natively |
|---|---|---|
| `Run` | Serves `h` and never returns, so the Go runtime stays alive for the next request | A plain HTTP server on `:9900`, or `$PORT` |
| `Serve` | Cancels the request's context when the client has gone, and says when the heap has no room for another request | The WebSocket adapter |

What Go answers a WebSocket upgrade with, and what the adapter then does:

| Go answers | The adapter |
|---|---|
| Anything but 2xx | Returns it. No socket opens |
| 200 with a body | Sends each line as one text frame. When the feed ends, closes with 1011 |
| 204 | No feed. The socket stays open |
| The header `X-Websocket-Messages: post` | Gives each text frame from the client to Go as a POST to the same URL, with the upgrade's headers. The answer's lines go back as frames; anything but 2xx closes with 1008 |

On Cloudflare the feed and each message run in Go runtimes of their own: they share nothing through memory.

## d1

A statement is one awaited call, and its rows come over as one string of JSON. Wasm only.

```go
func Open(name string) (DB, error)
func Query[T any](db DB, query string, args ...any) ([]T, error)
```

```go
db, err := d1.Open("DB") // per request: a binding belongs to the request's environment
notes, err := d1.Query[Note](db, "SELECT id, body FROM notes WHERE id < ? LIMIT ?", before, limit)
```

`T` is a struct whose `json` tags name the columns. Rows are those of a SELECT or a RETURNING clause. Arguments are strings, numbers, bools and nil.

## hub

Only a wake-up: the log is the source of truth, and `follow.Follow` fills any gap from it.

```go
type Hub[T any] interface {
	Publish(ctx context.Context, item T) error
	Subscribe(listener func(T), onError func(error)) (unsubscribe func(), err error) // neither may block
}

type Memory[T any] struct{ /* the zero value is ready */ }
func DurableObject[T any](binding, name string) (Hub[T], error) // Wasm only
```

```go
notes, err := hub.DurableObject[Note]("HUB", "notes")       // on Cloudflare, per request
devices, err := hub.DurableObject[Device]("HUB", "devices") // a second feed: the same class, another object
var local hub.Memory[Device]                                // natively and in tests
```

## follow

The one way a client receives a live, ordered, gap-free feed ([How real-time works](../realtime.md)).

```go
func Follow[T Item](ctx context.Context, src Source[T], opt Options, emit func(T) error) error

type Item interface{ Position() int64 }

type Source[T Item] interface {
	Subscribe(listener func(T), onError func(error)) (unsubscribe func(), err error)
	Since(ctx context.Context, after int64, limit int) ([]T, error) // id > after, ascending
	Latest(ctx context.Context) (int64, error)                      // 0 when empty
}

type Options struct {
	After       *int64        // nil: only items newer than now
	Recheck     time.Duration // default 30 s
	Retry       time.Duration // default 1 s
	MaxFailures int           // default 5
	PageSize    int           // default 100
	OnBroken    func(error)
}
```

```go
err := follow.Follow(ctx, source, follow.Options{After: &after}, func(note Note) error {
	_, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", note.ID, data)
	return err
})
```

It returns an error only when it gives up: `MaxFailures` failed subscribes in a row, a failing log, or a failing `emit`. A cancelled context ends it quietly.

## humamcp

A Huma API as MCP tools ([MCP](../guides/mcp.md)).

```go
func Handler(api *humaworkers.API) http.Handler           // mount it on one path
func Expose(op huma.Operation, tool bool) huma.Operation  // overrides IsTool for one operation
func IsTool(op *huma.Operation) bool                      // has an id, is in OpenAPI, does not stream
func Check(api *humaworkers.API) []error                  // why an operation cannot be the tool it would be
func Tools(ops []*huma.Operation, registry huma.Registry) []Tool

var Modern = []string{"2026-07-28"}
var Legacy = []string{"2025-11-25", "2025-06-18"}
```

```go
mcp := humamcp.Handler(routes) // then: case "/api/mcp": mcp.ServeHTTP(w, r)
```

- **No MCP SDK:** the Go ones do not build with TinyGo. It is MCP's Streamable HTTP transport written out: JSON-RPC, one POST per message, one JSON answer.
- **No state,** which is what a Worker needs and what MCP became in revision 2026-07-28. The handshake revisions are answered too.
- **A tool call builds an `http.Request` and runs it through `ServeHTTP`:** one code path with REST.
- **Anything but POST is 405,** a foreign `Origin` is 403, a message is at most 1 MB.

## asyncapi

An AsyncAPI 3.0.0 document from a Huma API, the way Huma generates OpenAPI.

```go
func Operation(op huma.Operation, channel Channel) huma.Operation // a WebSocket channel
func SendOperation(op huma.Operation, send Send) huma.Operation   // a message the client sends on one
func Generate(ops []*huma.Operation, registry huma.Registry, info Info, server string) (map[string]any, error)
func Is(op *huma.Operation) bool

type Channel struct {
	Name        string         // the channel id: Fern names the client after it
	OperationID string         // default: receive<Name>
	Message     string         // default: the payload's schema name
	Payload     any            // a value of the type every message carries
	Extensions  map[string]any // added to the channel object
}
type Send struct {
	Channel     string // the Name of the channel it is sent on
	OperationID string // default: send<Message>
	Message     string // default: the body's schema name
}
type Info struct{ Title, Version string }
```

```go
doc, err := asyncapi.Generate(api.Operations(), api.OpenAPI().Components.Schemas, asyncapi.Info{Title: "notes live", Version: "1.0.0"}, server)
```

A channel's `query` fields become its `bindings.ws.query`; a send operation's JSON body is its payload. Both are hidden from OpenAPI: Fern reads WebSockets from AsyncAPI only.

## specfile

```go
func Main(specs ...func(server string) ([]byte, error)) // spec [-check] <file per spec>... [server-url]
func Must(problems []error)                             // prints them and ends the command if there are any
func Examples(api *humaworkers.API) []error             // what is wrong with the contract's examples
```

```go
func main() { // a project's ./cmd/spec
	specfile.Must(humamcp.Check(humaworkers.New(humaworkers.Config(api.Title, api.Version), api.Routes(api.Env{}))))
	specfile.Main(api.OpenAPI, api.AsyncAPI)
}
```

`Examples` is for a contract's tests: a constrained request field needs an example, and every example must be valid against its schema.

## The Worker glue

JavaScript in `go/worker/`. `mise run build` writes it into a project's `build/`.

| File | What it gives |
|---|---|
| `go.mjs` | `goWorker()`: returns `fetch` (the Worker's) and `warm` |
| `websocket.mjs` | The WebSocket adapter |
| `hub.mjs` | `Hub`: the Durable Object class the `hub` package talks to |
| `tinygo-clock.mjs` | Makes Go timers fire on Cloudflare |

```js
import { goWorker } from "./build/go.mjs";
export { Hub } from "./build/hub.mjs";   // under the name cloudflare.config.ts declares

const go = goWorker();
await go.warm({ runtimes: 2, paths: ["/api/openapi.json", "/api/hello"] });

export default { fetch: go.fetch };
```

- **`runtimes`** (default 2, at most 4 wait): Go runtimes started while the module loads. Each holds its heap.
- **`paths`:** each of them answers a GET of these during start-up. Only paths whose handlers touch no binding.
- **A request with the header `x-go-runtime`** is told which kind of runtime served it: `reused`, `warm` or `new`.
- **The Go side must use `transport.Run`.**
