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

How long a fresh TinyGo build of the notes Worker takes (`go/examples-notes`, past charter's cache, TinyGo's own caches warm, on an Apple M2 Pro with 16 GB), and what it makes, 2026-10-15:

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

What `cf build` makes of each notes Worker (`npm run check` measures it), 2026-10-15, TinyGo 0.43.0:

| Worker | Bundle | Gzipped |
|---|---|---|
| `ts/examples-notes` | 1.02 MB | 0.24 MB |
| `go/examples-notes` (the Wasm is 1.38 MB of it, gzipped) | 3.83 MB | 1.38 MB |

Workers Free allows 3 MB gzipped, Workers Paid 10 MB.

## What closed the gap

The Go Worker's median CPU, 2026-10-01 and 2026-10-02, on TinyGo 0.42 (step 3's patch is gone: TinyGo 0.43.0 frees finished goroutines' stacks itself; not measured again since):

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

## Each difference alone, in another app

[go-htmx4](https://github.com/joeblew999/go-htmx4) is a Go Worker that is not built from charter: htmx 4 and gsxui pages, 13 locales, the server's Intl, a live board on D1 and a Durable Object, built with TinyGo 0.43.0 and workers-go v0.36.0. Its pages are big (50 to 170 KB of HTML). Each variant below is a Worker Preview of one commit, all sent the same requests in turns over the same six minutes (2026-10-20, 30 rounds, every answer a 200). CPU time is from Workers Logs, the median of 23 to 53 requests per cell, in ms:

| Variant | `/healthz` | `/de/` | `/board` (D1 read) | board write (D1, Durable Object) | `/de/formats` (111 Intl calls) |
|---|---|---|---|---|---|
| A. As it shipped: workers-go's entry, TinyGo as it ships | 20 | 35 | 58 | 70 | 124 |
| B. A, with TinyGo patched (tinygo#5800), an 8 MB heap and 128 KB stacks | 21 | 77 | 41 | 33 | 108 |
| C. Go runtimes reused (`go.mjs`, `transport.Run`), TinyGo as it ships (`workerbuild -patches=false`) | 15 | 53 | 25 | 67 | 127 |
| D. Both: charter's runtime | 4 | 19 | 22 | 49 | 80 |
| E. D, and the app's pages allocating a third (a pooled page buffer, cached Tailwind merging) | 1 | 22 | 18 | 24 | 73 |

What it shows:

- **Reused runtimes are most of it, and they need the patch.** C alone takes the cost of starting Go away (`/healthz` 15 ms), but without the patch a reused runtime collects at every await, over a heap that is no longer new: the heavy page costs the same as A (127 ms). With the patch (D) it is 80 ms, and the home page 19 ms.
- **The patch alone is no win.** Without reuse every request starts a runtime, and B pays for the 8 MB heap each time: some pages are cheaper, the home page dearer.
- **What is left is the app's own work.** E's `/de/formats` makes 111 calls from Go to the Worker's Intl, each a crossing between Wasm and JavaScript; the next step there is fewer crossings (step 4 of the table above).

The branches are `bench-a-shipped`, `bench-b-gcpatch`, `bench-c-reuse-nopatch`, `bench-d-runtime` and `render-allocs` in go-htmx4. A cell moves by 10 to 20 ms from one run to the next (which isolate a request lands in, a new one or a warm one), so read the table by its rows, not by single cells.
