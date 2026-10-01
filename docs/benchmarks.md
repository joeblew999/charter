---
title: Benchmarks
nav_order: 9
parent: This repository
---
# Benchmarks: what a request costs on each Worker

What one request costs on the oRPC Worker and on the Go Worker, how the Go build was made cheaper, and how to measure your own API. Read it before choosing between the two for a project, or before working on the Go Worker's cost ([plans/performance.md](plans/performance.md)).

Measured on 2026-10-01 on Cloudflare, with the two Workers serving the same API from the same kind of D1 database. CPU time is what Workers bills and limits. It is Cloudflare's own figure, from Workers Logs (`$workers.cpuTimeMs`), the median of 20 requests per operation unless a row says otherwise.

## On Cloudflare

CPU time per request, median:

| Operation | oRPC Worker (`orpc-api`) | Go Worker, TinyGo and workers-go as they come | Go Worker, tuned build only | Go Worker now: tuned build and reused runtimes |
|---|---|---|---|---|
| `GET /api/hello` | 1 ms | 55 ms | 6 to 19 ms | 3 ms |
| `GET /api/notes` (D1) | 1 to 3 ms | 71 ms | 8 to 29 ms | 7 ms |
| A path that does not exist (404) | not measured | 38 ms | 5 to 17 ms | 2 ms |
| `POST /api/notes` (D1 write and hub publish) | not measured | about 265 ms | 61 to 66 ms | 10 to 13 ms |

The last column is what a project made by `dev new` gets. Two runs of `mise run api-go:bench` with writes, each right after a deploy:

| Operation | Wall median | Wall slowest | CPU median | CPU p99 |
|---|---|---|---|---|
| `GET /api/hello` | 16.5 ms | 46.5 ms | 3 ms | 13 to 14 ms |
| `GET /api/notes` | 55.7 ms | 79.7 ms | 7 ms | 16 ms |
| `POST /api/notes` | 103.7 ms | 196.8 ms | 10 to 13 ms | 17 to 20 ms |
| A path that does not exist | 16.7 ms | 27.9 ms | 2 ms | 9 to 16 ms |

The oRPC Worker in the same hour: hello 13.1 ms wall and 1 ms CPU, the list 49.7 ms wall and 1 ms CPU.

The Go showcase Worker (`orpc-showcase-go`: every Fern feature, a bearer token checked with HMAC on every request): a list 2 ms (p99 13), a create 5 ms (p99 10), a 404 2 ms. With the tuned build only they were 14 to 15, 15 to 18 and 7 to 12 ms.

What this means:

- **A read costs 2 to 7 ms and a write 10 to 13 ms.** That is 10 to 20 times less than TinyGo and workers-go as they come. Nothing in the API changed: the same code, the same tests.
- **It is still several times the oRPC Worker,** which uses about 1 ms.
- **Workers Free's limit is 10 ms of CPU per request.** Reads fit. Writes are at the limit and the slowest requests are over it. Free is enough to try a project; plan on Workers Paid for production.
- **A client sees almost no difference:** 16.5 ms for a hello against 13.1 ms from the oRPC Worker.

## What made it cheaper

Request by request, the CPU time of a hello looks like this (Workers Logs, one line per build):

```
TinyGo and workers-go as they come   55 55 43 67 20 60 56 57
tuned build, new isolate             12 30 12 30 13 31 11 28 12 26 ...
tuned build, isolate in use a while   5 13  6 12  5 13  5 12  6 11 ...
tuned build and reused runtimes       3  3  6  3  3 13  2  3  5  2  3  9 ...
```

Three things, in the order they were found:

1. **TinyGo's collector ran at every pause.** TinyGo collects whenever the program waits and 32 objects with finalizers were made since the last time. workers-go makes one for every JavaScript value it touches, and a request waits many times: for its body, for D1, for the hub. It also collects each time its small starting heap must grow. `dev wasm-build` builds against a copy of TinyGo's runtime with that threshold at 0 and with a starting heap of 8 MB ([the dev tool](reference/dev.md#wasm-build)). Reported as [tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800). That is the second and third line.
2. **Every second request cost double, and a new isolate cost double again.** workers-go starts a Go runtime for each request and drops it. The JavaScript engine then has an 8 MB memory to clear away per request, and the Wasm is not yet optimised in an isolate that has just started.
3. **Reusing a runtime removes both,** and the start-up of Go with them. `api-go/worker/go.mjs` keeps a runtime that has finished a request for the next one. That is the last line: the higher values are the requests that start a new runtime.

Why a runtime is reused only a few times: a request's memory is mostly goroutine stacks, and TinyGo's collector rarely frees them. Measured under `cf dev`, with 256 KB stacks:

| | Allocated per request | In use after 9 hellos, collector run |
|---|---|---|
| A hello | 3.4 MB (about 13 stacks) | TinyGo as it is: 23 MB after 14 collections |
| A create | 8 MB (about 30 stacks) | |

A runtime that was reused without limit had collections costing 65, 148, 175 and 293 ms of CPU, then ran out of memory. So the Go side counts what each request allocates and says when the heap has no room for another (`transport.Serve`), and the runtime is dropped before the collector runs. With stacks of 128 KB a hello allocates 1.7 MB, so a runtime serves about three of them.

The same Wasm under `cf dev` on an Apple M-series Mac, time inside the Worker per request (4 requests each, 2026-10-01, runtimes not reused, 256 KB stacks). It shows what the patch and the heap each do:

| Build | 404 | hello | list 20 (D1) | openapi.json | create |
|---|---|---|---|---|---|
| TinyGo as it is | 8 ms | 8 ms | 14 to 16 ms | 10 to 11 ms | 37 to 44 ms |
| Patched runtime, TinyGo's own starting heap | 8 ms | 8 to 9 ms | 9 to 10 ms | 10 to 11 ms | not measured |
| Patched runtime, 2 MB heap | 4 to 5 ms | 4 to 5 ms | 5 to 6 ms | 7 ms | 13 to 20 ms |
| Patched runtime, 4 MB heap | 1 to 2 ms | 1 to 2 ms | 3 ms | 3 to 4 ms | 11 to 16 ms |
| **Patched runtime, 8 MB heap (what is built)** | 1 ms | 1 to 2 ms | 2 to 3 ms | 3 to 4 ms | 5 to 10 ms |
| Patched runtime, 16 MB heap | 1 to 2 ms | 1 to 2 ms | 3 ms | 3 ms | 4 to 11 ms |
| No collector (`-gc=leaking`) | 1 to 3 ms | 1 to 4 ms | 3 to 5 ms | 2 to 5 ms | not measured |

- **Both changes are needed.** The patch alone helps only the D1 read; the heap does the rest.
- **8 MB is enough.** 16 MB measured the same.

## Other build options, measured before the fix

Wall clock under `cf dev`, mean of 20, with TinyGo as it is. Kept because they say what does not help. The sizes are of the build measured that day; the tuned build is 876 KB gzipped.

| Build (`tinygo build ...`) | Wasm gzipped | 404 | hello | list 20 (D1) | openapi.json |
|---|---|---|---|---|---|
| **default (`-gc=precise`), what is deployed** | 848 KB | 13.2 ms | 13.4 ms | 27.1 ms | 20.2 ms |
| `-opt=2` | 1037 KB | 13.6 ms | 14.3 ms | 33.3 ms | |
| `-opt=s` | 879 KB | 14.2 ms | 14.3 ms | 29.7 ms | |
| `-gc=conservative` | 840 KB | 13.3 ms | 14.1 ms | 27.3 ms | |
| `-gc=boehm` | 862 KB | 10.7 ms | 11.5 ms | 13.1 ms | 11.8 ms |
| `-gc=leaking` (no collector) | 698 KB | 6.4 ms | 6.6 ms | 7.2 ms | 7.1 ms |
| precise, 8 MB initial memory | 848 KB | 11.0 ms | 12.7 ms | 18.8 ms | |
| precise, 32 MB initial memory | 848 KB | 16.6 ms | 14.7 ms | 23.9 ms | |
| an empty workers-go handler, no Huma (precise) | 326 KB | | about 8 ms | | |

What this says:

- **The optimisation level is not the cost.** The collector was.
- **`-gc=boehm` is not usable:** deployed to Cloudflare, its requests hung (48 requests took 11 minutes). Not investigated further.
- **`-gc=leaking` is not safe:** a stream or WebSocket the Worker holds can live for hours, and its memory would only grow.

## Measuring it again, and measuring your own API

```sh
mise run api-go:bench         # REMOTE, read-only: the deployed Go Worker, wall time and Cloudflare's CPU time
mise run api-go:perf          # REMOTE: build, deploy, then the same bench: about three minutes
mise run api:bench            # the same bench against the deployed oRPC Worker
go run ./dev bench <url>      # any server: a local cf dev, the native build. Wall time only
```

`bench` works on any API: it reads the OpenAPI spec the server gives at `/api/openapi.json`, and calls every GET operation whose required inputs have an example in the spec, plus one path that does not exist. So in your own project the same two tasks measure your operations, not the notes example's.

- **`-write`** adds the operations that change data, with the example of each request body.
- **`-header 'Authorization: Bearer <token>'`** sends a header with every request, for an API that needs a token.
- **`-cpu`** needs `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` (the environment, or fnox) and waits up to two minutes for Cloudflare's logs.
- **To see each request,** not the median: Workers Logs in the dashboard, the field `$workers.cpuTimeMs`.

Every flag is in [the dev tool](reference/dev.md#bench).
