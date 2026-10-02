---
title: Findings
nav_order: 11
parent: This repository
---
# Findings

Verified results only: each entry was run and checked. Newest sections last.

## Fern TypeScript SDK on Cloudflare Workers (sdk/harness)
- It runs in workerd, both under `cf dev` and deployed with `cf deploy`. All 7 pass:
  - pagination, OAuth client credentials, idempotency, SSE streaming and webhook HMAC;
  - the WebSocket client inside a Worker and from Node.
- WebSocket auth from a Worker: the SDK puts the token in a handshake header, and Workers can't send those (checked: it arrived empty, locally and on Cloudflare). Fixed by passing the token as `queryParams` to `connect()`, with the server accepting `?access_token=`. After that, 8/8 pass on Cloudflare.
- Worker-to-Worker calls on the same workers.dev zone need the `global_fetch_strictly_public` compatibility flag (else error 1042; confirmed in Cloudflare's docs). A Worker can't call its own URL.
- It needs two standard Fern options:
  - `guardProcessEnvAccess: true` (Workers have no `process`);
  - `outputSourceFiles: false` (compiled `.js` plus `.d.ts`; the raw `.ts` source fails typechecking against Workers types).
- A cf gap found along the way: there's no command for the account's workers.dev subdomain. `cf workers subdomains get` doesn't exist and exits 0 with the help text, and `cf cli search` can't find one. Our Forge-based search finds `GET /accounts/{id}/workers/subdomain`. The deploy task now records the URL cf deploy prints.

## oRPC -> OpenAPI -> Fern (api/ts/)
- An oRPC 1.15.4 contract-first Worker (Zod 4, D1) was deployed with cf. `@orpc/openapi`'s OpenAPIGenerator, with `ZodToJsonSchemaConverter` from `@orpc/zod/zod4`, produces an OpenAPI 3.1.1 spec offline from the contract.
- `fern check` passes on it. Go and TypeScript SDKs generate and pass their checks, and the Rust CLI works against the live Worker.
- oRPC's spec needs an overlay for Fern: `x-fern-pagination` plus group and method names, because operationIds come out dotted (`notes.list`).
- Numeric cursors break the CLI's `--page-all` (it stops after the first page); string cursors work. Fixed in the contract.
- orpc.dev documents oRPC **2.0 beta** (`.meta(openapi(...))`, `@orpc/zod@beta`). The stable release, 1.15.4, uses `.route({...})` and `@orpc/zod/zod4`.
- Real-time on the same API, verified live:
  - A NotesHub Durable Object (Hibernation API for WebSockets) broadcasts each new note to WebSocket clients and to oRPC SSE streams (`eventIterator`).
  - The Fern SDKs get SSE (Go and TypeScript) and the CLI gets `notes watch` (overlay: `x-fern-streaming`, plus the note schema in place of oRPC's event envelope).
  - Only the TypeScript SDK gets a WebSocket client, from a hand-written `asyncapi.yml`.
  - DO RPC carries only byte streams, and a redeploy restarts the DO, which drops live connections.
- The overlay can move into the oRPC contract. In stable 1.15.4 each route's `operationId`, `tags` and `spec` hook add the `x-fern-*` extensions and fix the SSE schema, so the only hand-written spec file left is `asyncapi.yml`. The 2.0 beta isn't needed.
- The hand-rolled hub was replaced by oRPC's `@orpc/experimental-publisher-durable-object` (1.15.4): `PublisherDurableObject` plus `DurablePublisher`.
  - SSE resume via `Last-Event-ID` works (60 s retention): a missed note was replayed on reconnect.
  - WebSocket clients get notes forwarded by the Worker as plain JSON.
  - `api:ts:live-test` passes 5/5.
- The CLI's `--page-all` with `--format jsonl` prints one line per page, not per item.

## oRPC 2.0 beta on api/ts/ (2.0.0-beta.40, verified 2026-09-30)
- The port is small. `.route({...})` becomes `.meta(openapi({...}))` from `@orpc/openapi`, `eventIterator` becomes `asyncIteratorObject`, and `@orpc/zod/zod4` becomes `@orpc/zod`. On the generator, `schemaConverters` becomes `converters` and `info`/`servers` move under `base`.
- The Durable Object publisher is in **`@orpc/cloudflare`**, not `@orpc/publisher`: `PublisherDurableObject` becomes `DurablePublisherObject`, and `{ retentionSeconds: 60 }` becomes `{ enabled: true, seconds: 60 }`.
- 2.0 generates **OpenAPI 3.2.0 by default, and `fern check` rejects it** ([fern-api/fern#9559](https://github.com/fern-api/fern/issues/9559)) ("Unsupported OpenAPI version: 3.2.0"). Pass `version: "3.1.1"`. With that, the spec matches the 1.15.4 one except that output objects now carry `additionalProperties: false`.
- Even at 3.2.0, oRPC describes SSE as its envelope (`event: message|close|error`) under `schema`, not with 3.2's `itemSchema`. The contract's `spec` hook that gives Fern the note schema is still needed. The stream's end event is now `close`; it was `done` in 1.x.
- Everything passes on 2.0:
  - `fern check`;
  - `sdk:gen api go|typescript` + `sdk:check` (Go build, vet, tests; TypeScript typecheck);
  - `api:ts:live-test` 5/5 live (SSE, WebSocket, resume, plus the TypeScript SDK);
  - the CLI's `notes list --page-all` and `notes watch`, against the deployed Worker.
- Hibernation: `DurablePublisherObject` accepts subscribers with `ctx.acceptWebSocket` (the hibernatable API) and keeps its state in SQLite plus an alarm, so the DO can sleep between events. SSE streams and `/api/notes/live` sockets are held by the Worker, which bills CPU time, not wall time, and connects to the DO as a WebSocket client.
- 2.0 doesn't remove `asyncapi.yml`: no oRPC package generates AsyncAPI (asked upstream in middleapi/orpc#2115). `@orpc/hibernation` gives hibernatable WebSockets that speak oRPC's RPC protocol, not the plain JSON messages Fern's WebSocket client expects.

## SSE and WebSockets across a redeploy (api/sse-soak.mjs, since replaced by test/soak.mjs, verified 2026-09-30, two runs)
- The test: six clients watch while a note is created every 2 s, and the Worker is redeployed mid-run (`node api/sse-soak.mjs, since replaced by test/soak.mjs <origin>`).
- The hub (NotesHub) restarted 5 s after the deploy finished in one run and 36 s after in the other. Rollout is eventually consistent, so the break comes at an unpredictable time after a deploy.
- **SSE streams end with an error, not silently.** At the hub restart the Worker's stream sends `event: error` (`INTERNAL_SERVER_ERROR`) and closes. oRPC surfaced the hub's close as an error here; the feared silent 1000/1001 close didn't happen.
- **Reconnecting with `Last-Event-ID` loses nothing:** 41/41 and 26/26 notes, 0 duplicates, for a raw client that reconnects like a browser's `EventSource`.
- **The generated TypeScript SDK's `notes.watch()` hides the error.** Its iterator just ends, as if the stream finished normally, and it doesn't reconnect. Every note after the restart was missed.
- **The generated CLI's `notes watch` exits 0 after the error event** and doesn't reconnect.
- **The generated CLI's `notes watch` doesn't stream in the json/jsonl/table formats.** It prints everything when the stream ends: `capture_output` is on for every format except `raw`/`http` (fern-cli-generator 0.44.0). `--format raw` streams, but as raw SSE lines.
- **`/api/notes/live` (WebSocket) goes silently dead.** The socket stays open, but no note arrives after the hub restart. The Worker subscribes without `onError` and never closes or resubscribes. The TypeScript SDK's `liveNotes.connect()` can't reconnect either, because nothing tells it the socket died. This was our bug in `api/ts/src/index.ts`; `follow()` fixed it (next section).
- Cloudflare doesn't compress or buffer the stream: there's no `content-encoding` with gzip, br or zstd, and the first byte arrives in about 0.1 s. The response has no `Cache-Control` header.

## The real-time system: follow() (verified 2026-09-30; design in docs/realtime.md)
- **Design:**
  - D1 is the log and the note id is the only position (`after`, the SSE `id:`, the WebSocket message `id`).
  - The hub DO only wakes followers.
  - One Worker primitive, `follow()` (`api/ts/src/follow.ts`, 10 unit tests), serves both transports. It subscribes, catches up from D1, goes live, dedupes by id, resubscribes on a hub drop, and re-reads D1 after 30 s idle.
- **`mise run api:ts:soak` passes on the deployed Worker:** 7 clients, 50/50 notes each, no gaps, no duplicates, in order, through a redeploy (hub restart) and a 6 s client drop. The clients were SSE raw (`after`), SSE `Last-Event-ID`, TypeScript SDK `notes.watch()`, Go SDK `Notes.Watch()`, CLI `notes watch`, WebSocket raw (`?after`) and TypeScript SDK `liveNotes.connect({ after })`.
- **The hub restart was real, and invisible to clients.** Workers Logs show 7 `follow: hub subscription broken, resubscribing` (`WebSocket closed unexpectedly: 1006`), one per open stream. The hub closed with 1006, not a silent 1000/1001.
- **Both specs are generated from the contract.** `api/ts/src/asyncapi.ts` (an `asyncapi()` meta plugin plus `AsyncAPIGenerator`, on oRPC 2.0's public APIs) writes `asyncapi.json`, and the hand-written `asyncapi.yml` is gone.
  - `notes.live`'s input becomes `bindings.ws.query`, and Fern's TypeScript SDK then generates `liveNotes.connect({ after })`.
  - The OpenAPI generator's `filter` keeps the channel out of `openapi.json`, which was otherwise byte-identical.
- **Fern clients read the SSE `event:` field as data** ([fern-api/fern#17938](https://github.com/fern-api/fern/issues/17938)). Against a server that sends one note and then `event: error`:
  - the TypeScript SDK yields the error object as a note;
  - the Go SDK yields a zero note (`id` 0);
  - the CLI prints it and exits 0.
  - So the Worker no longer sends error events: if `follow()` gives up, the stream just ends.
- **Fern's Go SDK stops at any note containing `[DONE]`** ([fern-api/fern#17936](https://github.com/fern-api/fern/issues/17936)). When the spec sets no terminator, the generated client still uses `DefaultSSETerminator = "[DONE]"`, matched as a substring of each event's data (`bytes.Contains`). A note with body `task [DONE] ok` ended the stream with nothing yielded. Under a resume loop the client would be stuck on that note forever.
- **Both the TypeScript and Go runtimes match the terminator as a substring of each event's data.** The contract's terminator is therefore `"\u0000"` (JSON-escaped NUL, returned by `watch` on a planned end), and note bodies reject NUL.
- **Fern's `x-fern-streaming.resumable: true` reconnects only on a clean end** ([fern-api/fern#17937](https://github.com/fern-api/fern/issues/17937)) (TypeScript 3.98.0, Go 1.64.0).
  - Against a mock server that sends notes 1–2 and ends without the terminator, one `watch()` call in each SDK reconnected with `Last-Event-ID: 2` and got 1, 2, 3.
  - The reconnect resends the original query (`after=0`), so `watch` takes the newer of `after` and `Last-Event-ID`.
  - A network reset doesn't trigger it. Through a proxy that cuts connections after 3 s, TypeScript threw `terminated` and Go returned `unexpected EOF` (Go reconnects only on `io.EOF`; TypeScript has no catch around the read). So it covers the Worker's give-up path, and the client rule still covers resets.
- **Fern's Rust generator pastes the terminator into source unescaped** ([fern-api/fern#17939](https://github.com/fern-api/fern/issues/17939)). With a terminator containing `"` or `\`, the CLI's SDK doesn't compile (`Some(""\u0000"".to_string())`). The terminator is therefore plain text, `[end-of-stream]`, and note bodies reject it.
- **The CLI's `notes watch` prints json/jsonl only when a stream ends** ([fern-api/fern#17939](https://github.com/fern-api/fern/issues/17939)), with generators 0.44.0 and 0.45.1 (`--format raw` streams). It still receives every note, at p50 about 9 s behind with 15 s streams.


## Go on workers-go: Huma -> OpenAPI + AsyncAPI -> Fern (api/go/, verified locally 2026-10-01)

Everything in this section ran on this machine: natively, and as TinyGo Wasm under workerd (`cf dev`). The results on Cloudflare itself are in the next section.

- **Versions:** Huma 2.39.1, workers-go 0.36.0, TinyGo 0.42.0 (binaryen 133), Go 1.27.1, workerd through cf 1.0.0-beta.5.
- **Huma runs under TinyGo on workerd,** with three things done on our side (`api/go/humaworkers`, `mise run api:go:build`):
  - `-stack-size=256kb`. With TinyGo's default stack, the first request fails with `memory access out of bounds`; 128 KB also worked, 64 KB didn't.
  - No `SchemaLinkTransformer` hook (`huma.DefaultConfig` installs it). It calls `reflect.StructOf`, and TinyGo answers `panic: unimplemented: reflect.StructOf()`.
  - Our own route matching. TinyGo's `http.ServeMux` doesn't match method patterns: `"GET /a"` is 404 (`/b/{id}` wildcards work; reproduced with `tinygo run` on the host). Huma's `humago` adapter registers exactly those, so every route is 404.
- **What then works under TinyGo:** query, header and body parsing; validation from struct tags (`minimum`, `maximum`, `default`, `minLength`, `pattern`, so `regexp` too); `Resolve` for custom rules; 422 `application/problem+json` errors with locations; `StreamResponse`; and generating the OpenAPI document inside the Worker.
- **Size:** `app.wasm` is 2.41 MB, 848 KB gzipped (the Workers Free limit is 3 MB gzipped). An empty workers-go handler was 863 KB, 326 KB gzipped.
- **Request time, local workerd, wall clock:** about 14 ms for `/api/hello`, about 28 ms for a D1 list of 20 notes, about 25 ms for `/api/openapi.json`. An empty workers-go handler took about 8 ms. workers-go starts a fresh Go runtime per request, so `humaworkers` registers only the operation a request matches. CPU time on Cloudflare isn't measured yet.
- **Both specs come from the Go contract** (`api/go/api/contract.go`), nothing hand-written:
  - OpenAPI 3.1.0 by Huma, with `x-fern-sdk-*`, `x-fern-pagination` and `x-fern-streaming` set through `Operation.Extensions`, and the SSE response declared as `text/event-stream` with the `Note` schema.
  - AsyncAPI 3.0.0 by `api/go/asyncapi` (a port of `api/ts/src/asyncapi.ts` onto Huma's operations and schema registry). The channel is a hidden Huma operation, so it stays out of OpenAPI; its query parameters become `bindings.ws.query`.
- **Fern accepts them.** `fern check --api api-go` passes. The Go SDK (build, vet, tests against WireMock), the TypeScript SDK and the Rust CLI all generate and build from them.
- **The SDK surface is the oRPC one.** `TestSameSurfaceAsTheORPCContract` compares each operation's id, tags, summary, parameters with their constraints, `x-fern-*` extensions and 200 media types, plus the channel's address, summary, query binding and receive operation, with `sdk/fern/apis/api/`. One thing had to be matched by hand: Huma writes `format: int64` for a Go `int`, and Fern's Go SDK then types the parameter `*int64` instead of `*int`, so the contract uses `int32`.
- **The real-time design carries over unchanged.** `follow.Follow` is `follow()` in Go, with the same ten tests plus one, passing under the race detector. The hub is a small JavaScript Durable Object with hibernating WebSockets.
- **workers-go can't answer a WebSocket upgrade** (a 101 from Go has no `webSocket`) and has no WebSocket client. So Go answers the upgrade with a stream of lines and `api/go/worker/index.mjs` sends each as a frame; Go subscribes to the hub through `syscall/js`.
- **The same tests pass against the Go Worker, locally:**
  - `test/live-test.mjs`: 3/3 (SSE, WebSocket through the hub, resume with `Last-Event-ID`), against the Wasm under workerd and against the native build.
  - `test/sdk-live-test.mjs` with the TypeScript SDK generated from the Go specs: 2/2 (`notes.watch()`, `liveNotes.connect()`).
  - `test/soak.mjs --sdk api-go --no-deploy`: 7/7 clients, 27/27 notes each, no gaps or duplicates, in order, through a 6 s client drop. The SDKs and the CLI were the ones generated from the Go specs. The hub-restart scenario needs a deploy and hasn't run.
- **The two servers are interchangeable to clients.** The SDKs and CLI generated from the *oRPC* specs (`sdk/out/api`) pass the same tests against the *Go* server (native build): SDK live test 2/2, soak 7/7 through a client drop.
- **The SSE wire format is byte for byte the oRPC Worker's** (a comment line, then `event: message`, `retry`, `id`, `data` per note, then `event: close` with the terminator), checked against a capture from the deployed oRPC Worker.
- **The same Go code runs natively** (`mise run api:go:run`): REST, SSE and the WebSocket, on an in-memory store.

## The Go Worker on Cloudflare (orpc-api-go, verified 2026-10-01)

Deployed with `mise run api:go:deploy` to https://orpc-api-go.gedw99.workers.dev, with its own D1 database (`orpc-api-go-db`) and the same schema.

- **`mise run api:go:live-test`: 5/5.** Raw SSE, raw WebSocket through the hub Durable Object, SSE resume with `Last-Event-ID`, and the TypeScript SDK's `notes.watch()` and `liveNotes.connect()` (the SDK generated from the Go specs).
- **`mise run api:go:soak`: 7/7 clients, 38/38 notes each,** no gaps, no duplicates, in order, through a redeploy (the hub restart) at 30 s and a 6 s client drop. The SDKs and the Fern CLI were the ones generated from the Go specs. SSE streams ended on schedule every 15 s (9 connections per client).
- **`mise run api:go:soak --idle 20`: 7/7.** After 20 quiet minutes one note reached every client at once; the WebSocket clients held one connection the whole time.
- **Go timers hang on Cloudflare without a fix, and not under local workerd.** The first soak there failed for the Fern CLI (5/37), because streams never ended at their limit: a stream asked for 2 s was still open after 40.
  - Go's timers were fine in isolation (`time.Sleep`, `time.NewTimer`, a context deadline and a `select` all fired at 2 s in a probe).
  - The cause is the production clock, measured in plain JavaScript on the deployed Worker: after `setTimeout(d)` both `Date.now()` and `performance.now()` have moved by `d` rounded down to a whole millisecond (`1999.7` gives 1999; `0.4` gives 0, however often it's repeated). TinyGo's scheduler sleeps with `setTimeout(ns / 1e6)`, so with under a millisecond left it re-arms a timer that never moves the clock.
  - 6 of 10 streams with a 2 s limit hung. With `api/go/worker/tinygo-clock.mjs` (round every TinyGo sleep up to a whole millisecond), 16 of 16 ended on time, and the soak above passed.
- **The hub Durable Object hibernates** (Cloudflare's GraphQL analytics, `durableObjectsPeriodicGroups`, namespace `orpc-api-go_NotesHub`). In the hour that held the soak and the 20-minute idle run, with client streams subscribed for about half of it, the hub's active time was 5.7 s; it sent 804 WebSocket messages and took 565 requests. The next hour, mostly idle: 0.2 s.
- **Everything Fern's clients do works against the Go Worker,** with the clients generated from the Go specs:
  - the CLI: `meta hello`, `notes create`, `notes watch --after` (the notes after the cursor, then the terminator), and `notes list --page-all --limit 30`: 4 pages, 102 notes, 102 unique ids;
  - the Go SDK's auto-paging iterator: 102 notes;
  - invalid input comes back through the CLI as Huma's 422 with its details.
- **MCP works on Cloudflare.** `node test/mcp-test.mjs https://orpc-api-go.gedw99.workers.dev`: 29/29, twice, with the official TypeScript client (`@modelcontextprotocol/client` 2.2.0), in both the stateless (2026-07-28) and the handshake (2025-11-25) mode. Run it a few seconds after a deploy: the first try hit the previous version, which had no `/api/mcp`.
- **CPU time per request was 40 to 70 ms with TinyGo as it is, against 1 to 3 ms for the oRPC Worker** (Workers Logs, median): hello 55 ms, a D1 list 71 ms, a 404 38 ms, a create about 265 ms; oRPC hello 1 ms, list 3 ms.
- **The cause was TinyGo's collector, and a build setting removes most of it.** TinyGo runs a full collection whenever the scheduler is idle and 32 objects with finalizers were made (`finalizerGCThreshold` in `src/runtime/gc_finalizer.go`); workers-go makes one per JavaScript value. With that threshold at 0 in a copy of the runtime and an 8 MB starting heap (`dev wasm-build`), the same Worker measured, per run of 20 requests: hello 6 to 8 ms or 14 to 19 ms, the list 8 to 11 or 22 to 29, a 404 5 to 7 or 12 to 17, a create 61 to 66. A whole run is in the lower or the higher range; why is not established. `mise run api:go:check`, the live test and the soak pass on that build. Filed as tinygo-org/tinygo#5800. The tables are in [benchmarks.md](benchmarks.md).
- **`-opt=2` is no faster than `-opt=z` on Cloudflare** with the tuned build, and the Wasm is larger.
- **Reusing a Go runtime for the next request takes a hello to 3 ms of CPU, the list to 7, a create to 10 to 13 and a 404 to 2** (medians of 20, two runs, each right after a deploy; p99 9 to 20 ms). `api/go/worker/go.mjs` keeps a runtime that has finished a response; `transport.Run` keeps the Go program alive.
  - Per request the CPU time is 2 to 3 ms, and 5 to 13 ms for the one in three that starts a runtime.
  - Before, every second request cost double (5, 13, 5, 13 or 12, 30, 12, 30 in Workers Logs), and a new isolate double again. Both are gone.
  - A runtime cannot be reused without limit: TinyGo allocates a stack per goroutine and per callback from JavaScript (about 13 for a hello, 30 for a create) and its collector rarely frees them. Unlimited reuse gave collections of 65, 148, 175 and 293 ms and then a 503 from memory. So a runtime is dropped when its heap has no room for another request (`transport.Serve` sets the binding's `full`).
  - With TinyGo as it is and 256 KB stacks, 9 hellos left 23 MB in use after 14 collections (`runtime.ReadMemStats` under `cf dev`).
  - On that build: `mise run check` passes, `mise run api:go:live-test` 34 of 34 on Cloudflare, `mise run api:go:soak` 7 of 7 clients with 45 of 45 notes through a redeploy and a client drop, and `test/showcase-test.mjs` passes against the deployed showcase, which measured a list at 2 ms and a create at 5.
  - `mise run api:go:soak --idle 20` on it: 7 of 7 after 20 quiet minutes.
- **Reusing goroutine stacks inside TinyGo takes a hello to 1 ms of CPU, the list to 3, a create to 4 and a 404 to 1** (medians of 20, right after a deploy; p99 2 to 19 ms). It is a second patch that `dev wasm-build` applies to its copy of TinyGo's runtime (`src/internal/task/task_asyncify.go`): a finished goroutine's stack is kept for the next one. Filed with the patch as tinygo-org/tinygo#5801.
  - Under `cf dev`, a hello allocated 38 KB instead of 1.7 MB (128 KB stacks) and a create about 100 KB instead of 4 MB; one runtime served 30 hellos and 2 creates with no collection.
  - On Cloudflare a hello's CPU time, request by request: 1 2 2 2 1 2 1 1 2 1 1 2.
  - The showcase Worker on the same build: a list 1 ms, a create 2 ms, a 404 1 ms.
  - During the soak (7 open streams, a note every 2 s) a create cost 3 to 6 ms; on the build before it cost 50 to 115 ms for most of them.
  - **The first request a new isolate serves costs 40 to 100 ms.** 8 connections opened at once: the first request of each cost 41 to 77 ms, the rest 1 to 2. Not new: it was 90 to 170 ms with TinyGo and workers-go as they come.
  - On that build: `mise run check` passes; locally 692 requests 8 at a time with 12 cut-off streams, all 200; on Cloudflare 1,016 requests 8 at a time with 16 cut-off streams, all 200, `mise run api:go:live-test` 34 of 34, `mise run api:go:soak` 7 of 7 with 46 of 46 notes through a redeploy and a client drop, and `test/showcase-test.mjs` passes against the deployed showcase.
  - In the soak's logs 6 WebSocket requests ended as `exception`, as 7 did on the build before: sockets cut by the redeploy and the client drop.
  - `mise run api:go:soak --idle 20` on it: 7 of 7 after 20 quiet minutes.
- **Starting Go runtimes while the Worker's module loads takes the first request in a new isolate from 54 to 68 ms of CPU to 7 to 11** (2026-10-02, three deploys each; `go.warm` in `api/go/worker/go.mjs`).
  - Step by step: a runtime started at module load, 32 to 37 ms; answering the OpenAPI route during start-up (`transport.Run`), 19 to 20; and the hello route, 7 to 11.
  - Cloudflare refuses crypto randomness at module load ("Disallowed operation called within global scope"), and TinyGo's `runtime.hardwareRand` asks for its seed in `_start`. With that one call served from `Math.random`, start-up reaches `workers.Ready()` there. Reading a `Response` body at module load is refused too, which is why the warm-up requests are made inside Go.
  - Requests of a burst correlated by ray id: one served by a warm runtime cost 4 to 19 ms, one that started its own 89 to 111 when four did so at once. Alone, a start costs 9 to 32 ms.
  - The first list in a new isolate still costs 18 to 33 ms and the first create 14 to 25.
  - Under local workerd none of the module-load limits are enforced: a warm start that works there says nothing about Cloudflare.
  - `mise run check` passes with it, and both Go Workers answer their first request from a warm runtime on Cloudflare (`x-go-runtime: warm`).
- **`-gc=boehm` hangs on Cloudflare.** Locally it was the fastest build with a real collector; deployed, 48 requests took 11 minutes. The default collector stays.
- **CI runs it all on Linux:** TinyGo and binaryen install through mise on `ubuntu-24.04`, and `api:go:check` (including the Wasm under workerd) passes there.

## MCP from the Huma contract (api/go/humamcp, verified locally 2026-10-01; on Cloudflare: see the section above)

Everything here ran on this machine: natively, and as TinyGo Wasm under workerd (`cf dev`). Nothing ran on Cloudflare itself. The design and its limits are in [mcp.md](mcp.md).

- **Versions:** MCP revisions 2026-07-28, 2025-11-25 and 2025-06-18; clients `@modelcontextprotocol/client` 2.2.0, `@modelcontextprotocol/sdk` 1.31.0 and `@modelcontextprotocol/inspector` 2.9.0 (CLI); Huma 2.39.1, TinyGo 0.42.0, Go 1.27.1, workerd through cf 1.0.0-beta.5.
- **An MCP server needs no MCP SDK.** `api/go/humamcp` is the Streamable HTTP transport written out: JSON-RPC over one POST, one `application/json` answer, no state. It serves `/api/mcp` from the same Huma operations as REST, in the Worker and in the native build.
- **The official Go MCP SDK doesn't compile with TinyGo.** `tinygo build -target wasm` of a minimal server on `github.com/modelcontextprotocol/go-sdk` v1.8.0 stops in the standard library's `internal/runtime/maps` (`undefined: abi.MapType`), reached through `hash/maphash`, which the SDK's `github.com/google/jsonschema-go` imports. That is the first error, not necessarily the only one; standard Go builds the same program. mcp-go was not tried.
- **MCP became stateless in revision 2026-07-28** (no `initialize`, no sessions; every request carries its version in `_meta`; `server/discover`), which is what workers-go needs: a fresh Go runtime per request can hold no session. The endpoint also answers the handshake revisions without giving a session id, which they allow.
- **Real clients work against it, in both eras, natively and under workerd.** `test/mcp-test.mjs` (the official TypeScript client 2.2.0, pinned to 2026-07-28, then in its `legacy` mode at 2025-11-25, then `auto`, which picked 2026-07-28): 29/29 against the native build and 29/29 against the Wasm under workerd. By hand under workerd: the v1 SDK 1.31.0 (negotiated 2025-11-25, no session id; list, create, list, a refused call, an unknown tool, ping) and the Inspector CLI (`tools/list`, `tools/call listNotes`).
- **A tool call is the REST call.** It goes through `humaworkers.API.ServeHTTP`, so Huma's validation, `Resolve` and the handler are the same code. `listNotes` over MCP returned the bytes `GET /api/notes` returned. A refused call is a tool result with `isError` and Huma's problem as text (`"location":"query.limit"`); an unknown tool is JSON-RPC error -32602, an unknown method -32601, malformed JSON -32700.
- **Tools:** `hello`, `listNotes`, `createNote`. `watchNotes` (SSE) and `liveNotes` (WebSocket) are left out: a tool call is one request and one answer.
- **Size:** `app.wasm` went from 2,414,932 B (856,794 B gzipped) to 2,468,047 B (874,628 B gzipped): +17.8 KB gzipped.
- **Request time, local workerd, wall clock, p50 of 100–200 sequential requests:** an MCP call costs what a REST POST costs.
  - `tools/call createNote` 43.5 ms against `POST /api/notes` 41.9 ms; the same two refused with 422: 27.5 ms against 27.3 ms.
  - `tools/call hello` 27 ms, `tools/list` 29 ms, `ping` (which does nothing) 27 ms; `GET /api/hello` 12 ms, `GET /api/openapi.json` 19 ms. So about 15 ms of an MCP call is what any POST with a body costs here, not MCP. Why a POST costs that much wasn't looked into.
- **`go test` missed one thing and the combined check found it.** In the native build (one process for all requests), routes are registered as requests need them, so `tools/list` came out in the order of earlier calls (`hello, createNote, listNotes` after the live test had created a note). `humaworkers.Operations()` now returns the routes' order whatever came before, and registration is under a mutex: the race detector reported unsynchronised registration when the first requests arrive together, which the native build could already do before MCP.
- **Seen once each and not reproduced** (so not explained): the first run of the client test against a just-started `cf dev` ended with `ECONNRESET` at its first tool call (after 5 passing checks), and one of eight back-to-back runs took about five minutes instead of one second (all its checks passed). After that: 5 runs against a just-started server (3 of them with no local state and no Vite cache), 24 runs against a running one, and about 3,000 single requests, with no failure and no request over one second.
- **Not run:** anything on Cloudflare; an MCP host with a model behind it (Claude, an IDE); authorization.

## The first release, v0.1.0 (verified 2026-10-01)

Cut with `git tag v0.1.0 && git push origin v0.1.0` on main, after `api-check`, `sdk-check` and `dev-check` passed there. Both release workflows succeeded.

- **The GitHub Release has 12 files:** the `dev` tool for linux and darwin (amd64, arm64); for each API the Go and TypeScript SDK sources and its two specs; and each API's Fern CLI for linux/amd64.
- **The Go module tags were added by the workflow:** `api/go/v0.1.0` and `dev/v0.1.0`, on the same commit.
- **Checked from outside the repo, in an empty folder:**
  - the downloaded `dev-darwin-arm64` runs;
  - `go run github.com/joeblew999/orpc-api/dev@v0.1.0 help` runs;
  - `... dev@v0.1.0 workflows -into <dir>` writes `api-check.yml`, `api-deploy.yml`, `sdk-check.yml` and `sdk-release.yml` there;
  - a new module with `go get github.com/joeblew999/orpc-api/api/go@v0.1.0` imports and uses `humaworkers`, `asyncapi`, `follow` and `humamcp`.
- **The two servers are interchangeable to clients, in both directions, on Cloudflare.** The SDKs and CLI generated from the Go specs pass the SDK live test (2/2) and a short soak (7/7, nothing missing, no duplicates, through a client drop) against the deployed *oRPC* Worker; the oRPC-spec ones had passed against the Go server.

## The showcase, contract first in oRPC (sdk/harness, verified locally 2026-10-01)

Everything in this list ran on this machine, the Worker under `cf dev`. It was then deployed (`mise run sdk:harness:deploy`), and `mise run sdk:harness:test -remote` passes 9/9 on Cloudflare against the oRPC implementation: SSE, the multipart upload, idempotent create, OAuth client credentials, pagination, the webhook signature, and the WebSocket client inside a Worker and from Node. How it is built is in [sdk.md](sdk.md#the-orpc-showcase).

- **Versions:** oRPC 2.0.0-beta.40, Zod 4.6.5, Fern 5.140.0 (Go SDK 1.64.0, TypeScript SDK 3.98.0, CLI generator 0.44.0), workerd through cf 1.0.0-beta.5.
- **One oRPC contract now gives the whole showcase:** OAuth client credentials with a form-encoded token endpoint, idempotency, cursor pagination, an SSE stream, a multipart upload, a webhook with an HMAC signature, a WebSocket both ways, and audiences. `openapi.json` and `asyncapi.json` are generated from it, and the hand-written `asyncapi.yml` is deleted.
- **Fern makes the same SDKs from the generated specs.** All five groups were generated from the hand-written specs and from the generated ones and compared file by file. They differ in two things only: `NoteEvent` has a new optional `auth`, and the default WebSocket URL is the deployed harness where it was `api.example.com`. (The CLI also carries a copy of the spec.)
  - What changed in the spec but in no SDK: OpenAPI 3.1.1, `additionalProperties: false` on response objects, safe-integer bounds, `allowEmptyValue` and `allowReserved` on query parameters, `contentEncoding` on the file, a `name` on AsyncAPI messages.
  - `sdk/harness/test/surface.test.ts` holds the hand-written specs' surface as a fixture and compares the generated specs with it (`mise run showcase:test`).
- **The harness test passes against the oRPC server: 9/9.** The eight checks that passed against the plain mock, unchanged, plus a new one for the multipart upload. The eight also passed with the SDK generated from the old hand-written specs, before any SDK was regenerated.
- **The server also runs in Node,** as a fetch function: `sdk/harness/test/showcase.test.ts` calls every route and the channel without a Worker (7 tests), including a message the contract rejects.
- **`fern check` passes;** the Go SDK builds, vets and passes its tests; the TypeScript SDK and the public one typecheck; the compiled TypeScript SDK and the CLI generate. The CLI was not built.
- **oRPC's handler reads more than its generator writes.** `OpenAPIHandler` decodes `application/x-www-form-urlencoded` and `multipart/form-data` bodies into the contract's input. The generator writes `multipart/form-data` when the input has a `z.file()`, but a form-encoded body always comes out as `application/json`.
- **oRPC's generator has no webhooks, no per-operation `security`, and the contract has no document level.** `base` on the generator carries `security`, `securitySchemes` and document-level `x-fern-*`. Webhooks are written by our `openapiSpec({ webhooks })` from a contract of webhook procedures.
- **Zod names the SDK's types.** `z.object(...).meta({ id: "Note" })` becomes `components.schemas.Note`, referenced everywhere it is used, and Fern names the type `Note`. A webhook payload has to be converted as output to share it: as input the same schema lacks `additionalProperties: false`, and oRPC would name a second one `NoteInput`.
- **A WebSocket both ways is one procedure:** input `asyncIteratorObject(Subscribe)`, output `asyncIteratorObject(NoteEvent)`. The Worker feeds the socket's messages in as a stream and sends out what the handler yields; the contract validates each message in both directions. `api/ts/src/asyncapi.ts` writes the input as a `send` operation, and Fern's TypeScript client gets a typed `sendSubscribe`. `sdk/fern/apis/api/asyncapi.json` is unchanged, byte for byte.
- **Fern's SSE reader accepts oRPC's stream as it is.** The chat stream is `event: message` per chunk and a last `event: close` with no data; the TypeScript SDK yielded the chunks and nothing else.
- **Two installs of oRPC work as one.** The contract is built with `sdk/harness/node_modules` and the generators run from `api/ts/node_modules` (the same versions). oRPC recognises its objects by name and by `Symbol.for`, not by class identity, so spec generation and the typecheck both pass.
- **Fern ignores an AsyncAPI server's `pathname`.** With `host` and `pathname: /api/mock`, the TypeScript, Go and Rust output all have `wss://<host>` as the default WebSocket URL.
- **Not run:** anything on Cloudflare; the Rust CLI build; the Go SDK against the server.

## The showcase in Go: every Fern feature from a Huma contract (api/go/showcase, verified locally 2026-10-01; not deployed)

Everything here ran on this machine: natively, and as TinyGo Wasm under workerd (`cf dev`). Nothing ran on Cloudflare. How it is built, feature by feature, is in [showcase-go.md](showcase-go.md).

- **Versions:** Huma 2.39.1, workers-go 0.36.0, TinyGo 0.42.0, Go 1.27.1, Fern 5.140.0 (Go SDK 1.64.0, TypeScript SDK 3.98.0, CLI generator 0.44.0), workerd through cf 1.0.0-beta.5.
- **One Go contract gives the whole showcase:** OAuth client credentials with a form-encoded token endpoint, idempotency, cursor pagination, an SSE stream, a multipart upload, a webhook with an HMAC signature, a WebSocket both ways, and audiences. `sdk/fern/apis/showcase-go/openapi.json` and `asyncapi.json` are generated from it, and `fern check` passes.
- **`test/showcase-test.mjs` passes 12/12 against the Go server, natively and as Wasm under workerd** (`mise run showcase-go:check`). It is the TypeScript SDK generated from the Go specs, run from Node over the network:
  - auto-pagination over three requests; the OAuth token fetched once, as a form, and sent with every request; an idempotent create (the same key twice gives the same note); an SSE stream of typed chunks; a 20 KB multipart upload with a form field;
  - the webhook: the server posts `noteCreated` to the test's receiver, and the SDK's `WebhooksHelper.verifySignature` accepts it and rejects it with another secret or a changed body;
  - the WebSocket: the SDK's client sends a typed `subscribe` and gets the topic's three events with the bearer token the server saw; the same over a header-less socket with `?access_token=`;
  - audiences: the generated public SDK has `notes` and `auth`, and no `files`;
  - the server enforces the contract: no token or a forged one is 401; a socket without a token is refused; a message the contract rejects closes the socket with 1008.
- **The two showcases are interchangeable to the SDKs.** The same program passes against the oRPC showcase under `cf dev` with its own SDK (9/9: `--open` leaves out the token checks, and it sends no webhook). Each server also passes with the SDK made from the other's specs: the oRPC-spec SDK against the Go server (12/12, native), the Go-spec SDK against the oRPC server (9/9).
- **The SDK surface is the oRPC showcase's.** `TestSameSurfaceAsTheORPCShowcase` compares the two generated specs: operations, parameters, request and response bodies by media type with their shapes, `x-fern-*` extensions, security, the webhook, the channel, its operations and its messages. Two differences are named in the test: `limit` has bounds and a default in Go, and the channel's `access_token` query parameter is declared in Go. Both showcases use the same overlay.
- **Fern's generators accept the Go specs.** The Go SDK builds, vets and passes its tests; the TypeScript SDK and the public one typecheck; the compiled TypeScript SDK and the CLI generate. The CLI was not built.
- **Huma says most of it natively.** The document's `security`, `securitySchemes` and `x-fern-*` are fields of its config; `security: []` and `x-fern-*` are fields of an operation; `OpenAPI.Webhooks` is a field (filled by hand in `api/go/showcase/spec.go`); the `contentType` tag writes a form-encoded body into the spec. What was added: a format that decodes a form (`humaworkers.WithForm`), and `send` operations in the AsyncAPI generator (`asyncapi.SendOperation`). The notes API's `asyncapi.json` is unchanged, byte for byte.
- **`crypto/hmac` and `crypto/sha256` work under TinyGo.** The access token is an expiry signed with HMAC, made by one request and checked by the next, and the webhook is signed, all in the Wasm under workerd.
- **Huma's typed multipart form doesn't run under TinyGo.** `huma.MultipartFormFiles[T]` panics with `unimplemented: (reflect.Value).MethodByName()` (tinygo-org/tinygo#3862). The plain `multipart.Form` as `RawBody` works, with the schema declared on the operation.
- **A multipart upload over 8 KB failed under workerd** with `cannot read multipart form: open /tmp/multipart-...: file does not exist`. Huma's adapter keeps 8 KB in memory (`humago.MultipartMaxMemory`) and writes the rest to a temporary file. With the limit at 32 MB (`humaworkers` sets it), 100 KB and 5 MB uploads passed; 5 MB took 2.4 s under workerd, 5 ms natively.
- **An outgoing request from Go works under workerd:** the webhook is sent with workers-go's `fetch` client as an `*http.Client`, to the test's receiver on localhost.
- **A WebSocket can carry client messages to Go without Go holding the socket.** The adapter gives each text frame to Go as a `POST` to the upgrade's URL, with the upgrade's headers, and sends the lines of the answer as frames. Go asks for it with `X-Websocket-Messages: post`; a 204 to the upgrade means no feed. Under workerd and natively: two messages on one socket were answered in order; a message Huma refuses (422) closed the socket with 1008; a binary frame closed it with 1003.
- **The notes API is unchanged, but for one thing in the native build.** `mise run api:go:check` passes as before (the live test and the MCP test, natively and under workerd), and its Wasm is 2,468,337 B, 874,769 B gzipped (141 B more). The native transport used to close the notes socket with 1008 (`unexpected data message`) when a client sent a frame; the Worker's adapter ignores it. The native one now ignores it too.
- **Size:** the showcase's `app.wasm` is 2,497,375 B, 884,341 B gzipped.
- **`cf dev` drops a refused WebSocket upgrade.** Natively a socket without a token gets Go's 401. Under `cf dev` the connection closes with no response (curl: an empty reply). What Cloudflare itself does was not checked.
- **Fern's Go generator writes a test that doesn't compile** for a form-encoded OAuth token endpoint whose request schema is named (`$ref`): `undefined: showcase.Request` in `auth/oauth_wire_test` (fern-go-sdk 1.64.0). Huma names body schemas; with this one written inline the same test compiles (`showcase.GetTokenRequest`) and passes. Not filed.
- **`go test` covers the rest:** every operation against five kinds of bad token, the cursor's edges, the webhook's exact body and signature, the WebSocket through the native adapter, and the adapter itself (`api/go/transport`), under the race detector.
- **Not run:** anything on Cloudflare (the Worker `orpc-showcase-go` is not deployed); the Go SDK against the server; the Rust CLI build; the SDK inside a Worker against the Go server.

## The Go showcase on Cloudflare (orpc-showcase-go, verified 2026-10-01)

Deployed with `mise run showcase-go:deploy` to https://orpc-showcase-go.gedw99.workers.dev. `node test/showcase-test.mjs https://orpc-showcase-go.gedw99.workers.dev showcase-go` (the TypeScript SDK Fern generated from the Go specs, run from Node) passes every check it runs there: pagination, idempotent create, OAuth client credentials, the SSE stream, the multipart upload, the webhook signature helper, the WebSocket both ways (the token as a header and as `?access_token=`), audiences, 401 without a token or with a forged one, and a refused or rejected WebSocket. The one check not run on Cloudflare is the server posting a webhook to the test's own receiver, which only exists on the machine running the test.
