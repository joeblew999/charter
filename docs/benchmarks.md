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

| Operation | oRPC Worker (`orpc-api`) | Go Worker, TinyGo as it is | Go Worker, tuned build: the faster runs | Go Worker, tuned build: the slower runs |
|---|---|---|---|---|
| `GET /api/hello` | 1 ms | 55 ms | 6 to 8 ms | 14 to 19 ms |
| `GET /api/notes` (D1) | 1 to 3 ms | 71 ms | 8 to 11 ms | 22 to 29 ms |
| A path that does not exist (404) | not measured | 38 ms | 5 to 7 ms | 12 to 17 ms |
| `POST /api/notes` (D1 write and hub publish) | not measured | about 265 ms | not measured | 61 to 66 ms |

The tuned build is what `mise run api-go:build` makes and what is deployed. The two columns for it are whole runs of `mise run api-go:bench`, minutes apart, on the same build: every run landed in one range or the other. The two runs made for this page were both in the slower one:

| Operation | Wall median | Wall slowest | CPU median | CPU p99 |
|---|---|---|---|---|
| `GET /api/hello` | 37.5 ms | 54.1 ms | 14 ms | 39 ms |
| `GET /api/notes` | 85.4 ms | 110.7 ms | 28 ms | 43 ms |
| `POST /api/notes` | 172.4 ms | 467.8 ms | 61 ms | 78 ms |
| A path that does not exist | 46.5 ms | 184.6 ms | 12 ms | 31 ms |

The oRPC Worker in the same hour: hello 13.1 ms wall and 1 ms CPU, the list 49.7 ms wall and 1 ms CPU.

The Go showcase Worker (`orpc-showcase-go`: every Fern feature, a bearer token checked with HMAC on every request), tuned build: a list 14 to 15 ms, a create 15 to 18 ms, a 404 7 to 12 ms.

What this means:

- **The tuned build costs a third to an eighth of what TinyGo's own build cost.** Nothing in the API changed: the same code, the same tests.
- **It is still several times the oRPC Worker,** which uses about 1 ms.
- **Workers Free's limit is 10 ms of CPU per request.** The faster runs fit for reads; the slower runs and every write do not. Plan on Workers Paid, where CPU is billed per millisecond and the default limit is 30 s.
- **Why two ranges is not established.** It did not depend on the optimisation level (`-opt=2` measured the same as `-opt=z`) or on whether the run wrote data. The likely cause is that Cloudflare runs the Worker on many machines and each optimises the Wasm on its own schedule; that was not verified ([plans/performance.md](plans/performance.md)).
- **The slowest requests cost about 40 ms** (the 99th percentile) in either range.

## What made it cheaper

TinyGo's collector ran far more often than a Worker needs:

- **A full collection at every pause.** TinyGo collects whenever the program waits and 32 objects with finalizers were made since the last time. workers-go makes one for every JavaScript value it touches, and a request waits many times: for its body, for D1, for the hub.
- **A collection each time the heap grows.** TinyGo starts with a heap of a few pages.

`dev wasm-build` builds against a copy of TinyGo's runtime with that threshold set to 0, and with a starting heap of 8 MB ([the dev tool](reference/dev.md#wasm-build)). The collector still runs when the heap is full, so a stream that lives for hours is still collected. Reported as [tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800).

The same Wasm under `cf dev` on an Apple M-series Mac, time inside the Worker per request (4 requests each, 2026-10-01):

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
- **The tuned build is as fast as having no collector,** and still collects.

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
- **Run it more than once.** One run shows one of the two ranges above.

Every flag is in [the dev tool](reference/dev.md#bench).
