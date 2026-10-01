# Upstream issues

Every workaround in the code carries a tag `Upstream: <owner>/<repo>#<n> (when fixed: ...)`. `mise run upstream:status` finds the tags and shows each issue's state. When an issue closes, do what its tag says, then re-run `mise run check` and the soak for that server.

## Filed, and worked around here

| Issue | Problem | Workaround here | When it's fixed |
|---|---|---|---|
| [fern-api/fern#17936](https://github.com/fern-api/fern/issues/17936) | SSE terminator matched as a substring; the Go SDK applies `[DONE]` when none is declared | `END = "[end-of-stream]"` declared as the terminator, and note bodies reject it (`src/contract.ts`) | Drop the body rule |
| [fern-api/fern#17937](https://github.com/fern-api/fern/issues/17937) | `resumable` SDKs don't reconnect after a network reset, only after a clean end | Client rule: catch, then call again with `after` (`test/soak.mjs`, the client loops in `docs/sdk.md`) | SDKs survive resets themselves; keep the loop for planned ends |
| [fern-api/fern#17938](https://github.com/fern-api/fern/issues/17938) | SSE `event:` ignored, so `event: error` is decoded as a note | `watch` never sends error events; it ends without the terminator (`src/index.ts`). The spec hook replaces oRPC's envelope with the note schema (`src/contract.ts`) | Error events could say why a stream ended; oRPC's envelope could be used as is |
| [fern-api/fern#17939](https://github.com/fern-api/fern/issues/17939) | CLI prints json/jsonl only when a stream ends; the Rust generator doesn't escape the terminator | The terminator is plain text (`src/contract.ts`); CLI latency is reported, not judged (`test/soak.mjs`) | Judge CLI latency like the SDKs' |
| [fern-api/fern#9559](https://github.com/fern-api/fern/issues/9559) | Fern rejects OpenAPI 3.2.0 (oRPC 2.0's default) | `version: "3.1.1"` (`src/specs.ts`) | Use oRPC's 3.2.0 default |
| [middleapi/orpc#2115](https://github.com/middleapi/orpc/issues/2115) | oRPC generates no AsyncAPI | `src/asyncapi.ts` (offered upstream in the issue) | Switch to oRPC's generator and delete the file |
| [tinygo-org/tinygo#3599](https://github.com/tinygo-org/tinygo/issues/3599) | TinyGo has no `reflect.StructOf`; Huma's default config installs a hook that calls it | `humaworkers.Config` leaves the hook out (`api-go/humaworkers`) | Keep Huma's default hooks |
| [syumai/workers-go#97](https://github.com/syumai/workers-go/issues/97) | workers-go can't answer a WebSocket upgrade | Go answers with a stream of lines; `api-go/worker/index.mjs` sends each as a frame | Answer the upgrade in Go and delete the adapter |
| [syumai/workers-go#220](https://github.com/syumai/workers-go/issues/220) | A Durable Object class can't be written in Go | The hub is JavaScript (`api-go/worker/hub.mjs`) | The hub can be Go |

The oRPC rows are tagged in `api/src/`; the Go Worker carries the same tags for the Fern issues (`api-go/api/`), so `upstream:status` lists both places.

## Not filed yet (drafts)

Found on 2026-10-01. No matching issue turned up in a search of each repo. Filing is outward-facing, so each needs a go-ahead; once filed, tag the code and move the row up.

### 1. TinyGo on Cloudflare: Go timers hang (the most important one)

- **Where it belongs:** tinygo-org/tinygo (`targets/wasm_exec.js`), with a note to syumai/workers-go, which ships that file.
- **What happens:** on Cloudflare Workers (not under local workerd), `time.Sleep`, `time.NewTimer` and context deadlines hang about half the time, until some other I/O happens.
- **Why:** in production the clock only moves on I/O, and after `setTimeout(d)` it has moved by exactly `d` rounded down to a whole millisecond. Measured in plain JavaScript on a deployed Worker: `setTimeout(1999.7)` advances `Date.now()` and `performance.now()` by 1999; `setTimeout(0.4)` by 0, every time. `runtime.sleepTicks` calls `setTimeout(ns / 1e6)`, a fraction, so when under a millisecond is left the scheduler re-arms a timer that never moves the clock. `ticks()` is `performance.now() * 1e6` near 1.8e18, where float rounding alone leaves such a remainder.
- **Fix:** round the delay up: `setTimeout(..., Math.ceil(Number(timeout) / 1e6))`.
- **Our workaround:** `api-go/worker/tinygo-clock.mjs` wraps `runtime.sleepTicks` to do that. Before: 6 of 10 streams with a 2 s limit were still open after 8 s. After: 16 of 16 ended on time.

### 2. TinyGo: `http.ServeMux` doesn't match method patterns

- **Where it belongs:** tinygo-org/tinygo (its `net/http`).
- **What happens:** with TinyGo 0.42.0, `mux.HandleFunc("GET /a", h)` then `GET /a` is 404; standard Go answers 200. Wildcards work (`/b/{id}` matches and `r.PathValue("id")` is right). Reproduced with `tinygo run` on the host, no Wasm involved.
- **Effect:** Huma's `humago` adapter registers `"METHOD /path"`, so every route is 404.
- **Our workaround:** `api-go/humaworkers` matches routes itself.

### 3. TinyGo: Huma needs a bigger stack than the Wasm default

- Not a bug, a setting: with the default 64 KB stack the first Huma request fails with `memory access out of bounds`. `-stack-size=256kb` works (128 KB also did). Worth a line in workers-go's README rather than an issue.

## Useful to the workers-go maintainers

- **A typed client for the whole Cloudflare API.** `mise run sdk:cloudflare` slices products (D1, KV, R2, Workers, ...) out of Cloudflare's own OpenAPI spec, and Fern generates a Go or TypeScript SDK from it ([sdk.md](sdk.md)).
- **A TypeScript Worker and a Go Worker serving the same API, with the same tests and measured side by side** ([benchmarks.md](benchmarks.md)): a ready place to try a workers-go or TinyGo change and see what it does to CPU time per request.
