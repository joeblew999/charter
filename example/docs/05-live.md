# The live feed

A feed is declared once and serves everything that shows it.

```go
var Notes = live.Feed[Note]{
	Binding: "HUB", Name: "notes",
	Log: func() (live.Log[Note], error) { return Open() },
}
```

The log, a D1 table, is the truth. The hub, a Durable Object, only wakes whoever follows. So no item is lost or sent twice, across reconnects and deploys alike.

## Writing

```go
note, err := store.Create(ctx, body) // the write goes to the log
Notes.Publish(ctx, note)             // then every follower is woken
```

## Three ways out, one feed

| For | Call | What it sends |
|---|---|---|
| An open page | `Notes.Fragments(w, r, render)` | each item as HTML, which htmx swaps into the list |
| An SDK client | `Notes.Events(ctx, w, positions, seconds, end)` | each item as JSON over server-sent events |
| A WebSocket | `Notes.Lines(ctx, w, positions)` | each item as a line of JSON |

Write a note on [the home page](../) in one tab and it appears in another. Create one through the SDK and it appears there too.

## Resuming

Every event's id is the item's position in the log. A browser sends the last one it had when it reconnects, and an SDK client passes `after`. Nothing in between is lost.
