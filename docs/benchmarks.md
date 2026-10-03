---
title: Benchmarks
nav_order: 2
parent: How to help
---
# Benchmarks: what a request costs, and what made it cheaper

The only page with the numbers: CPU per request on Cloudflare, from Workers Logs. Workers Free allows 10 ms. How to measure: [Performance](guides/performance.md).

## The two notes Workers, side by side

`mise run compare` at the repo root, 2026-10-02, 30 requests each:

| Operation | TypeScript | Go |
|---|---|---|
| hello | 0 ms | 0 ms |
| list (a D1 read) | 0 to 1 ms | 1 ms |
| create (a D1 write and a hub publish) | 1 ms | 1 to 2 ms |
| The first request in a new isolate | | 7 to 11 ms |
| The first list or create in a new isolate | | 6 to 33 ms (on 2026-10-03: list 11, create 6) |
| A request of a burst that starts its own Go runtime, several at once | | about 100 ms |
| An SSE stream of 15 to 60 seconds, over its life | | 40 to 150 ms |

The Go showcase Worker (every Fern feature, a bearer token checked on every request), 2026-10-03: list about 1 ms, create about 1 ms, 404 0 ms.

## What closed the gap

The Go Worker's median CPU, 2026-10-01 and 2026-10-02:

| Step | hello | list | create |
|---|---|---|---|
| TinyGo and workers-go as they come | 55 ms | 71 ms | about 265 ms |
| 1. The collector no longer runs at every pause: a runtime patch, an 8 MB heap | 6 to 19 | 8 to 29 | 61 to 66 |
| 2. Go runtimes reused between requests (`go/worker/go.mjs`) | 3 | 7 | 10 to 13 |
| 3. Goroutine stacks reused: a second patch | 1 | 3 | 4 |
| 4. One crossing into Go and one out; D1 rows as one JSON string; the hub published to by RPC | 0 | 1 to 2 | 2 |
| 5. Two runtimes started and warmed while the module loads (a new isolate's first request) | 7 to 11, was 90 to 170 | | |

Tried and dropped: `-gc=boehm` (hung on Cloudflare), `-gc=leaking` (a stream's memory only grows), `-opt=2`, `-opt=s` (larger, no faster), a 32 MB heap (slower), unlimited runtime reuse without stack reuse (out of memory).

Left: the first use of each operation in a new isolate ([#22](https://github.com/joeblew999/charter/issues/22)), whose handler needs the database a starting runtime lacks.
