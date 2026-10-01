---
title: Performance
nav_order: 4
parent: Plans
grand_parent: This repository
---
# Plan: make the Go Worker cheaper per request

Done for ordinary requests: on Cloudflare a read went from 40 to 70 ms of CPU to 1 to 3 ms, and a write from about 265 ms to about 4, which is what the oRPC Worker costs or close to it ([../benchmarks.md](../benchmarks.md)). What is left is the first request in a new isolate. This page says what was done, what is left, and what was tried and dropped.

The goal: a Go Worker that fits Workers Free (10 ms of CPU) on every request, without giving up anything the soak matrix checks.

## Done (2026-10-01)

- **The collector no longer runs in an ordinary request.** TinyGo ran a full collection at every pause once 32 objects with finalizers had been made, and at each growth of its small heap. `dev wasm-build` builds against a copy of TinyGo's runtime with that threshold at 0, and with a starting heap of 8 MB. Reported as [tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800).
- **Go runtimes are reused** (`api-go/worker/go.mjs`, `transport.Run`). A runtime serves requests one at a time until its heap has no room for another, and is dropped before the collector would run. Told to workers-go in [syumai/workers-go#240](https://github.com/syumai/workers-go/issues/240).
- **Goroutine stacks are reused,** and are 128 KB, half of what they were. A second patch in `dev wasm-build`, to TinyGo's scheduler: a finished goroutine's stack is kept for the next one. A hello allocates 38 KB instead of 3.4 MB. Reported with the patch as [tinygo-org/tinygo#5801](https://github.com/tinygo-org/tinygo/issues/5801).
- **One command that gives a verdict:** `mise run api-go:perf` builds, deploys and prints wall time and Cloudflare's CPU time per operation, in about three minutes.
- **Accepted by the tests:** `mise run check`, the live test, the showcase test and the soak (7 of 7 clients, through a redeploy and a client drop) pass on what is deployed.

## What is left, most promising first

1. **The first request in a new isolate: 40 to 100 ms.** Go starts up there, in Wasm the engine has not optimised yet. Idea: start one Go runtime while the Worker's module loads, so the first request finds it waiting. Start-up up to `workers.Ready()` makes no I/O, which module scope forbids; it has to fit Cloudflare's start-up limit, which a deploy checks. Other candidates: fewer package initialisers (Huma's formats, `regexp`), and pre-initialising the Wasm at build time (Wizer-style), which needs checking against TinyGo's scheduler.
2. **Get the two patches into TinyGo.** Both issues offer a pull request. Until then they live in `dev wasm-build`, and every project that builds with it has them.
3. **Offer the reusing entry to workers-go** (`worker/go.mjs`), as an option of its generator.
4. **Let a runtime live until it is idle.** Today it is dropped when its heap is nearly full, after about 80 hellos, because a collection in a full heap was so costly. With stacks reused the heap holds little that is live, so a collection may now be cheap. Measure it on Cloudflare first.
5. **A stream's own cost:** 40 to 150 ms of CPU over the life of a 15 to 60 second SSE stream. Not looked at.
6. **Let the hub Durable Object hold the client WebSockets, with hibernation.** Then the Go Worker only serves short requests. The cost: catch-up from D1 moves into JavaScript, or the hub calls the Go Worker for it.

## Tried and dropped

- **Reusing a runtime without limit, before stacks were reused:** fast at first, then collections of 65 to 293 ms of CPU, then out of memory after about 40 requests.
- **`-gc=leaking` (no collector):** not safe: a stream the Worker holds can live for hours and its memory would only grow.
- **`-gc=boehm`:** fast locally; deployed to Cloudflare its requests hung (48 requests took 11 minutes).
- **`-opt=2`, `-opt=s`:** no faster than `-opt=z` on Cloudflare, and larger.
- **A 32 MB starting heap:** slower than 8 MB locally. 16 MB measured the same as 8.

## Done when

- The first request in a new isolate fits in 10 ms of CPU, or the docs say plainly that it cannot, with a measurement on Cloudflare.
- `mise run api-go:soak` passes, including `--idle 20`.
