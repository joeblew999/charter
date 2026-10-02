---
title: Streaming and real-time
nav_order: 3
parent: Guides
---

# Streaming and real-time: SSE and a WebSocket

How to give clients a live stream of your own resource, over Server-Sent Events or a WebSocket, that loses nothing across disconnects and deploys. A new project already has both for notes. Why it is built this way: [How real-time works](../realtime.md).

## Try the stream you have

```sh
mise run run                                                   # natively: http://localhost:5174
curl -X POST localhost:5174/api/notes -H 'content-type: application/json' -d '{"body":"first"}'
curl -N 'localhost:5174/api/notes/watch?after=0&seconds=5'     # every note after id 0, then the end marker
```

```
: 

event: message
retry: 1000
id: 1
data: {"id":1,"body":"first","created_at":"2026-10-01 10:37:00"}

event: close
data: "[end-of-stream]"
```

| You send | You get |
|---|---|
| `?after=1` | Notes with an id above 1, then new ones |
| header `Last-Event-ID: 2` (what a browser's `EventSource` sends) | The same as `after=2` |
| both | The larger position wins |
| neither | Only notes created from now on |

## The client rule

**When a stream ends or fails, call again with `after` set to the id of the last item received.** That is the whole client, for every reason a stream ends: the planned end after `seconds`, a dropped connection, a deploy.

```ts
// TypeScript SDK (SSE)
let after: string | undefined;
for (;;) {
  try {
    for await (const note of await client.notes.watch({ after, seconds: 60 })) { handle(note); after = String(note.id); }
  } catch {
    await new Promise(r => setTimeout(r, 1000));   // network reset: back off, then resume
  }
}
// TypeScript SDK (WebSocket): on close, connect again with after
const socket = await client.liveNotes.connect({ after, reconnectAttempts: 0 });
```

```go
// Go SDK
for ctx.Err() == nil {
	stream, err := c.Notes.Watch(ctx, &notes.WatchNotesRequest{After: after})
	if err == nil {
		for note, err := stream.Recv(); err == nil; note, err = stream.Recv() {
			handle(note)
			id := fmt.Sprint(note.ID)
			after = &id
		}
		stream.Close()
	}
	time.Sleep(time.Second)
}
```

- **A browser's `EventSource` needs no loop:** the SSE id is the item's id.
- **The generated SDKs reconnect by themselves** after a clean end without the end marker. The loop covers what they do not: the planned end, and a network reset ([fern-api/fern#17937](../upstream.md)).
- **These loops are those of the soak's clients** (`test/soak.mjs`, `test/soak-go/main.go`). The snippets themselves are not run by a test.

## What is guaranteed

- **Guaranteed:** a client that follows the rule receives every item once, in order. The soak proved it on the deployed notes Workers for seven kinds of client ([results](../realtime.md#how-it-is-tested)). Prove it on yours: `mise run soak`.
- **Not guaranteed: delivery time.** A missed wake-up costs up to 30 seconds, never an item.
- **Not guaranteed: that a stream stays open.** It ends by design after `seconds` (1 to 300, default 30).
- **No error events.** If the feed gives up, SSE ends without the end marker and the WebSocket closes with 1011.

## The four parts of a stream

**1. The SSE operation** (`api/contract.go`). `Responses` says each event's `data:` is a `Note`; `x-fern-streaming` makes the SDK return a stream:

```go
Responses: map[string]*huma.Response{"200": {
	Description: "OK",
	Content:     map[string]*huma.MediaType{"text/event-stream": {Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[Note](), true, "")}},
}},
Extensions: sdk("notes", "watch", map[string]any{
	"x-fern-streaming": map[string]any{"format": "sse", "terminator": END, "resumable": true},
}),
```

The input has `After` (digits only), `Seconds`, and a hidden `LastEventID` (`header:"Last-Event-ID" hidden:"true"`). No item may contain the terminator `END`: `CreateInput.Resolve` refuses it.

**2. The handler** (`watch` in `api/handlers.go`) returns a `huma.StreamResponse` and writes the frames itself: `event: message`, `retry: 1000`, `id: <id>`, `data: <JSON>`, and after `seconds` the `event: close`. Write through the `flushing` helper: `net/http` buffers.

**3. The feed.** Both transports call `follow.Follow` with a source and emit what it yields ([Go packages](../reference/packages.md#follow)). `feed` in `api/handlers.go` joins the store and the hub into that source:

| You provide | What it does |
|---|---|
| `Position() int64` on the item type | The item's place in the log: its id |
| `Since(ctx, after, limit)` on the store | Items above `after`, oldest first |
| `Latest(ctx)` on the store | The newest id, or 0 |
| `Subscribe(listener, onError)` from the hub | Live items. Neither callback may block |

The ids must come from one ordered log in which no gap is filled later. A SQLite row id (D1) does.

**4. The hub.** `create` calls `hub.Publish`, and every open stream is woken. On Cloudflare it is the library's Durable Object class, exported by `worker.mjs`; natively `MemStore` plays both parts.

## The WebSocket

`GET /api/notes/live` is the same feed over a WebSocket. `asyncapi.Operation` marks the operation as a channel: it leaves OpenAPI and goes into `fern/asyncapi.json`.

```go
huma.Register(api, asyncapi.Operation(huma.Operation{
	OperationID: "liveNotes", Method: http.MethodGet, Path: "/api/notes/live",
	Summary: "New notes over a WebSocket, as plain JSON. ...",
	Errors:  []int{http.StatusUpgradeRequired},
}, asyncapi.Channel{Name: "liveNotes", OperationID: "receiveNote", Message: "Note", Payload: Note{}}), env.live)
```

- **`Name`** is what the SDK is named after: `client.liveNotes.connect()`. The query inputs (`after`) are the connect options.
- **The handler answers with plain HTTP:** one JSON item per line. The Worker glue sends each line as a frame; natively `transport.Serve` does. A plain `GET` gets 426.
- **A channel the client also sends on:** [Fern features](fern-features.md#a-websocket-the-client-also-sends-on).

## A stream of your own resource

1. **Copy** `watch`, `live`, `feed` and their input structs, and rename.
2. **Give your item type `Position()`,** and your store `Since` and `Latest`.
3. **Add a hub for it:** a second name on the same binding. `hub.DurableObject[Device]("HUB", "devices")` in `platform_js.go`, a `hub.Memory[Device]` in `platform_other.go`, and a function for it in `Env`. The class and the `HUB` binding stay as they are.

## Check it

```sh
mise run test:native     # the live test against the native build
mise run test:workerd    # the same on the Wasm under workerd
mise run live-test       # REMOTE: against the deployed Worker
mise run soak            # REMOTE, redeploys the Worker: every client, every scenario
```
