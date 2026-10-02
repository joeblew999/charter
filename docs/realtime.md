---
title: How real-time works
nav_order: 3
parent: Concepts
---
# How real-time works: one gap-free feed over SSE and a WebSocket

Why a stream in a charter project loses nothing, in Go ([Huma](https://huma.rocks)) and in TypeScript ([oRPC](https://orpc.dev)) alike. Read it before you change a stream, the hub or the feed. Code comments cite the five rules by number. To build a stream: [Streaming and real-time](guides/streaming.md).

## The guarantee

A client that says where it got to never misses an item and never sits on a dead connection: across deploys, hub restarts, idle hours and its own disconnects. The price is one client rule: **when the stream ends or fails, call again with `after` = the last id received** ([the loops](guides/streaming.md#the-client-rule)).

## The five rules

1. **One log, one cursor.**
   - The database (D1) is the source of truth. The item's id is the only position anywhere: list pagination (`cursor`), stream resume (`after`), the SSE `id:`, the `id` in every WebSocket message.
   - Resume is a normal contract input (`after`), so every generated client has it.
   - A browser's `EventSource` works too: its `Last-Event-ID` header is read as a position. When both are sent, the newer wins.
2. **The hub is disposable.**
   - The hub is a Durable Object that only fans live items out. It hibernates, and may restart at any time.
   - It stores nothing and keeps no resume log: the database replaces it.
3. **One subscription primitive: the feed.**
   - `follow.Follow` in Go (`go/follow/`), `follow()` in TypeScript (`examples/notes-ts/src/follow.ts`): one design, the same tests.
   - Given `after`, it subscribes to the hub first, catches up from the database (`id > after`), then goes live, dropping what it already sent.
   - When the hub drops, it resubscribes and catches up. Clients never see a hub restart or a deploy.
4. **Transports are thin adapters over the feed, declared in the contract.**
   - **SSE:** each event has `id: <item id>` and `retry: 1000`. Described in OpenAPI.
   - **WebSocket:** plain JSON items. Described in AsyncAPI, generated from the contract.
   - Neither has reconnect or resume logic of its own.
5. **Streams are finite and never fail silently.**
   - SSE ends after `seconds` (1 to 300, default 30) with `event: close` carrying the terminator `[end-of-stream]`.
   - If the feed gives up, SSE ends without the terminator, so the SDKs reconnect by themselves, and the WebSocket closes with 1011.
   - No SSE error events: generated clients read them as items ([fern-api/fern#17938](upstream.md)).

## How the feed works

| Setting | Go (`follow.Options`) | TypeScript (`follow()` options) | Default |
|---|---|---|---|
| Resume position | `After` | `after` | absent: only items newer than now |
| Read the log when nothing arrived for this long | `Recheck` | `recheckMs` | 30 s |
| Wait between resubscribes after a hub failure | `Retry` | `retryMs` | 1 s |
| Failed subscribes in a row before giving up | `MaxFailures` | `maxFailures` | 5 |
| Page size for log reads | `PageSize` | `pageSize` | 100 |
| Called when the subscription breaks, for logs | `OnBroken` | `onBroken` | none |

- **A live item is only a wake-up.** The feed emits it directly only when its id is the next one; otherwise it reads the log. One SQLite writer means a visible id implies all lower ones, so this is gap-free.
- **A silent hub costs a delay, never an item.** After the recheck time the feed reads the log anyway.
- **It gives up** after that many failed subscribes in a row, or when the log fails. The adapter then ends the stream as rule 5 says. A cancelled context ends it quietly.

Who holds what: the Worker holds each client's SSE response or WebSocket, and subscribes to the hub with a WebSocket of its own. The hub accepts subscribers with the hibernatable WebSocket API, so it sleeps between items.

## Why it is built this way

- **Resume from the database, not from a log in the hub.** A short log in the hub cannot cover a phone that slept, and it is one more store.
- **Resume with the `after` input, not only `Last-Event-ID`.** Generated SDKs and the CLI cannot send the header or see event ids.
- **The Worker holds client connections, not the hub.** The hub then stays generic and hibernates. The limit to watch is the hub's 32,768 subscriber sockets (Cloudflare's figure, not reached or tested here).
- **Dedupe by item id in the feed,** not by the hub's event ids, which change when it restarts.
- **Use Fern's options before writing client code.** `x-fern-streaming` has `terminator` and `resumable: true`; an AsyncAPI channel's query parameters give `connect({ after })`.
- **The terminator is `[end-of-stream]`, plain text.** Fern matches it as a substring of each event's data, so no item may contain it: note bodies reject it. Fern's Rust generator pastes it into source unescaped, so it has no quotes or backslashes ([upstream](upstream.md)).

## What the platform does

Measured here ([Findings](findings.md)):

- **A deploy restarts every Durable Object and drops its sockets.** The restart came 5 s and 36 s after the deploy finished, in two runs. Subscribers saw close code 1006.
- **Nothing buffers a stream.** Cloudflare does not compress or buffer `text/event-stream`; the first byte arrived in about 0.1 s.
- **The hub hibernates.** In an hour that held a soak and a 20-minute idle run, the Go Worker's hub was active for 5.7 s.
- **Go timers stall on Cloudflare without a fix,** and streams then do not end at their deadline. The library's glue has it (`go/worker/tinygo-clock.mjs`, [tinygo-org/tinygo#5798](upstream.md)).

Not measured, taken from Cloudflare's documentation: runtime updates end long requests after 30 s, a few times a week, and an evicted Worker loses the sockets it holds. The client rule covers both.

## How it is tested

The feed is the only code that handles failure, so it has unit tests (`go/follow/follow_test.go`, `examples/notes-ts/test/follow.test.ts`). The soak then runs every client against every scenario on the deployed Worker (`mise run soak`, in `examples/notes-go/test/soak.mjs`).

| Scenario | How it is produced | Expected of every client |
|---|---|---|
| Steady state | A note every 2 s | All notes, in order, no duplicates |
| Planned end | SSE streams end every 15 s | The client calls again with `after`; no gap |
| Hub restart | The Worker is redeployed 30 s in | No gap: the feed resubscribes |
| Client drop | 65 s in, every client is offline for 6 s | No gap: catch-up from the database |
| Long idle | `--idle 20`: 20 minutes with no notes, then one | The note arrives |

The seven clients: raw SSE with `after`, SSE with `Last-Event-ID`, the TypeScript SDK's `notes.watch()`, the Go SDK's `Notes.Watch()`, the CLI's `notes watch`, a raw WebSocket, the TypeScript SDK's `liveNotes.connect({ after })`. PASS is every note exactly once, in order. Latency is reported, not judged.

| Worker | Run | Result | When |
|---|---|---|---|
| TypeScript notes | redeploy and client drop | 7 clients, 50/50 notes each | 2026-09-30 |
| Go notes | redeploy and client drop | 7 clients, 38/38 notes each | 2026-10-01 |
| Go notes | `--idle 20` | 7/7 | 2026-10-01 |

The long-idle run against the TypeScript Worker has no entry in the findings.
