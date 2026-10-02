---
title: Go packages
nav_order: 3
parent: Reference
---

# Go packages: the library a Go project imports

The packages of the library (`go/`, the module `github.com/joeblew999/charter/go`): what each is for, its exported names, and one minimal use. Each is `github.com/joeblew999/charter/go/<name>`. The doc comments in the code say more; `go doc` prints them.

```sh
go get github.com/joeblew999/charter/go@latest    # in a project that does not have it yet
```

| Package | What it is |
|---|---|
| [`humaworkers`](#humaworkers) | A [Huma](https://huma.rocks) API on Cloudflare Workers: routing, lazy registration, a config that runs under TinyGo |
| [`transport`](#transport) | What goes around the handler: the request bridge on Cloudflare, a plain server natively, WebSockets in both |
| [`d1`](#d1) | Statements on a D1 database, with rows crossing as one JSON string |
| [`hub`](#hub) | The live signal of a feed: publish an item, every subscriber gets it |
| [`follow`](#follow) | The gap-free feed over a log and a hub |
| [`humamcp`](#humamcp) | A Huma API as an MCP server |
| [`asyncapi`](#asyncapi) | An AsyncAPI 3.0.0 document from a Huma API |
| [`specfile`](#specfile) | The body of a spec command, and a check of the contract's examples |
| [The Worker glue](#the-worker-glue) | The JavaScript in `go/worker/` that `transport` and `hub` talk to |

## humaworkers

Runs a Huma API on Workers through workers-go and TinyGo. It exists because Huma's own adapter needs `http.ServeMux` method patterns, which TinyGo lacks, and because registering every operation at start-up would be paid each time a Go runtime starts.

```go
type Route struct {
	Method      string
	Path        string // Huma's form: /api/notes/{id}
	OperationID string // optional: lets Operation find one without registering the others
	Register    func(api huma.API)
}

func Config(title, version string) huma.Config
func WithForm(config huma.Config) huma.Config
func New(config huma.Config, routes []Route) *API

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request)
func (a *API) Operation(id string) *huma.Operation
func (a *API) Operations() []*huma.Operation
func (a *API) OpenAPI() *huma.OpenAPI

const FormContentType = "application/x-www-form-urlencoded"
```

```go
api := humaworkers.New(humaworkers.Config("billing-api", "1.0.0"), []humaworkers.Route{
	{Method: http.MethodGet, Path: "/api/hello", OperationID: "hello", Register: func(api huma.API) {
		huma.Register(api, huma.Operation{OperationID: "hello", Method: http.MethodGet, Path: "/api/hello"}, hello)
	}},
})
```

| Name | What it does |
|---|---|
| `Config` | `huma.DefaultConfig` without what does not run on Workers: no schema-link hook (`reflect.StructOf`), no built-in `/openapi`, `/docs` and `/schemas` routes |
| `WithForm` | Lets operations take form-encoded bodies: tag the input's `Body` with `contentType:"application/x-www-form-urlencoded"`. Its fields must be strings |
| `ServeHTTP` | Registers the one route the request matches and runs it. Unknown path: 404. Known path, other method: 405. The most specific route wins, whatever the order |
| `Operation` | The operation with this id, registering as little as it can |
| `Operations`, `OpenAPI` | Register every route. `Operations` includes hidden ones, which the AsyncAPI generator reads |

Also, for every operation it registers:

- **422 and 401 are declared by status** where they apply, so Fern's SDKs get a typed error. What a handler itself answers with goes in the operation's `Errors`.
- **`application/octet-stream` is taken out** of a request body that has it only because the input keeps a `RawBody []byte` beside a typed `Body`.
- **Uploads stay in memory,** up to 32 MB: a Worker has no disk.

`API` embeds `huma.API`, so `UseMiddleware` and the rest of Huma work as usual.

## transport

What a platform puts around the API's handler.

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
| `Run` | Serves `h` and never returns, so the Go runtime stays alive for the next request. A runtime started while the module loads answers the warm paths first | A plain HTTP server on `:9900`, or `$PORT` |
| `Serve` | Cancels the request's context when the client has gone, and says when this runtime's heap has no room for another request | The WebSocket adapter |

What Go answers a WebSocket upgrade (a GET with `Upgrade: websocket`) with, in both places:

| Go answers | The adapter does |
|---|---|
| Anything but 2xx | Returns it as it is. No socket opens |
| 200 with a body | The feed: each line is sent as one text frame. When it ends, the socket closes with 1011 |
| 204 | No feed. The socket stays open until the client closes it |
| The header `X-Websocket-Messages: post` | Each text frame from the client becomes a POST to the same URL, with the upgrade's headers and the frame as the JSON body. The answer's lines go back as frames. Anything but 2xx closes the socket with 1008. Without the header, what the client sends is ignored |

The limit: on Cloudflare the feed and each message run in Go runtimes of their own, so they share nothing through memory.

## d1

Runs statements on a D1 database from a Go Worker. A statement is one awaited call, and its rows come over as one string of JSON, whatever their number. Wasm only.

```go
func Open(name string) (DB, error)
func Query[T any](db DB, query string, args ...any) ([]T, error)
```

```go
db, err := d1.Open("DB") // per request: a binding belongs to the request's environment
notes, err := d1.Query[Note](db, "SELECT id, body FROM notes WHERE id < ? LIMIT ?", before, limit)
```

- **`T` is a struct whose `json` tags name the columns.** INTEGER and REAL are numbers, TEXT a string, NULL null, a BLOB an array of numbers.
- **Rows** are those of a SELECT or a RETURNING clause; none for any other statement.
- **Arguments** are bound to the `?` in order: strings, numbers, bools and nil.

## hub

The live signal of a feed. It is only a wake-up: the log is the source of truth, and `follow.Follow` fills any gap from it.

```go
type Hub[T any] interface {
	Publish(ctx context.Context, item T) error
	Subscribe(listener func(T), onError func(error)) (unsubscribe func(), err error)
}

type Memory[T any] struct{ /* the zero value is ready */ }
func DurableObject[T any](binding, name string) (Hub[T], error) // Wasm only
```

```go
notes, err := hub.DurableObject[Note]("HUB", "notes")       // on Cloudflare
devices, err := hub.DurableObject[Device]("HUB", "devices") // the same class, another object
var local hub.Memory[Device]                                // natively and in tests
```

One hub carries one type of item. A second feed is a second name on the same binding. Open it per request. Neither callback of `Subscribe` may block.

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

`Follow` emits until `ctx` is cancelled, which ends it quietly. It returns an error only after `MaxFailures` failed subscribes in a row, when the log fails, or when `emit` does.

## humamcp

Serves a Huma API as MCP tools ([MCP](../guides/mcp.md)).

```go
func Handler(api *humaworkers.API) http.Handler
func Expose(op huma.Operation, tool bool) huma.Operation
func IsTool(op *huma.Operation) bool
func Check(api *humaworkers.API) []error
func Tools(ops []*huma.Operation, registry huma.Registry) []Tool

var Modern = []string{"2026-07-28"}
var Legacy = []string{"2025-11-25", "2025-06-18"}
```

```go
mcp := humamcp.Handler(routes) // mount it on one path, e.g. /api/mcp
```

| Name | What it does |
|---|---|
| `Handler` | Serves the operations as tools. Nothing is registered until a request needs it: `tools/call` registers one operation, `tools/list` all |
| `Expose` | Says whether an operation is a tool, whatever `IsTool` would decide |
| `IsTool` | True for an operation with an `OperationID` that is in OpenAPI and does not stream |
| `Check` | Why each operation that would be a tool cannot be one. Call it in the contract's tests and the spec command |

- **No MCP SDK:** the Go ones do not build with TinyGo. It is MCP's Streamable HTTP transport written out: JSON-RPC, one POST per message, one JSON answer.
- **No state:** no session, which is what a Worker needs and what MCP became in revision 2026-07-28. Both that and the handshake revisions are answered.
- **A tool call builds an `http.Request` and runs it through `ServeHTTP`:** one code path with REST.
- **Anything but POST is 405,** a foreign `Origin` is 403, and a message is at most 1 MB.

## asyncapi

Generates an AsyncAPI 3.0.0 document from a Huma API, the way Huma generates OpenAPI.

```go
func Operation(op huma.Operation, channel Channel) huma.Operation
func SendOperation(op huma.Operation, send Send) huma.Operation
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

| Marked with | In the AsyncAPI document |
|---|---|
| `Operation` | A WebSocket channel. The input's `query` fields are its `bindings.ws.query`; `Payload` is the payload of a `receive` operation |
| `SendOperation` | A message the client sends: its JSON body is the payload of a `send` operation |

Both are hidden from OpenAPI: Fern reads WebSockets from AsyncAPI only.

## specfile

The body of a project's spec command (`./cmd/spec`).

```go
func Main(specs ...func(server string) ([]byte, error))
func Must(problems []error)
func Examples(api *humaworkers.API) []error
```

```go
func main() {
	specfile.Must(humamcp.Check(humaworkers.New(humaworkers.Config(api.Title, api.Version), api.Routes(api.Env{}))))
	specfile.Main(api.OpenAPI, api.AsyncAPI)
}
```

| Name | What it does |
|---|---|
| `Main` | Runs `spec [-check] <file per spec>... [server-url]`: writes each spec indented, or with `-check` fails if a file differs |
| `Must` | Prints the problems and ends the command if there are any |
| `Examples` | What is wrong with the contract's examples: a constrained request field without one, or one not valid against its schema. For a contract's tests |

## The Worker glue

JavaScript in `go/worker/`. It ships with the library and is not copied into a project: `mise run build` writes it into `build/`.

| File | What it gives |
|---|---|
| `go.mjs` | `goWorker()`: returns `fetch` (the Worker's) and `warm` |
| `websocket.mjs` | The WebSocket adapter, used by `go.mjs` |
| `hub.mjs` | `Hub`: the Durable Object class the `hub` package publishes to and subscribes from |
| `tinygo-clock.mjs` | Makes Go timers fire on Cloudflare. Imported by `go.mjs` |

A project's entry:

```js
import { goWorker } from "./build/go.mjs";
export { Hub } from "./build/hub.mjs";   // under the name cloudflare.config.ts declares

const go = goWorker();
await go.warm({ paths: ["/api/openapi.json", "/api/hello"] });

export default { fetch: go.fetch };
```

| `warm` option | Meaning | Default |
|---|---|---|
| `runtimes` | How many Go runtimes to start while the module loads. Each holds its heap; at most 4 wait | 2 |
| `paths` | Each runtime answers a GET of these during start-up, into nothing. Only paths whose handlers touch no binding | none |

A request with the header `x-go-runtime` is told in the answer's `x-go-runtime` which kind of runtime served it (`reused`, `warm`, `new`), and why a warm start failed if it did. The Go side must use `transport.Run`.
