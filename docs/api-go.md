---
title: Go Worker internals (api/go/)
nav_order: 2
parent: This repository
---
# api/go/: the notes API in Go, on Cloudflare and natively

The Go Worker: the notes API of [api.md](api.md#the-notes-api), written in Go. [Huma](https://huma.rocks) is the contract, [workers-go](https://github.com/syumai/workers-go) runs it on Cloudflare Workers, and TinyGo builds the Wasm. From the one Go contract come the handlers' validation, both specs (which Fern turns into SDKs, a CLI and docs, as for the oRPC Worker: [sdk.md](sdk.md)) and MCP tools ([mcp.md](mcp.md)). Read this page to run or change the Go Worker, to learn what Huma needs on workers-go, or to start a Go project from it.

`api/go/` is also a Go module that other projects import: the packages marked "Import it" below. A second API lives in the same module, the Go showcase ([showcase-go.md](showcase-go.md)).

## Tasks (from the repo root)

```sh
mise run api:go:run            # natively on :5174 (API_GO_PORT): no Cloudflare, an in-memory store
mise run api:go:dev            # under workerd on :5174: the TinyGo build, then cf dev, with local D1 and the hub
mise run api:go:migrate:local  # first time, while api:go:dev runs: the D1 schema
mise run api:go:build          # the Wasm into api/go/build; run it after a Go change while api:go:dev runs (cf dev reloads)
mise run api:go:spec           # write both specs again, after changing the contract
mise run api:go:check          # LOCAL: lint, test, spec:check, test:native and test:workerd (the five below)
mise run api:go:lint           # gofmt, and go vet for the host and for Wasm
mise run api:go:test           # go test ./... (the whole module, the Go showcase included)
mise run go:check              # the library in go/ (its own module): gofmt, go vet for the host and for Wasm, go test
mise run api:go:spec:check     # fail if a committed spec is stale against the contract
mise run api:go:test:native    # the live test and the MCP test against the native build
mise run api:go:test:workerd   # the same against the TinyGo Wasm under workerd
mise run api:go:mcp-test       # the MCP test against a running api:go:run or api:go:dev
mise run sdk:gen api-go go     # Fern (Docker); the other groups: typescript, typescript-dist, cli
mise run api:go:deploy         # REMOTE: build and deploy orpc-api-go, then apply pending D1 migrations
mise run api:go:migrate        # REMOTE: apply pending migrations to orpc-api-go-db
mise run api:go:live-test      # REMOTE, writes test notes: SSE and WebSocket, raw and through the TypeScript SDK, then MCP
mise run api:go:soak           # REMOTE, redeploys orpc-api-go: the real-time matrix; --idle 20 for the long-idle case
mise run api:go:bench          # REMOTE, read-only: wall time per route (benchmarks.md)
```

The remote tasks use `API_GO_URL`, which defaults to this repo's deployed Worker; on another Cloudflare account set it in `mise.local.toml` (gitignored). The build fails if the Wasm is over 3 MB gzipped, the Workers Free limit. The tests are the shared ones in [testing.md](testing.md).

## Layout

| Path | What it is |
|---|---|
| `api/go/api/contract.go` | **The contract (edit this):** every route, its input and output structs, and what Fern needs |
| `api/go/api/handlers.go` | The contract implemented: REST on the store, SSE and the WebSocket feed over `follow`. It also serves the two spec routes and mounts `/api/mcp` |
| `api/go/api/store.go` | The log (`Store`) and the live signal (`Hub`): D1 through `database/sql` (`SQLStore`), or memory (`MemStore`) |
| `api/go/api/spec.go` | Both specs from the contract, for `api/go/cmd/spec/` and for `/api/openapi.json`, `/api/asyncapi.json` |
| `api/go/cmd/spec/` | Writes the two spec files offline (`mise run api:go:spec`), with `specfile` |
| `go/specfile/` | **Import it.** The body of a spec command: write the generated specs, or check the committed ones (`-check`) |
| `go/follow/` | **Import it.** `Follow`: the gap-free feed, the port of `api/ts/src/follow.ts` with the same tests ([realtime.md](realtime.md)) |
| `go/hub/` | **Import it.** The live signal of a feed, for any item type: the hub Durable Object on Cloudflare (`hub.DurableObject[T]`), memory natively (`hub.Memory[T]`) |
| `go/asyncapi/` | **Import it.** The AsyncAPI 3.0.0 generator for Huma, the port of `api/ts/src/asyncapi.ts` |
| `go/humaworkers/` | **Import it.** What Huma needs to run on workers-go and TinyGo ([below](#huma-on-workers-go-and-tinygo-what-it-takes)), and a format for form-encoded bodies (`WithForm`) |
| `go/humamcp/` | **Import it.** An MCP server from a Huma API: every operation a tool, no MCP SDK ([mcp.md](mcp.md)) |
| `go/transport/` | **Import it.** What goes around the handler so Go can serve WebSockets: natively the adapter itself, on Workers the cancel when the client has gone. Its package comment is the rule both adapters follow |
| `api/go/main.go`, `api/go/platform_js.go` | The Worker: bindings (D1, the hub, variables) through workers-go |
| `api/go/platform_other.go` | The native build: one in-memory store and hub for the process |
| `api/go/worker.mjs`, `go/worker/go.mjs`, `go/worker/websocket.mjs`, `go/worker/hub.mjs`, `go/worker/tinygo-clock.mjs` | What must be JavaScript: the entry, the runner that reuses Go runtimes between requests, the WebSocket adapter, the hub Durable Object class (hibernating WebSockets), and the fix that makes Go timers fire on Cloudflare. The four in `go/worker/` ship with the library: `mise run api:go:build` writes them into `api/go/build/`, where the entry imports them |
| `api/go/showcase/`, `api/go/cmd/showcase/`, `api/go/cmd/showcase-spec/` | The Go showcase: a second API and its server in the same module ([showcase-go.md](showcase-go.md)) |
| `api/go/cloudflare.config.ts` | The Worker `orpc-api-go`: D1 (`DB`), the hub (`HUB`), `APP_NAME` |
| `api/go/build/` | Gitignored: the TinyGo Wasm and workers-go's glue (`mise run api:go:build`) |
| `migrations/` | The D1 schema, shared with the oRPC Worker: each Worker has its own database |

Versions: Huma 2.39.1 and workers-go 0.36.0 (`go/go.mod`, `api/go/go.mod`), TinyGo 0.42.0 (`mise.toml`), Go 1.27.1 (`go.work`).

## How it fits together

```
api/go/api/contract.go  --(mise run api:go:spec)-->  sdk/fern/apis/api-go/{openapi,asyncapi}.json  --(Fern)-->  SDKs, CLI, docs
      |
      +--> api/go/api/handlers.go serves it: REST and SSE directly, the WebSocket as a stream of lines
                                                                          |
     on Cloudflare: go/worker/websocket.mjs turns lines into frames --+-- natively: go/transport/ does
```

- **The contract is Go structs.** Their tags (`query:"limit" minimum:"1" maximum:"100" default:"20"`) are the schema, the way Zod is for oRPC. Huma validates requests against them and writes the OpenAPI from them. A rule the tags can't say goes in the input's `Resolve` method (a note body must not contain the stream terminator).
- **Fern's needs are said in the contract:** `OperationID` and `Tags` name the SDK methods, and `Extensions` carry `x-fern-sdk-*`, `x-fern-pagination` and `x-fern-streaming`.
- **Give every constrained field an example:** `After string` with `query:"after" pattern:"^\\d+$" example:"42"`. Fern shows an example of each request in the SDKs' READMEs and references, and makes up what the contract doesn't give (`"id"` for an id, whatever its pattern), so whoever copies it gets a 422. `TestExamplesAreThereAndValid` (`api/go/api/contract_test.go`) fails when a request field with a pattern, an enum, a bound or a length has no example, or when an example isn't valid against its own schema.
- **Errors are typed by status.** `humaworkers` declares 422 on every operation that has input and 401 on every one that needs credentials, because Huma by itself declares its error only as the `default` response, which Fern doesn't type. A status a handler answers with itself (404, 409) goes in the operation's `Errors`. In the Go SDK a refused request is then `errors.As(err, &refused)` with a `*UnprocessableEntityError`, and `refused.Body.Errors` has each `Location` (checked against the native server on 2026-10-01, fern-go-sdk 1.64.0).
- **The WebSocket is in the contract too.** `asyncapi.Operation(...)` marks an operation as a channel: it's hidden from OpenAPI and written to AsyncAPI, with its query parameters as the channel's `bindings.ws.query`.
- **One contract entry is one `humaworkers.Route`:** its method, path, `OperationID`, and the `huma.Register` call.
- **To keep a request exactly as posted,** give the input a `RawBody []byte` next to its typed `Body`: Huma validates `Body` and the handler stores `RawBody`. The spec still says JSON only, so the SDKs take the typed request ([upstream.md](upstream.md#found-not-filed)).

## Which route a request reaches

The most specific route that matches, whatever the order of `Routes`: at the first path segment where one route has a literal and the other a `{name}`, the literal one wins. So with `/api/notes/watch` and `/api/notes/{id}`, a request for `/api/notes/watch` reaches `watch`, and every other segment is an id. Only routes with the request's method take part, and `TestALiteralSegmentWinsOverAParameterWhateverTheOrder` (`go/humaworkers/`) holds it.

Fern warns that two such paths "conflict". The pair is valid OpenAPI, which gives the literal path precedence, so the warning is expected and the generated SDKs call the right one.

## Huma on workers-go and TinyGo: what it takes

Huma works, with four things done on our side (in `go/humaworkers/`, `go/worker/` and the build task). The upstream issues behind them are in [upstream.md](upstream.md), and the measurements in [findings.md](findings.md).

| Problem | What happens without | What we do |
|---|---|---|
| TinyGo's default stack is too small for Huma | `memory access out of bounds` on the first request | Build with `-stack-size=128kb` (`dev wasm-build` does) |
| Huma's default config installs a hook that calls `reflect.StructOf`, which TinyGo lacks | `panic: unimplemented: reflect.StructOf()` | `humaworkers.Config` leaves the hook out (it only adds `$schema` links). It also switches off Huma's built-in `/openapi`, `/docs` and `/schemas` routes |
| TinyGo's `http.ServeMux` doesn't match method patterns (`"GET /path"`), which Huma's net/http adapter registers | every route is 404 | `humaworkers` matches routes itself |
| **Go timers hang on Cloudflare** (not under local workerd): the production clock moves by a `setTimeout`'s delay rounded down to a millisecond, and TinyGo sleeps for fractions | streams don't end at their deadline about half the time; `time.Sleep` and context timeouts stall | `go/worker/tinygo-clock.mjs` rounds TinyGo's sleeps up to a whole millisecond |

Two more, met by the Go showcase's file upload ([showcase-go.md](showcase-go.md#tinygo-what-it-took)):

- **TinyGo has no `reflect.Value.MethodByName`,** which Huma's typed multipart form (`huma.MultipartFormFiles[T]`) calls. Take the plain `multipart.Form` and declare its schema on the operation.
- **A Worker has no disk,** and Huma's adapter writes an upload over 8 KB to a temporary file. `humaworkers` keeps uploads in memory, up to 32 MB.

One design point: **a Go runtime starts often** (`go/worker/go.mjs` reuses one for the next request, but every new isolate and every request that finds none waiting starts one; workers-go on its own starts one per request). Registering every Huma operation at start-up would be paid each time, so `humaworkers` registers an operation when a request first matches it. Specs and `tools/list` register them all.

Two things workers-go can't do, and where they went:

- **Answer a WebSocket upgrade.** Go answers the upgrade with a stream of lines, one JSON message each, and `go/worker/websocket.mjs` sends each line as a frame (`transport.Serve` does the same natively). Which paths are WebSockets, their input and their feed stay in Go. A channel can also take what the client sends: each frame becomes a `POST` to Go ([showcase-go.md](showcase-go.md#the-websocket-both-ways)). The notes channel doesn't.
- **Be a Durable Object.** `go/worker/hub.mjs` is the hub (hibernating WebSockets, stores nothing). Go publishes to it with workers-go's stub and subscribes with a WebSocket through `syscall/js` (`api/go/platform_js.go`).

## Running natively

`mise run api:go:run` is the same handlers on `net/http`, with an in-memory store and hub in one process. REST, SSE, the WebSocket and MCP all work, and `mise run api:go:test:native` runs the live test and the MCP test against it. There's no database: notes are gone when the process stops. `api.SQLStore` is plain `database/sql`, so a native SQLite driver is the missing piece ([plans/next.md](plans/next.md)).

Started without the task (`go run .` in `api/go/`), it listens on `PORT`, or on 9900.

## Cost

It passes the same tests as the oRPC Worker and costs about the same to run: on Cloudflare a simple read uses under 1 ms of CPU, a database read 1 to 2 ms and a write about 2 ms, against 0, 1 and 1 to 2 ms for the oRPC Worker. That is with the tuned build (`dev wasm-build`), the Worker entry that reuses Go runtimes (`go/worker/go.mjs`), the lean request path in `transport`, and direct D1 and hub calls; TinyGo and workers-go as they come cost 40 to 70 ms for a read. The numbers and how to measure your own are in [benchmarks.md](benchmarks.md); why, in [concepts/workers-go.md](concepts/workers-go.md).

## Differences from the oRPC Worker

Fern sees the same API: the same operations, parameters, constraints, `x-fern-*` extensions and WebSocket channel. `TestSameSurfaceAsTheORPCContract` (`api/go/api/surface_test.go`) checks this against the committed oRPC specs in `sdk/fern/apis/api-ts/`. What differs:

- **Invalid input is 422** with Huma's `application/problem+json` body (`errors[].location`, e.g. `query.limit`), where oRPC answers 400. The spec declares it by status, so the Go SDK gets a typed error ([above](#how-it-fits-together)).
- **A known path with the wrong method is 405,** where the oRPC Worker answers 404.
- **Schemas are named.** Huma writes `components.schemas.Note` and refers to it; oRPC inlines it. Fern's SDKs then have a `Note` type instead of one type per response.
- **OpenAPI 3.1.0** (Huma) against 3.1.1 (oRPC). Fern accepts both.
- **`limit` and `seconds` are `int32` in the contract.** Huma writes `format: int64` for a Go `int`, and Fern's Go SDK then types the parameter `*int64` instead of `*int`.
- **`/api/mcp` exists only here,** and so does the native build.
- **No static page.** The oRPC Worker serves one outside `/api/`.

## Starting a Go (workers-go) project from it

Run `go run github.com/joeblew999/orpc-api/dev@latest new -name <name>` ([dev.md](dev.md#a-new-project-dev-new)). It does the three steps below, and what follows is what you then own:

1. **The contract as Huma operations,** as in `api/go/api/contract.go`: one `humaworkers.Route` per operation, with `asyncapi.Operation(...)` around the WebSocket ones. For OAuth, idempotency, file upload, webhooks or a WebSocket the client also sends on, see the Go showcase ([showcase-go.md](showcase-go.md)).
2. **The packages as imports** (`go get github.com/joeblew999/orpc-api/go`): `humaworkers`, `asyncapi`, `follow`, `hub`, `humamcp`, `transport` and `specfile`. With the module comes its Worker glue, `go/worker/` (the runner, the WebSocket adapter, the hub and the clock fix): the build writes it into `api/go/build/`. Copied, because they are yours to change: `api/go/worker.mjs` (the entry), `api/go/cmd/spec/` and the two `platform_*.go` files.
3. **`sdk/fern/apis/api-go/` as your API's Fern folder,** plus the `api:go:*` and `sdk:*` tasks.

Then have clients follow the client rule in [realtime.md](realtime.md#what-a-client-does).
