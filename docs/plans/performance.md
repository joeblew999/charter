---
title: Performance
nav_order: 4
parent: Plans
grand_parent: This repository
---
# Plan: make the Go Worker cheaper per request

Partly done. The largest cost is found and removed: a read went from 40 to 70 ms of CPU on Cloudflare to 6 to 28 ms, and a write from about 265 ms to about 60 ([../benchmarks.md](../benchmarks.md)). This page says what was done, what is left, and what was tried and dropped.

The goal: a Go Worker that fits Workers Free (10 ms of CPU) on every request, without giving up anything the soak matrix checks.

## Done (2026-10-01)

- **The cause.** TinyGo ran a full garbage collection at every pause of the program once 32 objects with finalizers had been made, and workers-go makes one for every JavaScript value. On top of that, TinyGo's heap starts at a few pages and collects each time it grows.
- **The fix,** in `dev wasm-build`: a copy of TinyGo's runtime with that threshold set to 0, and a starting heap of 8 MB. Reported as [tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800) and to workers-go as [syumai/workers-go#240](https://github.com/syumai/workers-go/issues/240).
- **One command that gives a verdict:** `mise run api-go:perf` builds, deploys and prints wall time and Cloudflare's CPU time per operation, in about three minutes.
- **Accepted by the tests:** `mise run api-go:check`, the live test and the soak pass on the tuned build.

## What is left, most promising first

1. **Writes.** `POST /api/notes` costs about 60 ms of CPU, several times a read. Find where it goes: the D1 write, the call to the hub, or the request body crossing into Go. Measure each alone under workerd first.
2. **The two states.** The same read costs 6 to 11 ms in some runs and 14 to 28 ms in others, minutes apart on the same build. Find out what decides it: the first thing to test is whether it is the machine the request lands on (log the `cf-ray` colo and group CPU time by it). If it is how much of the Wasm the machine has optimised, candidates are a smaller Wasm, fewer package initialisers (Huma's formats, `regexp`), and pre-initialising the Wasm at build time (Wizer-style).
3. **Get the patch into TinyGo.** The issue proposes making the threshold a build setting, or counting only finalizers that are ready. A pull request follows once a maintainer says which. Until then the patch lives in `dev wasm-build` and every project that builds with it gets it.
4. **The showcase's features.** Its reads cost 14 to 15 ms and a create 15 to 18 ms with a bearer token (HMAC check on every request). Bench each Fern feature under workerd: auth, idempotency keys, pagination, uploads.
5. **Keep one Go runtime alive across requests** instead of one per request. A workers-go change. The hard part: Cloudflare ties I/O objects (timers, sockets, promises) to the request that made them, and TinyGo's scheduler is one shared loop.
6. **Let the hub Durable Object hold the client WebSockets, with hibernation.** Then the Go Worker only serves short requests. The cost: catch-up from D1 moves into JavaScript, or the hub calls the Go Worker for it.

## Tried and dropped

- **`-gc=leaking` (no collector):** the fastest build locally, and not safe: a stream the Worker holds can live for hours and its memory would only grow. The tuned build is within 1 ms of it locally and still collects.
- **`-gc=boehm`:** fast locally; deployed to Cloudflare its requests hung (48 requests took 11 minutes).
- **`-opt=2`, `-opt=s`:** no faster than `-opt=z` on Cloudflare, and larger.
- **A 32 MB starting heap:** slower than 8 MB locally. 16 MB measured the same as 8.

## Done when

- A read fits in 10 ms of CPU in every run and a write comes close, and benchmarks.md says so with a measurement on Cloudflare.
- `mise run api-go:soak` passes, including `--idle 20`.
