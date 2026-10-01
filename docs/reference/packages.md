---
title: Go packages
nav_order: 3
parent: Reference
---

# Go packages: what a project imports from orpc-api

The six Go packages a project made by `dev new` imports: what each is for, every exported name with its signature, and a minimal use. Read it when you write or change a contract, a handler or the Worker's entry. The signatures were checked against `go doc` on 2026-10-01.

All six are in one Go module, which the project's `api-go/go.mod` requires:

```sh
cd api-go && go get github.com/joeblew999/orpc-api/api-go@latest   # add or update the module
```

| Package | Import path | What it is for | Under TinyGo |
|---|---|---|---|
| `humaworkers` | `github.com/joeblew999/orpc-api/api-go/humaworkers` | Runs a [Huma](https://huma.rocks) API on Cloudflare Workers | Yes |
| `asyncapi` | `github.com/joeblew999/orpc-api/api-go/asyncapi` | Writes the AsyncAPI spec of the API's WebSocket channels | Yes |
| `follow` | `github.com/joeblew999/orpc-api/api-go/follow` | Gives a client every item of a log once, in order, live | Yes |
| `hub` | `github.com/joeblew999/orpc-api/api-go/hub` | The live signal of a feed: publish an item, every subscriber gets it. A Durable Object on Cloudflare, memory natively | Yes |
| `humamcp` | `github.com/joeblew999/orpc-api/api-go/humamcp` | Serves the API's operations as MCP tools | Yes |
| `transport` | `github.com/joeblew999/orpc-api/api-go/transport` | Lets the Go handler serve WebSockets, on Workers and natively | Yes, with a different body |
| `specfile` | `github.com/joeblew999/orpc-api/api-go/specfile` | The body of the command that writes the spec files | Not used there: it runs on your machine |

"Under TinyGo" means the package is part of the Wasm that `mise run api-go:build` makes and that the checks run under workerd. All of them also build with standard Go.

In the signatures, `huma` is `github.com/danielgtaylor/huma/v2`.

## humaworkers

A Huma API whose operations are registered only when a request needs them. On Workers a Go runtime starts often (in every new isolate, and for a request that finds none waiting), so registering every operation at start-up would be paid again and again ([Go on Cloudflare Workers](../concepts/workers-go.md)). It also matches routes itself and leaves out the one Huma hook that TinyGo cannot run.

| Name | Signature | What it does |
|---|---|---|
| `Config` | `func Config(title, version string) huma.Config` | Huma's default config without what does not run on Workers: no schema-link hook, and no built-in `/openapi`, `/docs` and `/schemas` routes |
| `WithForm` | `func WithForm(config huma.Config) huma.Config` | Lets operations take form-encoded request bodies. Tag the input's `Body` with `contentType:"application/x-www-form-urlencoded"` |
| `FormContentType` | `const FormContentType = "application/x-www-form-urlencoded"` | That media type |
| `Route` | `type Route struct` with `Method string`, `Path string`, `OperationID string`, `Register func(api huma.API)` | One operation: where it is served, and the `huma.Register` call that registers it. `Path` is in Huma's form (`/api/notes/{id}`). `OperationID` is optional: it lets `Operation` find one operation without registering the others |
| `API` | `type API struct` that embeds `huma.API` | The API. Nothing is registered until it is needed |
| `New` | `func New(config huma.Config, routes []Route) *API` | Makes the API |
| `(*API).ServeHTTP` | `func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request)` | Registers the one route the request matches and runs it. An unknown path is 404, a known path with another method is 405 |
| `(*API).Operation` | `func (a *API) Operation(id string) *huma.Operation` | The operation with this id, or nil, registering as little as it can |
| `(*API).Operations` | `func (a *API) Operations() []*huma.Operation` | Registers every route and returns every operation, hidden ones too, in the routes' order |
| `(*API).OpenAPI` | `func (a *API) OpenAPI() *huma.OpenAPI` | Registers every route and returns the OpenAPI document |

A minimal use, as in `api-go/api/contract.go` and `api-go/api/handlers.go`:

```go
// One Route per operation of the contract.
func Routes(env Env) []humaworkers.Route {
	return []humaworkers.Route{
		{Method: http.MethodGet, Path: "/api/hello", OperationID: "hello", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "hello", Method: http.MethodGet, Path: "/api/hello",
				Summary: "Say hello", Tags: []string{"meta"},
			}, env.hello)
		}},
	}
}

// The handler: *humaworkers.API is an http.Handler.
routes := humaworkers.New(humaworkers.Config(Title, Version), Routes(env))
```

Limits:

- **Build with `-stack-size=128kb` or more.** Huma overflows TinyGo's default stack. `mise run api-go:build` passes 128 KB.
- **Form values are strings.** With `WithForm`, the fields of the `Body` must be strings, or lists of strings for a repeated name.
- **Uploads stay in memory, up to 32 MB.** A Worker has no disk. Importing the package sets Huma's limit (`humago.MultipartMaxMemory`).
- **Huma's typed multipart form does not run under TinyGo.** Take the plain `multipart.Form` and declare its schema on the operation, as `api-go/showcase/contract.go` does in the orpc-api repo.

## asyncapi

Generates an AsyncAPI 3.0.0 document from Huma operations, the way Huma generates OpenAPI. An operation marked as a channel is a WebSocket: it is hidden from OpenAPI and written to AsyncAPI, because Fern reads WebSockets from AsyncAPI only.

| Name | Signature | What it does |
|---|---|---|
| `Operation` | `func Operation(op huma.Operation, channel Channel) huma.Operation` | Marks `op` as a channel. Its `Path` is the channel's address, its `query` parameters the channel's query parameters |
| `SendOperation` | `func SendOperation(op huma.Operation, send Send) huma.Operation` | Marks `op` as a message the client sends on a channel. Its JSON request body is the message's payload |
| `Is` | `func Is(op *huma.Operation) bool` | Reports whether `op` is a channel |
| `Generate` | `func Generate(ops []*huma.Operation, registry huma.Registry, info Info, server string) (map[string]any, error)` | Builds the document for the channels among `ops`. `server` is the API's URL: it gives the host, and `ws` or `wss` from its scheme |
| `Channel` | `type Channel struct` with `Name string`, `OperationID string`, `Message string`, `Payload any`, `Extensions map[string]any` | One channel. `Name` is the channel id, which Fern names the client after. `Payload` is a value of the type every message carries. `OperationID` defaults to `receive<Name>`, `Message` to the payload's schema name |
| `Send` | `type Send struct` with `Channel string`, `OperationID string`, `Message string` | A message the client sends. `Channel` is the `Name` of its channel. `OperationID` defaults to `send<Message>`, `Message` to the body's schema name |
| `Info` | `type Info struct` with `Title string`, `Version string` | The document's info object |

A minimal use, as in `api-go/api/contract.go` (the channel) and `api-go/api/spec.go` (the document):

```go
// In the contract: the WebSocket operation, marked as a channel.
huma.Register(api, asyncapi.Operation(huma.Operation{
	OperationID: "liveNotes", Method: http.MethodGet, Path: "/api/notes/live",
	Summary: "New notes over a WebSocket, as plain JSON",
}, asyncapi.Channel{Name: "liveNotes", OperationID: "receiveNote", Message: "Note", Payload: Note{}}), env.live)

// The spec: every operation, the schemas Huma collected, and the server's URL.
api := humaworkers.New(config(), Routes(Env{}))
info := asyncapi.Info{Title: LiveTitle, Version: Version}
doc, err := asyncapi.Generate(api.Operations(), api.OpenAPI().Components.Schemas, info, server)
```

Limits:

- **A channel that takes messages needs one operation per message.** A Huma operation takes one request, so each message the client sends is an operation of its own (`SendOperation`), on the channel's path with `POST`.
- **Only what is described above is written:** channels, their query parameters, one `receive` operation per channel and its `send` operations.

## follow

The one way a client receives a live, ordered feed without gaps. The log (the database) is the source of truth and an item's id is its only position. The live source (the hub) is only a wake-up signal: it may drop, restart or deliver out of order, and `Follow` reads the log to fill what it missed. Both the SSE stream and the WebSocket are thin adapters over it. The design is in [How real-time works](../realtime.md).

| Name | Signature | What it does |
|---|---|---|
| `Follow` | `func Follow[T Item](ctx context.Context, src Source[T], opt Options, emit func(T) error) error` | Emits the feed until `ctx` is cancelled. It returns an error only after `MaxFailures` failed subscribes in a row, when the log fails, or when `emit` does. A cancelled context ends it with nil |
| `Item` | `type Item interface` with `Position() int64` | Anything with a position in the log |
| `Source` | `type Source[T Item] interface` | The log plus the live signal: the three methods below |
| `Source.Subscribe` | `Subscribe(listener func(T), onError func(error)) (unsubscribe func(), err error)` | Subscribes to live items. `onError` is called when the subscription breaks. Neither callback may block |
| `Source.Since` | `Since(ctx context.Context, after int64, limit int) ([]T, error)` | Items with id greater than `after`, ascending, at most `limit` |
| `Source.Latest` | `Latest(ctx context.Context) (int64, error)` | The newest id in the log, 0 when it is empty |
| `Options` | `type Options struct` | Tunes `Follow`. The zero value is the production setting |

The fields of `Options`:

| Field | Type | Default | What it is |
|---|---|---|---|
| `After` | `*int64` | nil | The resume position: emit items with id greater than this. Nil: only items newer than now |
| `Recheck` | `time.Duration` | 30 s | Read the log when nothing has arrived for this long |
| `Retry` | `time.Duration` | 1 s | The wait between resubscribes after the live source fails |
| `MaxFailures` | `int` | 5 | Failed subscribes in a row before `Follow` gives up |
| `PageSize` | `int` | 100 | Items per read of the log |
| `OnBroken` | `func(error)` | none | Called when the live subscription breaks, before resubscribing: for logs |

A minimal use, shortened from the WebSocket handler in `api-go/api/handlers.go` (`notes` is a `Source` made of the store and the hub; `Note` has a `Position` method):

```go
options := follow.Options{After: &after}   // after: the id of the last note the client received
err := follow.Follow(ctx, notes, options, func(note Note) error {
	data, err := json.Marshal(note)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
})
```

Limits:

- **The log must make a visible id imply all lower ones.** A single SQLite writer (D1) does. `Follow` relies on it to restore order.
- **A hub that closes silently costs up to `Recheck` of delay,** never a lost item.

## hub

The live signal of a feed. A handler that creates an item publishes it; every open stream that subscribed is woken. It is only a signal: `follow.Follow` fills any gap from the log, so a hub may drop a message or restart.

```go
type Hub[T any] interface {
	Publish(ctx context.Context, item T) error
	Subscribe(listener func(T), onError func(error)) (unsubscribe func(), err error)
}

func DurableObject[T any](binding, name string) (Hub[T], error)   // on Cloudflare only
type Memory[T any] struct{ /* ... */ }                             // natively and in tests; the zero value is ready
```

- **`DurableObject`** is the object called `name` of the Durable Object namespace bound as `binding` in `cloudflare.config.ts`. The class is the example's `api-go/worker/hub.mjs`, which sends each published body to every subscriber and knows nothing of the type. Open it per request.
- **One class serves every feed.** Each name is its own object with its own subscribers, so a second feed needs no second class and no second binding:

```go
notes, err := hub.DurableObject[Note]("HUB", "notes")
devices, err := hub.DurableObject[Device]("HUB", "devices")
```

- **`Memory`** is the same in one process. The example's `MemStore` embeds `hub.Memory[Note]`.

Limits:

- **`DurableObject` exists only in the Wasm build** (`js && wasm`). Call it from a file with that build tag, as `api-go/platform_js.go` does.
- **Items travel as JSON,** so `T` must marshal and unmarshal to itself.
- **Checked with one feed.** The notes feed runs through it under workerd and on Cloudflare. Two names on one binding were not run there; `Memory` is tested with two feeds.

## humamcp

Serves a Huma API as an MCP server (Model Context Protocol). Every operation is a tool, its input struct is the tool's input schema, and a tool call runs the operation's own HTTP handler, so MCP gets the same validation, errors and code as REST. It is MCP's Streamable HTTP transport written by hand, with no MCP SDK, and it keeps no state.

| Name | Signature | What it does |
|---|---|---|
| `Handler` | `func Handler(api *humaworkers.API) http.Handler` | Serves the API's operations as MCP tools. Mount it on one path beside the API |
| `IsTool` | `func IsTool(op *huma.Operation) bool` | Reports whether `op` is a tool. By default: every operation that has an `OperationID`, is not hidden from OpenAPI and does not stream |
| `Expose` | `func Expose(op huma.Operation, tool bool) huma.Operation` | Says whether `op` is a tool, whatever `IsTool` would decide. A streaming operation made a tool must end by itself |
| `Tools` | `func Tools(ops []*huma.Operation, registry huma.Registry) ([]Tool, error)` | The tools among `ops`, in their order |
| `Tool` | `type Tool struct` with `Name string`, `Description string`, `InputSchema map[string]any`, `OutputSchema map[string]any`, `Annotations map[string]any` | One entry of `tools/list` |
| `Modern` | `var Modern = []string{"2026-07-28"}` | The protocol revisions served that name their version on every request |
| `Legacy` | `var Legacy = []string{"2025-11-25", "2025-06-18"}` | The revisions served that open with `initialize` |

A minimal use, as in `api-go/api/handlers.go`:

```go
routes := humaworkers.New(config(), Routes(env))
mcp := humamcp.Handler(routes)
// In the handler: requests to /api/mcp go to mcp, the rest to routes.
```

How an operation becomes a tool: the name is its `OperationID`, the description its `Summary` then its `Description`. Path, query, header and cookie parameters and the properties of a JSON object body are the tool's arguments, side by side. `GET` operations are annotated read-only.

Limits:

- **Tools only.** No resources, prompts, progress notifications or list-changed subscriptions.
- **No stream.** One `POST` per message, one JSON answer. `GET` is 405.
- **No authorization of its own.** The `Authorization` header is passed on to the operation.
- **Streaming operations and WebSocket channels are not tools** unless you `Expose` them.

## transport

What goes around the handler so that Go can serve WebSockets. In both builds Go answers the upgrade request with plain HTTP, and an adapter carries the answer over the socket: `api-go/worker/websocket.mjs` on Cloudflare, `Serve` natively. Which paths are WebSockets, their input and what they send stay in the Go handler, and so in the contract.

| Name | Signature | What it does |
|---|---|---|
| `Run` | `func Run(h http.Handler)` | Serves the handler and never returns: `Serve` around it, then workers-go. On Workers the Go runtime stays alive after a response, so `api-go/worker/go.mjs` can give it the next request. Natively it is a plain HTTP server on `:9900` or `$PORT` |
| `Serve` | `func Serve(h http.Handler) http.Handler` | Natively: the WebSocket adapter. Under TinyGo for Workers: it cancels the request's context when the client has gone, and tells `go.mjs` when the runtime's heap has no room for another request; the WebSocket adapter is the JavaScript file |
| `MessagesHeader` | `const MessagesHeader = "X-Websocket-Messages"` | The header that, on the answer to an upgrade, says how the channel takes what the client sends |
| `MessagesPost` | `const MessagesPost = "post"` | Its one value: every text frame from the client becomes a `POST` to the upgrade's URL |

What the handler answers a WebSocket upgrade (a `GET` with `Upgrade: websocket`) with:

| Answer | What the adapter does |
|---|---|
| Anything but 2xx (401, 422, 426) | Returns it as it is. No socket opens |
| 200 with a body | The body is the feed, a stream of lines. Each line is sent as one text frame; empty lines are padding. When the feed ends, the socket is closed with 1011 |
| 204 | There is no feed. The socket stays open until the client closes it |
| The header `X-Websocket-Messages: post` | The channel takes messages. Each text frame from the client is given to Go as a `POST` to the same URL, with the upgrade's headers and the frame as the JSON body. The lines of the answer are sent as frames. One message at a time, in order. Anything but 2xx closes the socket with 1008. Without the header, what the client sends is ignored |

A minimal use, the whole of `main` in `api-go/main.go`:

```go
func main() {
	transport.Run(api.Handler(env()))
}
```

Limits:

- **On Cloudflare the feed and each message run in Go runtimes of their own.** A message cannot change what the feed sends through memory.
- **`Run` needs the project's Worker entry.** With workers-go's own entry (`build/worker.mjs`) it works, one runtime per request as before, and gains nothing. What they share goes through a binding: a Durable Object or a database.
- **Text frames only.** Natively a binary frame on a channel that takes messages closes the socket.
- **A line is at most 1 MB natively.**

## specfile

The body of the command that writes the specs a contract generates into files, for Fern, or checks that the committed files are what the contract gives.

| Name | Signature | What it does |
|---|---|---|
| `Main` | `func Main(specs ...func(server string) ([]byte, error))` | Runs the command. Each function gives one spec for a server URL, as compact JSON. `Main` writes each to the file named for it, indented. It exits with 1 on a failure and 2 on wrong usage |

The command `Main` makes:

```sh
cd api-go && go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]   # one file per spec function, in order
```

| Argument | Default | What it is |
|---|---|---|
| `-check` | off | Write nothing, and fail if a file differs from what its function gives |
| a file per spec function | required | Where each spec is written |
| `server-url` | `https://api.example.com` | The server the specs name |

A minimal use, the whole of `api-go/cmd/spec/main.go`:

```go
func main() {
	specfile.Main(api.OpenAPI, api.AsyncAPI)
}
```

`mise run api-go:spec` and `mise run api-go:spec:check` run it with the project's files and `API_GO_URL`.

## Versions

The packages are released together, under one version ([Releases](releases.md)). They are built against the Huma and workers-go versions in the module's `go.mod`, which your project's `api-go/go.mod` then also uses.
