---
title: Benchmarks
nav_order: 8
---

# Benchmarks: what a request costs on each Worker

Measured on 2026-10-01 on Cloudflare, with the two Workers serving the same API from the same kind of D1 database. CPU time is what Workers bills and limits; it comes from Workers Logs (`$workers.cpuTimeMs`, median over 8 or more requests per route).

## On Cloudflare

| Route | oRPC Worker (`orpc-api`), CPU | Go Worker (`orpc-api-go`), CPU | Go Worker, wall |
|---|---|---|---|
| `GET /api/hello` | 1 ms | 55 ms | 56 ms |
| `GET /api/notes?limit=20` (D1) | 3 ms | 71 ms | 117 ms |
| `GET /api/openapi.json` | not measured | 71 ms | 72 ms |
| `GET /api/nope` (404) | not measured | 38 ms | 40 ms |
| `POST /api/notes` (D1 write + hub publish) | not measured | about 265 ms (max 386) | not measured |
| `GET /api/notes/watch` (a stream) | not measured | 103 to 171 ms over the stream's life | as long as the stream |

Wall time as one client saw it (`mise run api:bench`, `mise run api-go:bench`; median of 20, network included):

| Route | oRPC Worker | Go Worker |
|---|---|---|
| `GET /api/hello` | 14 ms | 66 ms |
| `GET /api/notes?limit=20` | 53 ms | 107 ms |
| `GET /api/openapi.json` | 15 ms | 85 ms |
| `GET /api/nope` (404) | 13 ms | 63 ms |

What this means:

- **The Go Worker costs roughly 40 to 70 ms of CPU per request; the oRPC Worker 1 to 3 ms.** Both are correct and pass the same tests; the difference is cost.
- **It fits Workers Paid** (CPU is billed per millisecond; the default limit is 30 s per request). **It does not fit Workers Free**, whose limit is 10 ms of CPU per request.
- **Most of it is start-up, paid on every request.** workers-go starts a fresh Go runtime for each request, and the 404, which runs no handler, already costs 38 ms.

## Locally (workerd on an Apple M-series Mac, wall clock, mean of 20)

The same Wasm under `cf dev`. Useful for comparing builds, not for absolute numbers: production CPU was about four times these.

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

- **The garbage collector is the cost, not the optimisation level.** With no collector a request takes half the time and a D1 list a quarter.
- **`-gc=boehm` looked like the safe win locally and is not usable:** deployed to Cloudflare, its requests hung (48 requests took 11 minutes). Not investigated further.
- **`-gc=leaking` is not safe as it stands:** a stream or WebSocket the Worker holds can live for hours, and its memory would only grow. [plans/performance.md](plans/performance.md) has the design that would make it safe.

## Measuring it again

```sh
mise run api:bench            # wall time per route against the deployed oRPC Worker
mise run api-go:bench         # the same against the deployed Go Worker
go run ./dev bench <url>      # any server: a local cf dev, the native build
```

`bench` reports wall time as a client sees it (median and slowest of 20, after 3 warm-up requests). CPU time has to be read from Workers Logs: in the dashboard, or with the Workers Observability API, the median of `$workers.cpuTimeMs` grouped by `$metadata.trigger`.
