# api-go/: the same API in Go, on Cloudflare and natively

The notes API of [api.md](api.md), written in Go: [Huma](https://huma.rocks) for the contract, [workers-go](https://github.com/syumai/workers-go) to run on Cloudflare Workers, TinyGo to build the Wasm. From the one Go contract come the handlers' validation, the OpenAPI spec and the AsyncAPI spec; Fern turns those into SDKs, a CLI and docs, exactly as it does for the oRPC Worker (see [sdk.md](sdk.md)).

If the repo's two halves are confusing, read ["What is what"](README.md#what-is-what) first.

## How it fits together

```
api/contract.go  --(mise run api-go:spec)-->  ../sdk/fern/apis/api-go/{openapi,asyncapi}.json  --(Fern)-->  SDKs, CLI, docs
      |
      +--> api/handlers.go serves it: REST and SSE directly, the WebSocket as a stream of lines
                                                                          |
            on Cloudflare: worker/index.mjs turns the lines into frames --+-- natively: platform_other.go does
```

- **The contract is Go structs.** Their tags (`query:"limit" minimum:"1" maximum:"100" default:"20"`) are the schema, the way Zod is for oRPC. Huma validates requests against them and writes the OpenAPI from them.
- **Fern's needs are said in the contract:** `OperationID` and `Tags` name the SDK methods, and `Extensions` carry `x-fern-sdk-*`, `x-fern-pagination` and `x-fern-streaming`.
- **The WebSocket is in the contract too.** `asyncapi.Operation(...)` marks an operation as a channel: it's hidden from OpenAPI and written to AsyncAPI, with its query parameters as the channel's `bindings.ws.query`.

## Layout

| Path | What it is |
|---|---|
| `api/contract.go` | **The API (edit this):** every route, its input and output structs, and what Fern needs |
| `api/handlers.go` | The contract implemented: REST on the store, SSE and the WebSocket feed over `follow` |
| `api/store.go` | The log (`Store`) and the live signal (`Hub`): D1 through `database/sql`, or memory |
| `api/spec.go` | Both specs from the contract, for `cmd/spec` and for `/api/openapi.json`, `/api/asyncapi.json` |
| `cmd/spec/` | Writes the two spec files offline (`mise run api-go:spec`) |
| `follow/` | **Import it.** `Follow`: the gap-free feed, the port of `api/src/follow.ts` with the same tests |
| `asyncapi/` | **Import it.** The AsyncAPI 3.0 generator for Huma, the port of `api/src/asyncapi.ts` |
| `humaworkers/` | **Import it.** What Huma needs to run on workers-go and TinyGo (below) |
| `main.go`, `platform_js.go` | The Worker: bindings (D1, the hub, vars) through workers-go and `syscall/js` |
| `platform_other.go` | The native build: an in-memory store and hub, and the WebSocket transport |
| `worker/index.mjs`, `worker/hub.mjs`, `worker/tinygo-clock.mjs` | What must be JavaScript: the WebSocket transport, the hub Durable Object class (hibernating WebSockets), and the fix that makes Go timers fire on Cloudflare |
| `cloudflare.config.ts` | The Worker `orpc-api-go`: D1 (`DB`), the hub (`HUB`), `APP_NAME` |
| `build/` | Gitignored: the TinyGo Wasm and workers-go's glue (`mise run api-go:build`) |

## Tasks (from the repo root)

```sh
mise run api-go:run            # natively on :5174: no Cloudflare, in-memory store, REST + SSE + WebSocket
mise run api-go:dev            # under workerd on :5174 (TinyGo build, then cf dev), with local D1 and the hub
mise run api-go:migrate:local  # first time, while api-go:dev runs
mise run api-go:build          # after a Go change while api-go:dev runs (cf dev reloads)
mise run api-go:spec           # regenerate the specs after changing the contract
mise run api-go:check          # gofmt, vet, tests, spec drift, TinyGo build, the live test natively and under workerd
mise run sdk:gen api-go go     # Fern: also typescript, typescript-dist, cli
mise run api-go:deploy         # REMOTE: deploy orpc-api-go, then apply D1 migrations
mise run api-go:live-test      # REMOTE: quick live check
mise run api-go:soak           # REMOTE: the real-time matrix (redeploys orpc-api-go)
mise run api-go:bench          # REMOTE, read-only: wall time per route
```

The schema is `../migrations/`: one schema for both Workers, each with its own D1 database. The tests are the shared ones in [testing.md](testing.md).

## Huma on workers-go and TinyGo: what it takes

Huma works, with four things done on our side (in `humaworkers/`, `worker/` and the build task), each measured. The upstream issues behind them are in [upstream.md](upstream.md).

| Problem | What happens without | What we do |
|---|---|---|
| TinyGo's default stack is too small for Huma | `memory access out of bounds` on the first request | Build with `-stack-size=256kb` |
| Huma's default config installs a hook that calls `reflect.StructOf`, which TinyGo lacks | `panic: unimplemented: reflect.StructOf()` | `humaworkers.Config` leaves the hook out (it only adds `$schema` links) |
| TinyGo's `http.ServeMux` doesn't match method patterns (`"GET /path"`), which Huma's net/http adapter registers | every route is 404 | `humaworkers` matches routes itself |
| **Go timers hang on Cloudflare** (not under local workerd): the production clock moves by a `setTimeout`'s delay rounded down to a millisecond, and TinyGo sleeps for fractions | streams don't end at their deadline about half the time; `time.Sleep` and context timeouts stall | `worker/tinygo-clock.mjs` rounds TinyGo's sleeps up to a whole millisecond |

And one design point: **workers-go starts a fresh Go runtime for every request.** Registering every Huma operation at start-up would be paid on every request, so `humaworkers` registers only the operation a request matches. Specs register them all.

Two more things that workers-go can't do, and where they went:

- **Answer a WebSocket upgrade.** Go answers the upgrade with a stream of lines, one JSON message each, and `worker/index.mjs` sends each line as a frame. Which paths are WebSockets, their input and their feed stay in Go.
- **Be a Durable Object.** `worker/hub.mjs` is the hub (hibernating WebSockets, stores nothing). Go publishes to it with workers-go's stub and subscribes with a WebSocket through `syscall/js` (`platform_js.go`).

## Cost

It works and passes the same tests as the oRPC Worker, and it costs more to run: 40 to 70 ms of CPU per request on Cloudflare against 1 to 3 ms, because workers-go starts a fresh Go runtime for every request. That fits Workers Paid, not Workers Free (10 ms). The numbers are in [benchmarks.md](benchmarks.md) and the plan to bring them down in [plans/performance.md](plans/performance.md).

## Differences from the oRPC Worker

Fern sees the same API: the same operations, parameters, constraints, `x-fern-*` extensions and WebSocket channel (`TestSameSurfaceAsTheORPCContract` checks this against `../sdk/fern/apis/api/`). What differs:

- **Invalid input is 422** with Huma's `application/problem+json` body (`errors[].location`, e.g. `query.limit`), where oRPC answers 400. Huma also puts this error in the spec, so SDKs get a typed error.
- **A known path with the wrong method is 405**, where the oRPC Worker answers 404.
- **Schemas are named.** Huma writes `components.schemas.Note` and refers to it; oRPC inlines it. Fern's SDKs then have a `Note` type instead of one type per response.
- **OpenAPI 3.1.0** (Huma) against 3.1.1 (oRPC). Fern accepts both.

## Running natively

`mise run api-go:run` is the same handlers on `net/http`, with an in-memory store and hub in one process. REST, SSE and the WebSocket all work, and `test/live-test.mjs` passes against it. There's no database yet: `api.SQLStore` is plain `database/sql`, so a native SQLite driver is the missing piece.
