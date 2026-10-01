---
title: Performance
nav_order: 4
parent: Plans
grand_parent: This repository
---
# Plan: make the Go Worker cheaper per request

Mostly done. On Cloudflare a read went from 40 to 70 ms of CPU to 2 to 7 ms, and a write from about 265 ms to 10 to 13 ([../benchmarks.md](../benchmarks.md)). This page says what was done, what is left, and what was tried and dropped.

The goal: a Go Worker that fits Workers Free (10 ms of CPU) on every request, without giving up anything the soak matrix checks.

## Done (2026-10-01)

- **The collector no longer runs in an ordinary request.** TinyGo ran a full collection at every pause once 32 objects with finalizers had been made, and at each growth of its small heap. `dev wasm-build` builds against a copy of TinyGo's runtime with that threshold at 0, and with a starting heap of 8 MB. Reported as [tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800) and to workers-go as [syumai/workers-go#240](https://github.com/syumai/workers-go/issues/240).
- **Go runtimes are reused** (`api-go/worker/go.mjs`, `transport.Run`). A runtime serves requests one at a time until its heap has no room for another, and is dropped before the collector would run.
- **Goroutine stacks are 128 KB,** half of what they were: stacks are most of what a request allocates.
- **One command that gives a verdict:** `mise run api-go:perf` builds, deploys and prints wall time and Cloudflare's CPU time per operation, in about three minutes.
- **Accepted by the tests:** `mise run check`, the live test and the soak (7 of 7 clients, through a redeploy and a client drop) pass on what is deployed.

## What is left, most promising first

1. **Reuse goroutine stacks in TinyGo.** Every goroutine and every call from JavaScript into Go allocates a stack, and the collector rarely frees one: that is why a runtime serves about three requests and not thousands. A free list of stacks in TinyGo's scheduler (`src/internal/task/task_asyncify.go`) would take a request's allocation from megabytes to kilobytes. Then a runtime could serve requests until it is idle, and a write would cost what a read does. This is a real change to TinyGo, to be proposed upstream first.
2. **Find what keeps dead stacks alive.** With TinyGo as it is, 9 hellos left 23 MB in use after 14 collections. If it is the conservative scan of stacks, 1 fixes it; if something holds them, that is a bug to report.
3. **The request that starts a runtime** costs 5 to 13 ms. Candidates: fewer package initialisers (Huma's formats, `regexp`), and pre-initialising the Wasm at build time (Wizer-style), which needs checking against TinyGo's scheduler.
4. **Writes.** 10 to 13 ms where a read is 2 to 7. A write crosses between Go and JavaScript many more times: the body, D1, the hub.
5. **Let the hub Durable Object hold the client WebSockets, with hibernation.** Then the Go Worker only serves short requests. The cost: catch-up from D1 moves into JavaScript, or the hub calls the Go Worker for it.

## Tried and dropped

- **Reusing a runtime without limit:** fast at first, then collections of 65 to 293 ms of CPU, then out of memory after about 40 requests. See 1 and 2 above.
- **`-gc=leaking` (no collector):** not safe: a stream the Worker holds can live for hours and its memory would only grow.
- **`-gc=boehm`:** fast locally; deployed to Cloudflare its requests hung (48 requests took 11 minutes).
- **`-opt=2`, `-opt=s`:** no faster than `-opt=z` on Cloudflare, and larger.
- **A 32 MB starting heap:** slower than 8 MB locally. 16 MB measured the same as 8.

## Done when

- A write fits in 10 ms of CPU, and so does the request that starts a runtime, and benchmarks.md says so with a measurement on Cloudflare.
- `mise run api-go:soak` passes, including `--idle 20`.
