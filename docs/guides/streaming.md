---
title: "Real-time: SSE and WebSocket"
nav_order: 2
parent: Guides
---

# Real-time: SSE and WebSocket

This page gets you a live stream of your own resource: clients receive new items as they are created, over Server-Sent Events (SSE) or a WebSocket, and a client that disconnects, or a deploy that restarts the Worker, loses nothing. Your project from `dev new` already has both for notes, so the page shows how they are built and what to copy for a second stream. The reasons for the design are in [How real-time works](../realtime.md).

## Try the stream you have

```sh
mise run api:go:run                                              # natively on :5174
curl -X POST localhost:5174/api/notes -H 'content-type: application/json' -d '{"body":"first"}'
curl -N 'localhost:5174/api/notes/watch?after=0&seconds=5'       # SSE
```

The stream sends every note after id 0, then the end marker (a real run, with the first note in the store):

```
: 

event: message
retry: 1000
id: 1
data: {"id":1,"body":"first","created_at":"2026-10-01 10:37:00"}

event: close
data: "[end-of-stream]"
```

Three inputs say where a stream starts:

| You send | You get |
|---|---|
| `?after=1` | Notes with an id above 1, then new ones |
| header `Last-Event-ID: 2` (what a browser's `EventSource` sends when it reconnects) | The same as `after=2` |
| neither | Only notes created from now on |

When both are sent, the larger position wins. Checked with three notes in the store: `after=1` gave notes 2 and 3, `Last-Event-ID: 2` gave note 3.

## The client rule

**When a stream ends or fails, call again with `after` set to the id of the last item you received.** It is the whole client, for every client and every reason a stream ends: the planned end after `seconds`, a dropped connection, a deploy.

TypeScript SDK:

```ts
import { BillingApiClient } from "billing-api";   // your generated SDK (sdk/out/api-go/typescript)

const client = new BillingApiClient({ baseUrl: "http://localhost:5174" });
let after: string | undefined;
for (;;) {
  try {
    for await (const note of await client.notes.watch({ after, seconds: 60 })) {
      handle(note);
      after = String(note.id);
    }
  } catch {
    await new Promise(r => setTimeout(r, 1000));   // network reset: back off, then resume
  }
}
```

Go SDK (the module name is the one in `sdk/fern/apis/api-go/generators.yml`):

```go
import (
	"context"
	"fmt"
	"time"

	billingapi "example.com/billingapi"
	"example.com/billingapi/client"
	"example.com/billingapi/option"
)

func follow(ctx context.Context, base string) {
	c := client.NewClient(option.WithBaseURL(base))
	var after *string
	for ctx.Err() == nil {
		stream, err := c.Notes.Watch(ctx, &billingapi.WatchNotesRequest{After: after})
		if err == nil {
			for {
				note, err := stream.Recv()
				if err != nil { // io.EOF is the normal end
					break
				}
				handle(note)
				id := fmt.Sprint(note.ID)
				after = &id
			}
			stream.Close()
		}
		time.Sleep(time.Second)
	}
}
```

`curl`, in bash (it reads the `id:` lines and starts again after the last one; this loop was run, against a stream that ended every 2 seconds, and resumed with no gap and no duplicate):

```sh
after=0
while :; do
  while IFS= read -r line; do
    case $line in
      id:*) after=${line#id: } ;;
      data:*\[end-of-stream\]*) ;;
      data:*) echo "${line#data: }" ;;
    esac
  done < <(curl -sN "http://localhost:5174/api/notes/watch?after=$after&seconds=60")
  sleep 1
done
```

The WebSocket client does the same: on close, connect again with `after`:

```ts
const socket = await client.liveNotes.connect({ after, reconnectAttempts: 0 });
```

A browser's `EventSource` needs no loop. Its automatic `Last-Event-ID` is the same position.

The TypeScript and Go loops are the ones the repo's soak test clients use, but the snippets above were not run for this page. They do not run on every end: the generated SDKs also reconnect by themselves after a clean end without the end marker.

## What is guaranteed, and what is not

- **Guaranteed:** a client that follows the rule receives every item exactly once and in order, across a planned end, a client disconnect, and a restart of the hub. The Worker's soak test checked this on the deployed example, for 7 kinds of client, through a redeploy and a 6-second client drop, in the repo's [findings](../findings.md). That test is the oRPC and Go notes API's, not yours: run it on yours with `mise run api:go:soak` (it redeploys your Worker).
- **Not guaranteed:** delivery time. If the hub is silent, the stream only looks at the database every 30 seconds, so a missed wake-up costs up to that delay and never an item.
- **Not guaranteed:** that the stream stays open. It ends by design after `seconds` (1 to 300, default 30), and the SDKs, the network and Cloudflare end it too. That is why the client rule exists.
- **Not kept:** items from before your `after`, if you never saw them. A new client without `after` gets only what is created from then on.
- **No error events.** If the feed gives up (the hub stays down for 5 tries), the SSE stream just ends without the end marker and the WebSocket closes with code 1011. The client rule handles both.

## How it is built

A stream has four parts. The first is the contract, the others are in your project already.

### 1. The SSE operation (`api/go/api/contract.go`)

```go
huma.Register(api, huma.Operation{
	OperationID: "watchNotes", Method: http.MethodGet, Path: "/api/notes/watch",
	Summary: "Stream notes as they are created (Server-Sent Events). ...",
	Tags:    []string{"notes"},
	Responses: map[string]*huma.Response{"200": {
		Description: "OK",
		Content:     map[string]*huma.MediaType{"text/event-stream": {Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[Note](), true, "")}},
	}},
	Extensions: sdk("notes", "watch", map[string]any{
		"x-fern-streaming": map[string]any{"format": "sse", "terminator": END, "resumable": true},
	}),
}, env.watch)
```

- **`Responses` with `text/event-stream`** says each event's `data:` is a `Note`. Huma has no tag for this.
- **`x-fern-streaming`** makes the SDK return a stream you loop over. `terminator` is the data of the planned end: an event whose data contains it ends the stream. `resumable: true` makes the SDKs reconnect by themselves after a clean drop, sending `Last-Event-ID`.
- **The terminator is the constant `END`** (`[end-of-stream]`). Fern matches it as a substring of every event's data, so no note may contain it: that is why `CreateInput` has the `Resolve` rule ([Define your API](contract.md#a-rule-the-tags-cannot-say)).
- **The input** has `After` (`query:"after"`, only digits), `Seconds` and a hidden `LastEventID` (`header:"Last-Event-ID" hidden:"true"`, left out of the spec, so the SDKs use `after`).

### 2. The handler returns a `huma.StreamResponse`

`env.watch` in `api/go/api/handlers.go` writes the SSE frames itself. Each event is `event: message`, `retry: 1000`, `id: <note id>` and `data: <JSON>`. The `id` is what makes `Last-Event-ID` the same as `after`. After `seconds` it writes `event: close` with the terminator. Write to the response through the `flushing` helper in the same file: workers-go does not buffer, but `net/http` does.

### 3. The feed: `follow.Follow`

Both the SSE and the WebSocket handler call `follow.Follow` (package `github.com/joeblew999/orpc-api/go/follow`, imported by your project; you do not copy it):

```go
err := follow.Follow(ctx, source, options, func(note Note) error { /* write one event */ return nil })
```

Given a resume position, `Follow` subscribes to live items, reads what is missing from the database, then streams live items, dropping any it already sent. When the live source drops, it resubscribes and catches up. Your `Source` and your item type provide:

| You provide | What it does |
|---|---|
| `Position() int64` on the item type | The item's place in the log. Here it is the note's id |
| `Since(ctx, after, limit)` | Items with a position above `after`, oldest first, at most `limit` |
| `Latest(ctx)` | The newest position, or 0 when there is none: where a follower with no `after` starts |
| `Subscribe(listener, onError)` | Call `listener` for every new live item and `onError` if the subscription breaks. Neither may block. Return an `unsubscribe` function |

The notes API's `feed` type in `api/go/api/handlers.go` joins its `Store` (`Since`, `Latest`) and its `Hub` (`Subscribe`) into one `Source`. `follow.Options` tunes it (`After`, `Recheck` 30 s, `Retry` 1 s, `MaxFailures` 5, `PageSize` 100, `OnBroken`); the zero value is the production setting. `Follow` returns an error only when it gives up, and a cancelled context ends it quietly.

The positions must come from one ordered log with no gaps that appear later. The id of a SQLite row (D1) does. A live item is only a wake-up: `Follow` sends it directly only when it is the next position, and otherwise reads the log.

### 4. The hub

The hub is the live fan-out: when a handler creates a note it calls `hub.Publish`, and every open stream is woken. In Go it is the `hub` package ([reference](../reference/packages.md#hub)). On Cloudflare it is a Durable Object class, the library's `Hub` (`go/worker/hub.mjs` in orpc-api, exported by `api/go/worker.mjs` as `NotesHub`): JavaScript, because a Durable Object class has to be. It stores nothing and sleeps between notes, so it may restart at any time without losing data: the database is the log.

Natively (`mise run api:go:run`) the in-memory store plays both parts (`MemStore` is a `Store` and embeds `hub.Memory[Note]`), so no Durable Object is needed to try a stream.

## The WebSocket

`GET /api/notes/live` is the same feed over a WebSocket, with plain JSON notes. It is in the AsyncAPI spec, not the OpenAPI one:

```go
huma.Register(api, asyncapi.Operation(huma.Operation{
	OperationID: "liveNotes", Method: http.MethodGet, Path: "/api/notes/live",
	Summary: "New notes over a WebSocket, as plain JSON. ...",
}, asyncapi.Channel{Name: "liveNotes", OperationID: "receiveNote", Message: "Note", Payload: Note{}}), env.live)
```

`asyncapi.Operation` marks the operation as a channel: it is left out of the OpenAPI spec and written into `asyncapi.json`, with `Payload` as the type of every message. The channel's `Name` is what the SDK is named after (`client.liveNotes.connect()`), and the operation's query inputs (`after`) become the connect options.

Go on Cloudflare cannot answer a WebSocket upgrade itself. So the handler answers with plain HTTP: one JSON note per line (`application/x-ndjson`), and the Worker's entry (`api/go/worker.mjs`, with the library's `websocket.mjs`) calls Go for any request with `Upgrade: websocket` and sends each line as one frame. If the feed gives up, the entry closes the socket with 1011. A plain `GET` with no upgrade is refused (checked):

```sh
curl -s localhost:5174/api/notes/live
# {"title":"Upgrade Required","status":426,"detail":"expected a WebSocket upgrade"}
```

Running natively, `go/transport` does the same job in Go. I did not open a WebSocket for this page; the repo's `api:go:live-test` does, natively and against a deployment.

## Add a stream of your own resource

The SSE and WebSocket operations, `Follow` and the `Source` are yours to reuse by writing them again for your type: copy `watch`, `live`, `feed` and their input structs, rename, give your item type `Position()`, and add the `Since` and `Latest` methods to your store.

**The hub needs no copying.** A second feed is a second name on the same binding: `hub.DurableObject[Device]("HUB", "devices")` in `api/go/platform_js.go`, and `hub.Memory[Device]` in `api/go/platform_other.go`. The library's class (`hub.mjs`) and the `HUB` binding stay as they are: each name is its own object with its own subscribers. Add a function for it to `Env` beside `Hub`. Why the hub is built this way: [How real-time works](../realtime.md).

## Check it

```sh
mise run api:go:test:native   # the real-time live test against the native build
mise run api:go:test:workerd  # the same on the Wasm under workerd
mise run api:go:live-test     # REMOTE: against the Worker you deployed
```
