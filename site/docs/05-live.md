# The live feed

A feed is declared once and serves everything that shows it.

```go file=../example/api/handlers.go#Notes
// Notes is the feed of notes, declared once: the table is the log, the Durable Object HUB wakes whoever follows.
// The page's live list, the API's stream and its WebSocket channel are all this.
var Notes = live.Feed[Note]{Binding: "HUB", Name: "notes", Log: func() (live.Log[Note], error) { return open() }}
```

The log, a D1 table, is the truth. The hub, a Durable Object, only wakes whoever follows. So no item is lost or sent twice, across reconnects and deploys alike.

## Writing

```go file=../example/api/handlers.go#Post
// Post writes a note to the store, as whoever is asking, and wakes every follower: what the API's createNote and
// the page's form both do.
func Post(ctx context.Context, store Store, by charter.Caller, body string) (Note, error) {
	note, err := store.Create(ctx, body, by.Subject, by.Name())
	if err != nil {
		return Note{}, err
	}
	Notes.Publish(ctx, note)
	return note, nil
}
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
