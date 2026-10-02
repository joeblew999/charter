---
title: Performance plan
nav_order: 6
parent: How to help
---
# Plan: make the Go Worker cheaper per request

The goal: a Go Worker that fits Workers Free (10 ms of CPU) on every request, without giving up anything the soak checks. Ordinary requests are there: they cost what the TypeScript Worker's do. What is left is a new isolate. The numbers are in [Benchmarks](../benchmarks.md); how to run an experiment is in [How to help](../contributing.md#make-it-faster). Issues: [`perf`](https://github.com/joeblew999/charter/labels/perf).

## Done

What closed the gap for ordinary requests is listed, step by step with its numbers, in [Benchmarks](../benchmarks.md#what-closed-the-gap). Around it:

- **The tool pins the TinyGo it patches** (2026-10-02), so a project cannot build with another by accident.
- **A scratch Worker per experiment:** `mise run perf:try`, about 70 seconds, several at once.
- **One command that gives a verdict:** `mise run perf`, and `mise run compare` for both Workers side by side.
- **Accepted by the tests:** `mise run check`, the live test, the showcase test and the soak (7 of 7 clients, through a redeploy and a client drop) pass on what is deployed.

## Left, most promising first

1. **The first use of each operation in a new isolate: 15 to 30 ms,** and about 100 ms for each request of a burst there that has to start its own runtime ([#22](https://github.com/joeblew999/charter/issues/22)). Ideas:
   - warm the handlers that need a binding, with a stand-in for it during start-up;
   - start a replacement runtime after a response when none is waiting;
   - fewer package initialisers (Huma's formats, `regexp`);
   - pre-initialise the Wasm at build time (Wizer-style).
2. **Drop the patches as TinyGo ships the fixes** ([#21](https://github.com/joeblew999/charter/issues/21)): the stack fix is on TinyGo's dev branch; the collector issue is open and offers a pull request.
3. **Offer the reusing entry to workers-go** (`go/worker/go.mjs`), as an option of its generator.
4. **Let a runtime live until it is idle.** Today it is dropped when its heap is nearly full, because a collection in a full heap was so costly. With stacks reused the heap holds little that is live, so a collection may now be cheap. Measure it on Cloudflare first.
5. **A stream's own cost:** 40 to 150 ms of CPU over the life of a 15 to 60 second SSE stream. Not looked at.
6. **Let the hub hold the client WebSockets, with hibernation.** Then the Go Worker only serves short requests. The cost: catch-up from D1 moves into JavaScript, or the hub calls the Go Worker for it.

## Tried and dropped

| What | Why not |
|---|---|
| Reusing a runtime without limit, before stacks were reused | Fast at first, then collections of 65 to 293 ms of CPU, then out of memory after about 40 requests |
| `-gc=leaking` (no collector) | Not safe: a stream the Worker holds can live for hours, and its memory would only grow |
| `-gc=boehm` | Fast locally; deployed to Cloudflare its requests hung (48 requests took 11 minutes) |
| `-opt=2`, `-opt=s` | No faster than `-opt=z` on Cloudflare, and larger |
| A 32 MB starting heap | Slower than 8 MB locally. 16 MB measured the same as 8 |

## Done when

- **The first use of each operation in a new isolate fits in 10 ms of CPU,** or the docs say plainly that it cannot, with a measurement on Cloudflare.
- **`mise run soak` passes,** including `--idle 20`.
