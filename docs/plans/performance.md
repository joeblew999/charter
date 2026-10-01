# Plan: make the Go Worker cheaper per request

Not started. The numbers are in [../benchmarks.md](../benchmarks.md): 40 to 70 ms of CPU per request on Cloudflare against 1 to 3 ms for the oRPC Worker, most of it start-up and garbage collection paid on every request.

The goal: a Go Worker that fits Workers Free (10 ms CPU), or gets close, without giving up anything the soak matrix checks.

## Set it up first, so many agents can try things

1. **One command that gives a verdict.** `go run ./dev bench <url>` exists for wall time. Add CPU time from the Workers Observability API to it, so a run prints a table like the one in benchmarks.md with no dashboard.
2. **A scratch Worker per experiment** (`cf deploy --mode <name>`, as `sdk/harness` does with `--mode api`), so experiments never touch `orpc-api-go` and can run side by side.
3. **A fixed acceptance:** `mise run api-go:check`, then the live test and a short soak against the scratch Worker. A faster build that fails those is not a result.

## Ideas, most promising first

1. **Let the hub Durable Object hold the client WebSockets, with hibernation.** Today the Worker holds each client's socket and subscribes to the hub (the same shape as the oRPC Worker). If the hub held client sockets directly, the Go Worker would only serve short requests, and `-gc=leaking` (half the time, a quarter for D1 reads) would be safe for everything except SSE. The cost: the hub must then send catch-up from D1 itself, so that logic moves into JavaScript, or the hub calls the Go Worker for it. SSE can't hibernate on Cloudflare either way; it stays finite and resumable, and a finite stream bounds what a no-collector build can leak.
2. **Two Wasm builds from one source.** A no-collector build for request/response routes and a collected build for streams, chosen by the Worker's entry from a list of stream routes that `cmd/spec` writes from the contract. Doubles the upload (about 1.5 MB gzipped, under the limit) and the build time.
3. **Find out what start-up spends its time on.** The 404 runs no handler and costs 38 ms. Candidates: Wasm instantiation of a 2.4 MB module per request, Go package initialisers (Huma, `regexp`, `encoding/json`), collector work during them. Measure an empty workers-go handler on Cloudflare first: that is the floor for any Go Worker.
4. **Keep one Go runtime alive across requests** instead of one per request. This is a workers-go change (a fork, then upstream). The hard part: Cloudflare ties I/O objects (timers, sockets, promises) to the request that made them, and TinyGo's scheduler is one shared loop.
5. **Snapshot the initialised runtime** (Wizer-style pre-initialisation of the Wasm), so package initialisers run at build time. Needs checking against TinyGo's asyncify scheduler.
6. **Why does `-gc=boehm` hang on Cloudflare?** Locally it was the best safe option. It may be the same clock behaviour that broke timers (see [../upstream.md](../upstream.md)), in the collector's own code.
7. **Less to initialise:** a Huma build without the parts a Worker doesn't use (formats, docs renderers), and validating `pattern` without `regexp`.

## Done when

- benchmarks.md has a row for the chosen build, measured on Cloudflare.
- `mise run api-go:soak` passes, including `--idle 20`.
