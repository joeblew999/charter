---
title: Benchmarks
nav_order: 17
---
# Benchmarks: what a request costs, and what made it cheaper

The only page with the numbers: CPU per request on Cloudflare, from Workers Logs. Workers Free allows 10 ms.

## The two notes Workers, side by side

Measured 2026-10-02, 30 requests each, CPU time from Workers Logs:

| Operation | TypeScript | Go |
|---|---|---|
| hello | 0 ms | 0 ms |
| list (a D1 read) | 0 to 1 ms | 1 ms |
| create (a D1 write and a hub publish) | 1 ms | 1 to 2 ms |
| The first request in a new isolate | | 7 to 11 ms |
| The first list or create in a new isolate | | 6 to 33 ms (on 2026-10-03: list 11, create 6) |
| A request of a burst that starts its own Go runtime, several at once | | about 100 ms |
| An SSE stream of 15 to 60 seconds, over its life | | 40 to 150 ms |

The Go conformance Worker (every Fern feature, a bearer token checked on every request), 2026-10-03: list about 1 ms, create about 1 ms, a 404 0 ms. Its TypeScript twin had no database then, so it was not compared; both keep their notes in D1 now, and have not been measured since.

## The Wasm's size

The notes Worker's `build/app.wasm`, gzipped, TinyGo 0.42, 2026-10-03, when its JWT library was chosen (the limit is 3,000,000 bytes):

| Build | Gzipped |
|---|---|
| Bearer tokens only (before Access and OIDC) | 862,587 B |
| With Access and OIDC, JWTs verified by go-jose v4.1.5 (what is used) | 1,198,297 B |
| The same with golang-jwt v5.3.1, keyfunc v3.8.2 and jwkset v0.11.3 instead (tried) | 1,273,945 B |

Both libraries passed the same tests under workerd. go-jose was kept: smaller, no dependencies of its own, keys fetched only in a request (keyfunc keeps them fresh from a goroutine of its own, which a Worker cannot run between requests; without it, they are never fetched again).

## Building the Worker

How long a fresh TinyGo build of the notes Worker takes (`examples/notes-go`, past charter's cache, TinyGo's own caches warm, on an Apple M2 Pro with 16 GB), and what it makes, 2026-10-15:

| TinyGo | Optimization | Build | `build/app.wasm` | Gzipped |
|---|---|---|---|---|
| 0.42.0 | `-opt=z` (what charter uses) | 21.8 s | 3.28 MB | 1.20 MB |
| 0.42.0 | `-opt=s` | 38.9 s | 3.53 MB | 1.26 MB |
| 0.42.0 | `-opt=2` | 45.9 s | 4.17 MB | 1.48 MB |
| 0.42.0 | `-opt=1` | 40.8 s | 4.39 MB | 1.50 MB |
| 0.42.0 | `-opt=0` | fails | | |
| 0.43.0 | `-opt=z` (what charter uses) | 22.9 s, 23.0 s | 3.81 MB | 1.38 MB |
| 0.43.0 | `-opt=s` | 39.8 s | 4.30 MB | 1.49 MB |
| 0.43.0 | `-opt=2` | 48.3 s | 5.23 MB | 1.79 MB |
| 0.43.0 | `-opt=1` | 43.1 s | 5.36 MB | 1.76 MB |
| 0.43.0 | `-opt=0` | 22.2 s | 17.22 MB | 7.47 MB (over the 3 MB limit) |

The smallest build is also the quickest: most of a build is wasm-opt's asyncify pass, and it takes longer the bigger the Wasm. So there is no faster build for local checks to switch to; a build is quick when charter's cache has it (the same sources make the same Wasm). TinyGo 0.43.0 builds in the same time as 0.42.0 and makes a Wasm 15 % bigger. Its first build after the upgrade took 38.7 s, while TinyGo filled its own caches.

## The Worker as a deploy uploads it

What `charter build` makes of each notes Worker (`charter check` measures it), every module it uploads, 2026-10-29 on macOS (arm64), TinyGo 0.44.0:

| Worker | Bundle | Gzipped |
|---|---|---|
| `examples/notes-ts` (esbuild, minified) | 1.32 MB | 0.27 MB |
| `examples/notes-go` (the Wasm is 1.36 MB of it, gzipped) | 3.79 MB | 1.36 MB |

Workers Free allows 3 MB gzipped, Workers Paid 10 MB.

## What closed the gap

The Go Worker's median CPU, 2026-10-01 and 2026-10-02, on TinyGo 0.42 (step 3's patch is gone: TinyGo 0.43.0 frees finished goroutines' stacks itself; not measured again since):

| Step | hello | list | create |
|---|---|---|---|
| TinyGo and workers-go as they come | 55 ms | 71 ms | about 265 ms |
| 1. The collector no longer runs at every pause: a runtime patch, an 8 MB heap | 6 to 19 | 8 to 29 | 61 to 66 |
| 2. Go runtimes reused between requests (`worker/go.mjs`) | 3 | 7 | 10 to 13 |
| 3. Goroutine stacks reused: a second patch | 1 | 3 | 4 |
| 4. One crossing into Go and one out; D1 rows as one JSON string; the hub published to by RPC | 0 | 1 to 2 | 2 |
| 5. Two runtimes started and warmed while the module loads (a new isolate's first request) | 7 to 11, was 90 to 170 | | |

Tried and dropped: `-gc=boehm` (hung on Cloudflare), `-gc=leaking` (a stream's memory only grows), `-opt=2`, `-opt=s` (larger, no faster), a 32 MB heap (slower), unlimited runtime reuse without stack reuse (out of memory).

Left: the first use of each operation in a new isolate ([#22](https://github.com/joeblew999/charter/issues/22)), whose handler needs the database a starting runtime lacks.

## Each difference alone, measured by anyone

`charter build`'s switches turn each optimisation off: `-reuse=false` (a Go runtime per request, as workers-go's own entry), `-patches=false` (TinyGo as it ships), `-heap=0` (TinyGo's default heap). `charter bench` measures the CPU a local workerd spends per request, from the operating system's count, in repeated runs. No Cloudflare account is needed, so a comparison can be checked by anyone on their own machine ([Libraries](libraries.md)).

[go-htmx4](https://github.com/joeblew999/go-htmx4) is a Go Worker that is not built from charter: htmx 4 and gsxui pages of 50 to 170 KB, 13 locales, the server's Intl, a live board on D1 and a Durable Object. `mise run bench:variants` builds its current code once per switch combination, measures each, and writes the table to its [docs/benchmarks.md](https://github.com/joeblew999/go-htmx4/blob/main/docs/benchmarks.md). Its CI runs the shipped build's half on every change.

On Cloudflare (Workers Logs, 2026-10-20, Worker Previews of one app sent the same requests in turns), the same comparison, median CPU per request in ms:

| | `/healthz` | `/de/` | `/board` | board write | `/de/formats` |
|---|---|---|---|---|---|
| A runtime per request, TinyGo as it ships | 20 | 35 | 58 | 70 | 124 |
| C runtimes reused, TinyGo as it ships | 15 | 53 | 25 | 67 | 127 |
| D runtimes reused, TinyGo patched (what charter ships) | 4 | 19 | 22 | 49 | 80 |

A runtime kept between requests saves the cost of starting Go, but without the patch it collects at every await over a heap that is no longer new, so a heavy page gains nothing (C); with the patch, it does (D). One run on Cloudflare moves 10 to 20 ms a cell; the local table is the one to check against.
