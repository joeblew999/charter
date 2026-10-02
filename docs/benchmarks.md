---
title: Benchmarks
nav_order: 3
parent: How to help
---
# Benchmarks: what a request costs, and what made it cheaper

What one request costs on the TypeScript ([oRPC](https://orpc.dev)) notes Worker and on the Go ([Huma](https://huma.rocks)) one, and how the Go build got there. This is the only page with the numbers: other pages link here. Read it before you choose between the two, or before you work on cost ([the plan](plans/performance.md)). To measure your own API: [Measure and improve performance](guides/performance.md).

Measured on Cloudflare on 2026-10-01 and 2026-10-02, with the two Workers serving the same API from the same kind of D1 database, before the repository's rename (the Workers were then called `orpc-api` and `orpc-api-go`). CPU time is what Workers bills and limits: Cloudflare's own figure, from Workers Logs (`$workers.cpuTimeMs`).

## On Cloudflare, side by side

`mise run compare` runs the same bench against both Workers. CPU per request in ms, 2026-10-02, 30 requests each, as Cloudflare logged them, on the Workers deployed now (`charter-notes-ts`, `charter-notes-go`):

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

## What closed the gap

Median CPU per request of the Go Worker, step by step:

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

## A new isolate

Cloudflare starts an isolate after a deploy, when a Worker has been idle, and when traffic spreads. The Wasm is not optimised there yet, and Go has to start. `mise run perf` in `examples/notes-go/` deploys, sends 8 requests at once, then each operation in turn (2026-10-02, CPU in ms):

```
GET /api/hello, 8 at once     3 6n 5w 74n 13w 110n 83n 101n
GET /api/hello                2 2 2 3 2 3 2 2 2 3 3 2 4 2 3 6 3 2 2 2 1 3 2
GET /api/notes                18 5 4 4 5 6 7 4 5 5 5 4 8w 5 5 4 4 5 4 6 4 4 7
POST /api/notes               16 6 7 7 8 6 6 6 9w 7 9 6 6 6 6 5 6 10 7 6 6 7 7
GET /__bench/not-found        3 2 1 3 3 3 24n 2 2 3 2 2 3 3 2 2 1 1 2 2 2 2 2
```

`w`: served by a Go runtime started while the Worker's module loaded. `n`: by one the request had to start. No letter: by a reused one.

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

## The patch and the heap are both needed

Under `cf dev` on an Apple M-series Mac (2026-10-01, runtimes not reused), a create took 37 to 44 ms with TinyGo as it is, 11 to 16 ms with the patched runtime and a 4 MB heap, and 5 to 10 ms with 8 MB, which is what is built. The patch alone helped only the D1 read. 16 MB measured the same as 8. The tables are in [Findings](findings.md).

What was tried and dropped (other collectors, optimisation levels, bigger heaps): [the performance plan](plans/performance.md#tried-and-dropped).
