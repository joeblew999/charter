---
title: How real-time works
nav_order: 2
parent: Concepts
---
# Real-time: one gap-free feed over SSE and a WebSocket

How both servers deliver new notes to clients without losing any, and what a client has to do. Read it before changing a stream, the hub or `follow()`, and copy it (the rules, the feed and the client loop) into a project that needs real-time on Cloudflare. The test programs are in [testing.md](testing.md); what was measured is in [findings.md](findings.md).

## The guarantee

A client that says where it got to never misses a note and never sits on a dead connection. This holds across deploys, hub restarts, idle periods and the client's own disconnects. The hub hibernates whenever nothing is happening.

## What a client does

**When the stream ends or fails, call again with `after` = the last note id.** That is the whole client rule, for every client and every reason a stream ends.

The generated SDKs reconnect by themselves after a clean end without the terminator (`x-fern-streaming` with `resumable: true`). The loop covers the two cases they don't: the planned end, and a network reset, which the SDKs throw ([upstream.md](upstream.md), fern-api/fern#17937).

```ts
// TypeScript SDK (SSE)
let after: string | undefined;
for (;;) {
  try { for await (const note of await client.notes.watch({ after, seconds: 60 })) { handle(note); after = String(note.id); } }
  catch { await new Promise(r => setTimeout(r, 1000)); } // network reset: back off, then resume
}
// TypeScript SDK (WebSocket): on close, connect again with after
const socket = await client.liveNotes.connect({ after, reconnectAttempts: 0 });
```

```go
// Go SDK
for {
	stream, err := c.Notes.Watch(ctx, &orpcapi.WatchNotesRequest{After: after})
	if err == nil {
		for note, err := stream.Recv(); err == nil; note, err = stream.Recv() { handle(note); a := strconv.Itoa(note.ID); after = &a }
		stream.Close()
	}
	time.Sleep(time.Second)
}
```

```sh
# The Fern CLI: prints a stream's notes when it ends (json/jsonl), or live with --format raw
after=""
while :; do
  out=$(orpc-api notes watch ${after:+--after "$after"} --seconds 60 --format jsonl)
  [ -n "$out" ] && { echo "$out"; after=$(echo "$out" | tail -1 | jq -r .id); }
done
```

A browser's `EventSource` needs nothing: the SSE id is the note id, so its automatic `Last-Event-ID` is the same position.

The loops above are written from the test clients (`test/soak.mjs`, `test/soak-go/main.go`), which follow the same rule; the snippets themselves are not run by a test.

## The five rules

Code comments refer to these by number.

1. **One log, one cursor.**
   - D1 is the source of truth. The note id is the only position anywhere: list pagination (`cursor`), stream resume (`after`), the SSE `id:`, and the `id` in every WebSocket message.
   - Resume is a normal contract input (`after`), so every generated client (SDK, CLI, docs) gets it without `Last-Event-ID` support.
   - A browser's `EventSource` still works: its `Last-Event-ID` header is read as a position. When both are sent (a reconnecting SDK resends its original `after`), the newer wins.
2. **The hub is disposable.**
   - The hub (the `NotesHub` Durable Object) is only live fan-out: hibernatable, and allowed to restart at any time.
   - It stores nothing that matters and keeps no resume log: D1 replaces it.
3. **One subscription primitive in the Worker: the feed.**
   - `follow()` in `api/src/follow.ts` and `Follow` in `api-go/follow/`, the same design with the same tests.
   - Given `after`, it yields notes in id order: subscribe to the hub first, catch up from D1 (`id > after`), then stream live, dropping anything already sent (by id).
   - When the hub drops, it resubscribes and catches up from the last id it sent, so clients never see hub restarts or deploys.
   - It only needs a source with `subscribe`, `since` and `latest`, so another project uses it unchanged.
4. **Transports are thin adapters over the feed, declared in the one contract.**
   - **SSE:** `notes.watch({ after, seconds })`. Each event has `id: <note id>` and `retry: 1000`. Described in OpenAPI.
   - **WebSocket:** `notes.live({ after })`, plain JSON notes (each has `id`). Described in AsyncAPI, generated from the contract.
   - Neither adapter has reconnect or resume logic of its own.
5. **Streams are finite and never fail silently.**
   - SSE ends after `seconds` (1 to 300, default 30) with `event: close` carrying the terminator `[end-of-stream]`.
   - If the feed gives up (the hub stays down), SSE ends without the terminator, so the SDKs reconnect by themselves, and the WebSocket closes with 1011.
   - No SSE error events are sent: generated clients read them as notes (fern-api/fern#17938).

## How the feed works

| Setting | TypeScript (`follow()` options) | Go (`follow.Options`) | Default |
|---|---|---|---|
| Resume position | `after` | `After` | absent: only notes newer than now |
| Read the log when nothing arrived for this long | `recheckMs` | `Recheck` | 30 s |
| Wait between resubscribes after a hub failure | `retryMs` | `Retry` | 1 s |
| Failed subscribes in a row before giving up | `maxFailures` | `MaxFailures` | 5 |
| Page size for log reads | `pageSize` | `PageSize` | 100 |
| Called when the subscription breaks (for logs) | `onBroken` | `OnBroken` | none |

- **A live note is only a wake-up.** The feed yields a live note directly only when its id is the next one; otherwise it reads the log. A single SQLite writer means a visible id implies all lower ones, so this is gap-free and cheap.
- **A silent hub costs a delay, never a note.** When nothing arrives for the recheck time, the feed reads the log anyway.
- **It gives up after that many failed subscribes in a row, or when reading the log fails.** The adapter then ends the stream as rule 5 says. An abort ends it quietly.

Who holds what: the Worker holds each client's SSE response or WebSocket and subscribes to the hub with a WebSocket of its own. The hub accepts subscribers with the hibernatable WebSocket API, so it sleeps between notes.

## Why it is built this way

- **Resume from D1, not from a log in the hub.** A short log in the hub can't cover a phone that slept, and it's one more store. D1 already has everything, ordered.
- **Resume with the `after` input, not only `Last-Event-ID`.** Generated SDKs and the CLI can't send the header or see event ids; the input they get for free.
- **The Worker holds client connections, not the hub.** A hub that held clients would have to speak plain JSON (a custom Durable Object instead of oRPC's) and couldn't hold SSE while hibernating. The Worker is billed for CPU time, and the hub still hibernates. The limit to watch is the hub's 32,768 subscriber sockets (Cloudflare's figure, not reached or tested here). [plans/performance.md](plans/performance.md) has an idea that moves client sockets into the hub.
- **Dedupe by note id in the feed,** not by event ids from the hub, which change when the hub restarts.
- **Use Fern's options before writing client code.** `x-fern-streaming` has `terminator` and `resumable: true`, so SDKs reconnect after a clean drop. The AsyncAPI channel's query parameters give `liveNotes.connect({ after })`. What Fern's clients don't cover is in [upstream.md](upstream.md).
- **The terminator is `[end-of-stream]`, plain text.** Fern matches it as a substring of each event's data, so it has to be text no note can contain: note bodies reject it. Fern's Rust generator pastes it into source unescaped, so it has no quotes or backslashes.

## What the platform does

Measured in this repo ([findings.md](findings.md)):

- **A deploy restarts every Durable Object and drops its sockets.** The restart came 5 s and 36 s after the deploy finished, in two runs. The hub's subscribers saw close code 1006.
- **Nothing buffers a stream.** Cloudflare doesn't compress or buffer `text/event-stream`; the first byte arrived in about 0.1 s.
- **The hub hibernates.** In the hour that held a soak and a 20-minute idle run, the Go Worker's hub was active for 5.7 s.
- **Go timers need a fix on Cloudflare,** or streams don't end at their deadline ([api-go.md](api-go.md#huma-on-workers-go-and-tinygo-what-it-takes)).

Not measured here, taken from Cloudflare's documentation when the design was made: runtime updates end long requests after 30 s, a few times a week, and an evicted Worker loses the sockets it holds. The client rule covers both.

## How it is tested

The feed is the only code that handles failure, so it has unit tests (`api/test/follow.test.ts`, `api-go/follow/follow_test.go`). Each transport then needs one live pass, and each client only has to show that it follows the client rule. That keeps the matrix small.

| Scenario | How it's produced | Expected, every client |
|---|---|---|
| Steady state | a note every 2 s | all notes, in order, no duplicates |
| Planned end | SSE streams end every 15 s | the client calls again with `after`; no gap |
| Hub restart | the Worker is redeployed 30 s in | no gap: the feed resubscribes and the client sees nothing |
| Client drop | 65 s in, every client is offline for 6 s, then resumes with `after` | no gap; catch-up from D1 |
| Long idle | `--idle 20`: 20 minutes with no notes, then one | the note arrives |

- **Clients (7):** raw SSE with `after`, SSE with `Last-Event-ID` (what `EventSource` sends), the TypeScript SDK's `notes.watch()`, the Go SDK's `Notes.Watch()`, the Fern CLI's `notes watch`, a raw WebSocket, and the TypeScript SDK's `liveNotes.connect({ after })`.
- **Runner:** `mise run api:soak` and `mise run api-go:soak` (`test/soak.mjs`, [testing.md](testing.md)). PASS is every note exactly once, in order. Latency is reported, not judged: the Fern CLI prints json/jsonl only when a stream ends.

Results, on the deployed Workers:

| Worker | Run | Result | When |
|---|---|---|---|
| oRPC Worker | redeploy and client drop | 7 clients, 50/50 notes each | 2026-09-30 |
| Go Worker | redeploy and client drop | 7 clients, 38/38 notes each | 2026-10-01 |
| Go Worker | `--idle 20` | 7/7 | 2026-10-01 |

The long-idle run against the oRPC Worker has no entry in findings.
