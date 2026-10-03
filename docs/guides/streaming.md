---
title: Streaming
nav_order: 2
parent: Guides
---
# Streaming: SSE and a WebSocket that lose nothing

A new project streams notes over Server-Sent Events (`watchNotes`) and a WebSocket (`liveNotes`). Both survive deploys and dropped connections. Code comments cite the rules below by number.

```sh
mise run run
curl -N 'localhost:5174/api/notes/watch?after=0&seconds=5'   # every note after id 0, then the end marker
```

## The client rule

**When a stream ends or fails, call again with `after` set to the id of the last item received.** A browser's `EventSource` needs no loop: its `Last-Event-ID` is read the same way. The generated SDKs reconnect only after a clean end, so other clients keep a loop: `test/soak.mjs` and `test/soak-go/main.go` have one for each kind.

## The rules

1. **One log, one cursor.** D1 is the source of truth; the item's id is the only position (the list's `cursor`, `after`, the SSE `id:`, each WebSocket message's `id`).
2. **The hub is disposable.** A Durable Object that only wakes open streams; it stores nothing and may restart at any time.
3. **One feed.** `follow.Follow` (Go) or `follow()` (`ts/src/follow.ts`) subscribes to the hub, catches up from the log, then goes live; after a hub drop it resubscribes and catches up, and without a wake-up it reads the log every 30 seconds.
4. **Transports are thin adapters over the feed,** declared in the contract: SSE in OpenAPI, the WebSocket in AsyncAPI.
5. **Streams end, and never fail silently.** SSE ends after `seconds` with `event: close` and `[end-of-stream]`; if the feed gives up, SSE ends without it and the WebSocket closes with 1011.

## Add a stream of your own

1. **Copy** `watchNotes` and `liveNotes` in `api/contract.go`, `watch`, `live` and `feed` in `api/handlers.go`, and rename.
2. **Give your item `Position() int64`,** and your store `Since(ctx, after, limit)` and `Latest(ctx)`. The ids must come from one ordered log with no gap filled later: a SQLite row id does.
3. **Add a feed to the hub:** `hub.DurableObject[Device]("HUB", "devices")` in `platform_js.go`, `hub.Memory[Device]` in `platform_other.go`. The binding stays the same.
4. **No item may contain `[end-of-stream]`:** refuse it in the input's `Resolve`, as `CreateInput` does.

A WebSocket the client also sends on: see `examples/showcase-go/api/contract.go` and [SDKs](sdks.md#fern-features).

## Check it

```sh
mise run test:native     # the live test, locally
mise run soak            # REMOTE, redeploys: seven kinds of client through a redeploy and a client drop
mise run soak -- --idle 20   # and after 20 quiet minutes
```

The soak passes when every client gets every item once, in order. Delivery time is not guaranteed: a missed wake-up costs up to 30 seconds, never an item.
