---
title: Benchmarks
nav_order: 2
parent: How to help
---
# Benchmarks: what a request costs, and what made it cheaper

The only page with the numbers: CPU per request on Cloudflare, from Workers Logs. Workers Free allows 10 ms.

## The two notes Workers, side by side

Measured 2026-10-02 with the compare task charter's Go tool had, 30 requests each:

| Operation | TypeScript | Go |
|---|---|---|
| hello | 0 ms | 0 ms |
| list (a D1 read) | 0 to 1 ms | 1 ms |
| create (a D1 write and a hub publish) | 1 ms | 1 to 2 ms |
| The first request in a new isolate | | 7 to 11 ms |
| The first list or create in a new isolate | | 6 to 33 ms (on 2026-10-03: list 11, create 6) |
| A request of a burst that starts its own Go runtime, several at once | | about 100 ms |
| An SSE stream of 15 to 60 seconds, over its life | | 40 to 150 ms |

The conformance Workers (every Fern feature, a bearer token checked on every request), 2026-10-03: Go list about 1 ms, create about 1 ms, 404 0 ms. The TypeScript one kept its notes in memory then (no database), so it was not a like-for-like comparison: list and create 0 ms. Both have kept their notes in D1, with the hub, since 2026-10-15: not measured since. Its API is at `/api/mock`; the Worker's other paths are a web page.

## The Wasm's size

The notes Worker's `build/app.wasm`, gzipped, TinyGo 0.42, 2026-10-03 (the limit is 3,000,000 bytes):

| Build | Gzipped |
|---|---|
| Bearer tokens only (before Access and OIDC) | 862,587 B |
| With Access and OIDC, JWTs verified by go-jose v4.1.5 (what is used) | 1,198,297 B |
| The same with golang-jwt v5.3.1, keyfunc v3.8.2 and jwkset v0.11.3 instead (tried) | 1,273,945 B |

Both libraries passed the same tests under workerd. go-jose was kept: smaller, no dependencies of its own, keys fetched only in a request (keyfunc keeps them fresh from a goroutine of its own, which a Worker cannot run between requests; without it, they are never fetched again).

## Building the Worker

How long a fresh TinyGo build of the notes Worker takes (`go/examples-notes`, past charter's cache, TinyGo's own caches warm, on the owner's Mac), and what it makes, 2026-10-15:

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

## What closed the gap

The Go Worker's median CPU, 2026-10-01 and 2026-10-02:

| Step | hello | list | create |
|---|---|---|---|
| TinyGo and workers-go as they come | 55 ms | 71 ms | about 265 ms |
| 1. The collector no longer runs at every pause: a runtime patch, an 8 MB heap | 6 to 19 | 8 to 29 | 61 to 66 |
| 2. Go runtimes reused between requests (`go/lib/worker/go.mjs`) | 3 | 7 | 10 to 13 |
| 3. Goroutine stacks reused: a second patch | 1 | 3 | 4 |
| 4. One crossing into Go and one out; D1 rows as one JSON string; the hub published to by RPC | 0 | 1 to 2 | 2 |
| 5. Two runtimes started and warmed while the module loads (a new isolate's first request) | 7 to 11, was 90 to 170 | | |

Tried and dropped: `-gc=boehm` (hung on Cloudflare), `-gc=leaking` (a stream's memory only grows), `-opt=2`, `-opt=s` (larger, no faster), a 32 MB heap (slower), unlimited runtime reuse without stack reuse (out of memory).

Left: the first use of each operation in a new isolate ([#22](https://github.com/joeblew999/charter/issues/22)), whose handler needs the database a starting runtime lacks.
