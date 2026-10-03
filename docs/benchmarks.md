---
title: Performance
nav_order: 9
parent: Guides
---
# Performance: what a request costs, how to measure it, how to try a change

What one request costs on the TypeScript ([oRPC](https://orpc.dev)) notes Worker and on the Go ([Huma](https://huma.rocks)) one, how to measure your own API on Cloudflare, how to try a change in about a minute and a half without touching the deployed Worker, and what made the Go build as cheap as it is. This is the only page with the numbers: other pages link here. What is left to do: [the plan](plans/next.md#make-the-go-worker-cheaper).

CPU time is what Workers bills and limits: Cloudflare's own figure, from Workers Logs (`$workers.cpuTimeMs`). The figures below were measured on Cloudflare on 2026-10-01 and 2026-10-02, with the two Workers serving the same API from the same kind of D1 database, partly before the repository's rename (the Workers were then called `orpc-api` and `orpc-api-go`).

## What a request costs, side by side

`mise run compare` (at the repo root) runs the same bench against both Workers. CPU per request in ms, 2026-10-02, 30 requests each, as Cloudflare logged them, on the Workers deployed now (`charter-notes-ts`, `charter-notes-go`):

```
                      TypeScript                   Go
GET /api/hello        0 0 0 0 0 0 0 0 0 0 0 0 0    0 0 0 0 0 0 0 0 0 0 0 0 2
GET /api/notes (D1)   0 0 1 1 1 0 1 1 1 3 0 0 1    1 1 1 1 1 1 1 3 1 1 1 1 1
POST /api/notes       1 1 2 1 1 1 1 1 1 1 1 1 0    2 1 1 1 2 3 2 2 2 1 3 1 1
a path not there      no figure                    0 0 0 0 0 0 0 0 0 0 0 0 0
```

| Operation | TypeScript | Go |
|---|---|---|
| hello | 0 ms | 0 ms |
| list (a D1 read) | 0 to 1 ms | 1 ms |
| create (a D1 write and a hub publish) | 1 ms | 1 to 2 ms |

- **A new isolate costs more at first:** about 10 ms for its first request ([below](#a-new-isolate)).
- **A newly created Worker reads about 1 ms higher for its first hours.** `charter-notes-go` measured 1 / 3 / 4 ms (hello, list, create) right after it was first deployed, and the figures above a few hours later, with the same code.
- **A stream costs CPU for as long as it is open:** 40 to 150 ms over the life of a 15 or 60 second SSE stream.
- **Workers Free allows 10 ms of CPU per request.**

## Measure your API

You need a deployed Worker and its `API_URL` ([Deploy and test](guides/deploy.md)). For CPU figures: `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment, and observability enabled in `cloudflare.config.ts` (the example has it). A bench takes about half a minute: the requests a few seconds, the rest waiting for Cloudflare to have their CPU figures (up to two minutes).

| Task | What it does | Touches |
|---|---|---|
| `mise run bench` | Times every GET operation of the deployed Worker once it is warm: wall time, CPU median and p99, and the CPU of each request | Read-only |
| `mise run bench -- -write` | The same with the operations that change data | Writes test data |
| `mise run perf` | Deploys, then benches the new isolate from its first request: 8 at once, then each operation. About two minutes | Redeploys your Worker |
| `mise run perf:try -- -name <experiment>` | Builds, deploys to a scratch Worker with a database and hub of its own, benches it, deletes it. About a minute and a half | Nothing of yours |
| `mise run perf:clean` | Deletes scratch Workers and databases a run left | Scratch only |

`bench` reads the operations from the spec the server gives at `/api/openapi.json` and their inputs from the spec's examples, so it measures your operations, not the notes. It skips streams, and operations without an example for a required input. Every flag: [The charter command](reference/charter.md#bench). Pass them after `--`: `mise run bench -- -write -n 30`. For any server, `charter bench <url>`: a local one gives wall time only.

### Read each request, not the median

`bench -each` and `perf` print the CPU of every request in the order sent, and which kind of Go runtime served it. No letter: one that had served before. `w`: one started ahead, while the Worker's module loaded. `n`: one the request had to start. `?`: Cloudflare has no figure yet. A median hides what this shows: the request that starts a runtime, the first use of an operation, the first request in a new isolate ([an example](#a-new-isolate)). Every finding on this page came from a pattern in that line.

## Try a change

```sh
mise run perf:try -- -name base                         # a baseline, from the same hour
mise run perf:try -- -name heap4 -build '-heap 4'       # the same code with other build flags
mise run perf:try -- -name myidea -keep                 # leave the scratch Worker; perf:clean removes it later
mise run perf:try -- -name tinygodev -prebuilt          # deploy the build/ that is there, made by another TinyGo
```

- **Compare against a baseline from the same hour.** Run it once before your change.
- **The bench it runs** is `-each -burst 8 -write`. Other bench flags go after a second `--` (`charter perf -name x -- <bench flags>`).
- **`-name`** is lower-case letters, digits and hyphens, at most 22. The scratch Worker is `<worker>-perf-<name>`.
- **Several can run at once,** by several people: each name is its own Worker and database.
- **It needs** `cloudflare.config.ts` to name the Worker after the mode when the mode starts with `perf-`. The example's does.
- **`-build`** takes the flags of `charter wasm-build`: `-heap`, `-stack`, `-opt`, `-plain` ([The charter command](reference/charter.md#wasm-build)).
- **Then prove it is still correct:** `mise run check`, and `mise run soak` for anything near a stream.

## What moves the cost

What you control in your own project:

1. **Cross between Go and JavaScript as few times as possible.** Every value that crosses costs, and so does every promise Go waits for. Read rows with the library's `d1` package, not `database/sql`; publish to the hub once.
2. **Warm what a new isolate needs.** `go.warm({ paths: [...] })` in `worker.mjs` has each waiting runtime answer those GET paths during start-up. Only paths whose handlers touch no binding.
3. **Start more runtimes ahead if bursts hit new isolates:** `go.warm({ runtimes: 4, paths })`. Each holds its heap and adds to the isolate's start-up time ([the options](reference/packages.md#the-worker-glue)).
4. **Do less before `main` serves.** Package initialisers run each time a runtime starts.

## A new isolate

Cloudflare starts an isolate after a deploy, when a Worker has been idle, and when traffic spreads. The Wasm is not optimised there yet, and Go has to start. `mise run perf` in `examples/notes-go/` deploys, sends 8 requests at once, then each operation in turn (2026-10-02, CPU in ms):

```
GET /api/hello, 8 at once     3 6n 5w 74n 13w 110n 83n 101n
GET /api/hello                2 2 2 3 2 3 2 2 2 3 3 2 4 2 3 6 3 2 2 2 1 3 2
GET /api/notes                18 5 4 4 5 6 7 4 5 5 5 4 8w 5 5 4 4 5 4 6 4 4 7
POST /api/notes               16 6 7 7 8 6 6 6 9w 7 9 6 6 6 6 5 6 10 7 6 6 7 7
GET /__bench/not-found        3 2 1 3 3 3 24n 2 2 3 2 2 3 3 2 2 1 1 2 2 2 2 2
```

| The first request in a new isolate, a hello | CPU |
|---|---|
| TinyGo and workers-go as they come | 90 to 170 ms |
| Runtimes reused, stacks reused | 54 to 68 ms |
| And one runtime started while the module loads | 32 to 37 ms |
| And that runtime answers the OpenAPI route during start-up | 19 to 20 ms |
| And the hello route too (what the example does) | 7 to 11 ms |

- **Two runtimes are started while the module loads** (`go.warm` in `worker.mjs`). Cloudflare counts that time against no request. A request that gets one costs 4 to 19 ms.
- **Each answers two routes during start-up,** into nothing: the OpenAPI route, which registers every operation, and the hello.
- **The first use of each operation still costs 15 to 30 ms:** the list 18 to 33, a create 14 to 25. Their handlers need the database, which a runtime does not have during start-up.
- **Requests that arrive together beyond the waiting runtimes start their own:** about 100 ms each when several do at once, 10 to 30 ms for one alone.
- **Cloudflare gives no crypto randomness while a module loads.** For those runtimes TinyGo's seed comes from `Math.random`; anything else that asks for random bytes during start-up stops the warm start.

## What closed the gap

Median CPU per request of the Go Worker, step by step. Why each step was needed: [Huma on Cloudflare Workers](concepts/workers-go.md#what-a-request-costs-and-why).

| Step | hello | list (D1) | create (D1 + hub) |
|---|---|---|---|
| TinyGo and workers-go as they come | 55 ms | 71 ms | about 265 ms |
| 1. The collector no longer runs at every pause: a patch to a copy of TinyGo's runtime, and an 8 MB starting heap | 6 to 19 | 8 to 29 | 61 to 66 |
| 2. Go runtimes are reused between requests (`go/worker/go.mjs`, `transport.Run`) | 3 | 7 | 10 to 13 |
| 3. Goroutine stacks are reused: a second patch | 1 | 3 | 4 |
| 4. A request crosses into Go in one call and out in one (`transport`); D1 rows cross as one JSON string (`d1`); the hub is published to by RPC | 0 | 1 to 2 | 2 |

Request by request, a hello looked like this (Workers Logs, one line per build):

```
TinyGo and workers-go as they come   55 55 43 67 20 60 56 57
tuned collector, new isolate         12 30 12 30 13 31 11 28 12 26 ...
tuned collector, isolate in use       5 13  6 12  5 13  5 12  6 11 ...
and runtimes reused                   3  3  6  3  3 13  2  3  5  2  3  9 ...
and stacks reused                     1  2  2  2  1  2  1  1  2  1  1  2 ...
```

What each step found:

1. **TinyGo's collector ran at every pause.** It collects whenever the program waits and 32 objects with finalizers were made, and workers-go makes one per JavaScript value. It also collects each time its small starting heap must grow ([tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800)).
2. **Every second request cost double, and a new isolate double again.** workers-go starts a Go runtime per request and drops it: the engine then has 8 MB to clear per request. In the fourth line the higher values are the requests that start a runtime: one in three.
3. **A runtime could serve only three requests because of goroutine stacks.** TinyGo allocates one for every goroutine and every call from JavaScript, and rarely frees one ([tinygo-org/tinygo#5801](https://github.com/tinygo-org/tinygo/issues/5801), since fixed on its dev branch).
4. **Every crossing between Go and JavaScript costs.** On scratch Workers, mean CPU of one such operation per request: an awaited promise about 0.12 ms; a D1 read of 21 rows 3.9 ms through `database/sql`, 3.7 ms value by value, 2.8 ms as one JSON string; a hub publish 4.3 ms through workers-go's stub, 3.0 to 3.2 ms as a direct fetch, 2.6 to 2.9 ms as an RPC call.

What a request allocates, under `cf dev` with `runtime.ReadMemStats`:

| | A hello | A create |
|---|---|---|
| 256 KB stacks | 3.4 MB (about 13 stacks) | 8 MB (about 30 stacks) |
| 128 KB stacks | 1.7 MB | 4 MB |
| 128 KB stacks, reused | 38 KB | about 100 KB |

Without stack reuse, 9 hellos left 23 MB in use after 14 collections, and a runtime reused without limit had collections of 65 to 293 ms of CPU and then ran out of memory. So the Go side says when its heap has no room for another request (`transport.Serve`), and the runtime is dropped before the collector runs: after about 80 hellos, not 3.

### The patch and the heap are both needed

Under `cf dev` on an Apple M-series Mac (2026-10-01, runtimes not reused), a create took 37 to 44 ms with TinyGo as it is, 11 to 16 ms with the patched runtime and a 4 MB heap, and 5 to 10 ms with 8 MB, which is what is built. The patch alone helped only the D1 read. 16 MB measured the same as 8. The tables are in [Findings](findings.md).

### Tried and dropped

| What | Why not |
|---|---|
| Reusing a runtime without limit, before stacks were reused | Fast at first, then collections of 65 to 293 ms of CPU, then out of memory after about 40 requests |
| `-gc=leaking` (no collector) | Not safe: a stream the Worker holds can live for hours, and its memory would only grow |
| `-gc=boehm` | Fast locally; deployed to Cloudflare its requests hung (48 requests took 11 minutes) |
| `-opt=2`, `-opt=s` | No faster than `-opt=z` on Cloudflare, and larger |
| A 32 MB starting heap | Slower than 8 MB locally. 16 MB measured the same as 8 |
