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

## oRPC -> OpenAPI -> Fern (api/)
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
  - `api:live-test` passes 5/5.
- The CLI's `--page-all` with `--format jsonl` prints one line per page, not per item.

## oRPC 2.0 beta on api/ (2.0.0-beta.40, verified 2026-09-30)
- The port is small. `.route({...})` becomes `.meta(openapi({...}))` from `@orpc/openapi`, `eventIterator` becomes `asyncIteratorObject`, and `@orpc/zod/zod4` becomes `@orpc/zod`. On the generator, `schemaConverters` becomes `converters` and `info`/`servers` move under `base`.
- The Durable Object publisher is in **`@orpc/cloudflare`**, not `@orpc/publisher`: `PublisherDurableObject` becomes `DurablePublisherObject`, and `{ retentionSeconds: 60 }` becomes `{ enabled: true, seconds: 60 }`.
- 2.0 generates **OpenAPI 3.2.0 by default, and `fern check` rejects it** ([fern-api/fern#9559](https://github.com/fern-api/fern/issues/9559)) ("Unsupported OpenAPI version: 3.2.0"). Pass `version: "3.1.1"`. With that, the spec matches the 1.15.4 one except that output objects now carry `additionalProperties: false`.
- Even at 3.2.0, oRPC describes SSE as its envelope (`event: message|close|error`) under `schema`, not with 3.2's `itemSchema`. The contract's `spec` hook that gives Fern the note schema is still needed. The stream's end event is now `close`; it was `done` in 1.x.
- Everything passes on 2.0:
  - `fern check`;
  - `sdk:gen api go|typescript` + `sdk:check` (Go build, vet, tests; TypeScript typecheck);
  - `api:live-test` 5/5 live (SSE, WebSocket, resume, plus the TypeScript SDK);
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
- **`/api/notes/live` (WebSocket) goes silently dead.** The socket stays open, but no note arrives after the hub restart. The Worker subscribes without `onError` and never closes or resubscribes. The TypeScript SDK's `liveNotes.connect()` can't reconnect either, because nothing tells it the socket died. This was our bug in `api/src/index.ts`; `follow()` fixed it (next section).
- Cloudflare doesn't compress or buffer the stream: there's no `content-encoding` with gzip, br or zstd, and the first byte arrives in about 0.1 s. The response has no `Cache-Control` header.

## The real-time system: follow() (verified 2026-09-30; design in .plans/realtime.md)
- **Design:**
  - D1 is the log and the note id is the only position (`after`, the SSE `id:`, the WebSocket message `id`).
  - The hub DO only wakes followers.
  - One Worker primitive, `follow()` (`api/src/follow.ts`, 10 unit tests), serves both transports. It subscribes, catches up from D1, goes live, dedupes by id, resubscribes on a hub drop, and re-reads D1 after 30 s idle.
- **`mise run api:soak` passes on the deployed Worker:** 7 clients, 50/50 notes each, no gaps, no duplicates, in order, through a redeploy (hub restart) and a 6 s client drop. The clients were SSE raw (`after`), SSE `Last-Event-ID`, TypeScript SDK `notes.watch()`, Go SDK `Notes.Watch()`, CLI `notes watch`, WebSocket raw (`?after`) and TypeScript SDK `liveNotes.connect({ after })`.
- **The hub restart was real, and invisible to clients.** Workers Logs show 7 `follow: hub subscription broken, resubscribing` (`WebSocket closed unexpectedly: 1006`), one per open stream. The hub closed with 1006, not a silent 1000/1001.
- **Both specs are generated from the contract.** `api/src/asyncapi.ts` (an `asyncapi()` meta plugin plus `AsyncAPIGenerator`, on oRPC 2.0's public APIs) writes `asyncapi.json`, and the hand-written `asyncapi.yml` is gone.
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


## Go on workers-go: Huma -> OpenAPI + AsyncAPI -> Fern (api-go/, verified locally 2026-10-01; not deployed yet)

Everything here ran on this machine: natively, and as TinyGo Wasm under workerd (`cf dev`). Nothing below has run on Cloudflare itself yet (`mise run api-go:deploy`, then `api-go:live-test` and `api-go:soak`).

- **Versions:** Huma 2.39.1, workers-go 0.36.0, TinyGo 0.42.0 (binaryen 133), Go 1.27.1, workerd through cf 1.0.0-beta.5.
- **Huma runs under TinyGo on workerd,** with three things done on our side (`api-go/humaworkers`, `mise run api-go:build`):
  - `-stack-size=256kb`. With TinyGo's default stack, the first request fails with `memory access out of bounds`; 128 KB also worked, 64 KB didn't.
  - No `SchemaLinkTransformer` hook (`huma.DefaultConfig` installs it). It calls `reflect.StructOf`, and TinyGo answers `panic: unimplemented: reflect.StructOf()`.
  - Our own route matching. TinyGo's `http.ServeMux` treats `"GET /api/hello"` as a literal path, so Huma's `humago` adapter gives 404 for every route.
- **What then works under TinyGo:** query, header and body parsing; validation from struct tags (`minimum`, `maximum`, `default`, `minLength`, `pattern`, so `regexp` too); `Resolve` for custom rules; 422 `application/problem+json` errors with locations; `StreamResponse`; and generating the OpenAPI document inside the Worker.
- **Size:** `app.wasm` is 2.41 MB, 848 KB gzipped (the Workers Free limit is 3 MB gzipped). An empty workers-go handler was 863 KB, 326 KB gzipped.
- **Request time, local workerd, wall clock:** about 14 ms for `/api/hello`, about 28 ms for a D1 list of 20 notes, about 25 ms for `/api/openapi.json`. An empty workers-go handler took about 8 ms. workers-go starts a fresh Go runtime per request, so `humaworkers` registers only the operation a request matches. CPU time on Cloudflare isn't measured yet.
- **Both specs come from the Go contract** (`api-go/api/contract.go`), nothing hand-written:
  - OpenAPI 3.1.0 by Huma, with `x-fern-sdk-*`, `x-fern-pagination` and `x-fern-streaming` set through `Operation.Extensions`, and the SSE response declared as `text/event-stream` with the `Note` schema.
  - AsyncAPI 3.0.0 by `api-go/asyncapi` (a port of `api/src/asyncapi.ts` onto Huma's operations and schema registry). The channel is a hidden Huma operation, so it stays out of OpenAPI; its query parameters become `bindings.ws.query`.
- **Fern accepts them.** `fern check --api api-go` passes. The Go SDK (build, vet, tests against WireMock), the TypeScript SDK and the Rust CLI all generate and build from them.
- **The SDK surface is the oRPC one.** `TestSameSurfaceAsTheORPCContract` compares each operation's id, tags, summary, parameters with their constraints, `x-fern-*` extensions and 200 media types, plus the channel's address, summary, query binding and receive operation, with `sdk/fern/apis/api/`. One thing had to be matched by hand: Huma writes `format: int64` for a Go `int`, and Fern's Go SDK then types the parameter `*int64` instead of `*int`, so the contract uses `int32`.
- **The real-time design carries over unchanged.** `follow.Follow` is `follow()` in Go, with the same ten tests plus one, passing under the race detector. The hub is a small JavaScript Durable Object with hibernating WebSockets.
- **workers-go can't answer a WebSocket upgrade** (a 101 from Go has no `webSocket`) and has no WebSocket client. So Go answers the upgrade with a stream of lines and `worker/index.mjs` sends each as a frame; Go subscribes to the hub through `syscall/js`.
- **The same tests pass against the Go Worker, locally:**
  - `test/live-test.mjs`: 3/3 (SSE, WebSocket through the hub, resume with `Last-Event-ID`), against the Wasm under workerd and against the native build.
  - `test/sdk-live-test.mjs` with the TypeScript SDK generated from the Go specs: 2/2 (`notes.watch()`, `liveNotes.connect()`).
  - `test/soak.mjs --sdk api-go --no-deploy`: 7/7 clients, 27/27 notes each, no gaps or duplicates, in order, through a 6 s client drop. The SDKs and the CLI were the ones generated from the Go specs. The hub-restart scenario needs a deploy and hasn't run.
- **The two servers are interchangeable to clients.** The SDKs and CLI generated from the *oRPC* specs (`sdk/out/api`) pass the same tests against the *Go* server (native build): SDK live test 2/2, soak 7/7 through a client drop.
- **The SSE wire format is byte for byte the oRPC Worker's** (a comment line, then `event: message`, `retry`, `id`, `data` per note, then `event: close` with the terminator), checked against a capture from the deployed oRPC Worker.
- **The same Go code runs natively** (`mise run api-go:run`): REST, SSE and the WebSocket, on an in-memory store.
