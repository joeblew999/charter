# Plan: the real-time system (SSE + WebSockets on Cloudflare)

Tracked in issue #2. Replaces the earlier `sse.md` list of tricks. The contract side is in [asyncapi.md](asyncapi.md) (issue #3).

The goal is one design where every transport and every client gets the same guarantee from the same code, so the test matrix stays small and a fix lands once. The design is meant to be copied into our real oRPC projects.

## The guarantee

A client that says where it got to never misses a note and never sits on a dead connection. This holds across deploys, Cloudflare runtime updates, hub restarts, idle periods and its own disconnects. The hub Durable Object hibernates whenever nothing is happening.

## What the platform gives us (verified; see docs/findings.md)

- **Streams:** a Worker can stream as long as the client is connected, and bills CPU time, not wall time.
- **They still get cut:**
  - Runtime updates kill long requests after 30 s, a few times a week.
  - A deploy restarts every Durable Object and drops its sockets. The restart came 5–36 s after the deploy finished.
  - An evicted Worker loses the sockets it holds.
- **Hibernation:** only a Durable Object acting as a WebSocket server hibernates. Outgoing sockets and SSE responses don't.
- **oRPC 2.0:**
  - `DurablePublisherObject` is a hibernatable hub.
  - A hub drop reaches the SSE handler as an error.
  - `withEventMeta(value, { id, retry })` sets SSE `id:` and `retry:`.
  - `lastEventId` reaches the handler.
- **Fern:**
  - Generated clients don't reconnect.
  - The TypeScript SDK hides the SSE error event and doesn't expose event ids.
  - The CLI exits 0 after the error, and only streams with `--format raw` (generator 0.44.0).
- **Nothing buffers:** Cloudflare doesn't compress or buffer `text/event-stream`.

## Status (2026-09-30)

Built and verified live: steps 1–6 are done, and 7–9 are in progress (see Steps). `mise run api:soak` passes for 7 clients through a redeploy and a client drop, and Workers Logs show the hub restart that `follow()` hid.

## Design: five rules

1. **One log, one cursor.**
   - D1 is the source of truth. The note id is the only position anywhere: list pagination (`cursor`), stream resume (`after`), the SSE `id:`, and the `id` in every WebSocket message.
   - Resume is a normal contract input (`after`), so every generated client (SDK, CLI, docs) gets it without needing `Last-Event-ID` support.
   - A browser's `EventSource` still works: its `Last-Event-ID` header is read as `after`.
2. **The hub is disposable.**
   - The NotesHub Durable Object is only live fan-out: hibernatable, and allowed to restart at any time.
   - It stores nothing that matters, so its resume log is switched off: D1 replaces it.
3. **One subscription primitive in the Worker: `follow()` (`api/src/follow.ts`).**
   - Given `after` and a signal, it yields notes in id order: subscribe to the hub first, catch up from D1 (`id > after`), then stream live, dropping anything already sent (by id).
   - When the hub drops, it resubscribes and catches up from the last id it sent, so clients never see hub restarts or deploys.
   - It's generic (a publisher topic plus a catch-up query), so real projects reuse it unchanged.
4. **Transports are thin adapters over `follow()`, declared in the one contract.**
   - **SSE:** `notes.watch({ after, seconds })`, an oRPC event iterator. Each event has `id: <note id>`, and the stream ends after `seconds` with `retry:`. Described in OpenAPI.
   - **WebSocket:** `notes.live({ after })`, plain JSON notes (each has `id`). Described in AsyncAPI, generated from the contract ([asyncapi.md](asyncapi.md)).
   - Neither adapter has its own reconnect or resume logic.
5. **Streams are finite and never fail silently.**
   - A planned end returns the terminator `[end-of-stream]`. If the hub stays down, the stream ends *without* it, so the SDKs (`resumable: true`) reconnect by themselves. No SSE error events: generated clients read them as notes.
   - Every stream ends: SSE after `seconds`, and either transport on a failure it can't recover from, which ends the stream explicitly (SSE `event: error`, WebSocket close 1011/1012).
   - The client rule is the same everywhere: **call again with `after = last id` until you're done.** It works whether the end was planned, an error, or a network drop.
   - Fern's gaps (hidden error, exit 0, no auto-reconnect) stop mattering.

## Decisions (with the alternative we didn't take)

- **Resume from D1, not the hub's log.** The hub's 60 s log can't cover a phone that slept, and it's one more store. D1 already has everything, ordered.
- **Resume with the `after` input, not only `Last-Event-ID`.** Generated SDKs and the CLI can't send the header or see event ids; the input they get for free.
- **The Worker holds client connections, not the hub.** A hub that held clients would have to speak plain JSON (a custom DO instead of oRPC's) and couldn't hold SSE while hibernating. The Worker costs CPU only, and the hub still hibernates. Revisit only if the hub's 32,768 subscriber sockets become a limit (then: one hub socket per Worker isolate, or a hub per topic).
- **Dedupe by note id in `follow()`,** not by event ids from the hub, which change when the hub restarts.
- **Use Fern's options before writing client code.**
  - `x-fern-streaming` gets `terminator` + `resumable: true`, so SDKs reconnect after a clean drop.
  - The AsyncAPI channel's query parameters give `liveNotes.connect({ after })`.
  - What they don't cover (network resets, event names, substring terminators, the unescaped Rust terminator) goes into the rules and to Fern as issues.
- **Terminator `[end-of-stream]`, plain text.** It has to be text that no note can contain (Fern matches it as a substring) and that every generator can embed (the Rust one doesn't escape it). Note bodies reject it.
- **Live events are only a wake-up.** `follow()` yields a live item directly only when its id is the next one; otherwise it reads D1. A single SQLite writer means a visible id implies all lower ones, so this is gap-free and cheap.

## The test matrix (kept small on purpose)

`follow()` is the only code that handles failure, so it gets unit tests. Each transport then needs one live pass, and each client only needs to show that it follows the client rule.

| Scenario | How it's produced | Expected, every client |
|---|---|---|
| Steady state | notes every 2 s | all notes, in order, no duplicates |
| Hub restart | redeploy mid-stream | no gap: `follow()` resubscribes and the client sees nothing |
| Planned end | `seconds` elapses | client calls again with `after`; no gap |
| Client drop | kill the client, restart it with `after` | no gap; catch-up from D1 |
| Long idle | 20 min, no notes, then one | the note arrives |

- **Clients:** raw SSE, browser-style `EventSource`, TypeScript SDK, Go SDK, CLI, raw WebSocket, TypeScript SDK WebSocket.
- **Runner:** `mise run api:soak` (`test/soak.mjs`) prints this table, with latency. A green table on the deployed Worker is the acceptance. `--idle 20` runs the long-idle case.
- **SDK reconnect** is checked separately against mock servers: a clean end without the terminator reconnects; a network reset throws, and the client rule covers it.
- **Unit tests** for `follow()`, with a fake publisher that drops and a fake catch-up: a drop mid-stream, a duplicate across catch-up and live, abort, and `after` beyond the newest note.

## Steps

1. **`follow()` plus unit tests** (`api/src/follow.ts`, vitest). No deploy needed.
2. **SSE onto `follow()`:** `after` in the `watch` input, SSE `id:` = note id, `Last-Event-ID` read as `after`, `seconds` plus `retry:`. Regenerate `openapi.json`; `fern check` and `sdk:gen` + `sdk:check` must pass.
3. **WebSocket onto `follow()`:** `/api/notes/live?after=`, and an explicit close on a failure it can't recover from. This fixes today's silently dead socket.
4. **Hub resume off** (`DurablePublisherObject` without `resume`).
5. **The contract describes both transports:** [asyncapi.md](asyncapi.md) steps 1–3, with `notes.live` taking `after`.
6. **Matrix runner:** `mise run api:soak` with the table above, plus the Go SDK. Run it on the deployed Worker; every cell must pass.
7. **Clients follow the rule:** a small documented loop per client (TypeScript, Go, CLI shell). Try Fern CLI generator 0.45.x for streaming in json/jsonl.
8. **Upstream (outward-facing; needs a go-ahead):**
   - To Fern: the SDK hides the SSE error event, the CLI exits 0 on a stream error, the CLI buffers non-raw formats, event ids aren't exposed, and there's no reconnect.
   - To oRPC: the AsyncAPI generator (#2115).
9. **Write-up:** docs/findings.md, plus a short "real-time on Cloudflare" section in `docs/sdk.md` that our other projects copy (the rules, `follow.ts`, the client loop).

## Done when

- `mise run api:soak` is green for every client × scenario on the deployed Worker.
- `follow()` has unit tests.
- Both specs are generated from the contract.
- The hub hibernates.
- No transport has its own resume or reconnect code.
