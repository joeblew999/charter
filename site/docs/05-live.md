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
	if OnPost != nil {
		OnPost(ctx, note)
	}
	return note, nil
}
```

## Three ways out, one feed

| For | Call | What it sends |
|---|---|---|
| An open page | `Notes.Signal(since, after, "li")` and `Notes.Since(ctx, after, limit)` | "there is something new", and then the rows, as that reader sees them |
| An SDK client | `Notes.Events(ctx, setHeader, w, positions, seconds, end)` | each item as JSON over server-sent events |
| A WebSocket | `Notes.Lines(ctx, setHeader, w, positions)` | each item as a line of JSON |

Write a note on [the home page](../) in one tab and it appears in another. Create one through the SDK and it appears there too.

## A page's stream holds no Go

A Go runtime takes 13 MB or more of a Worker's 128 MB for as long as it serves a request, and a stream is a request that lasts minutes. Served by Go, a handful of open pages took a Worker over its memory, and Cloudflare ended it with every request it was answering (a 503 for each; `exceededMemory` in its analytics).

So a page's stream is the hub's, a small Durable Object in JavaScript, and it says one thing: there is something after the position you have, ask for it.

```gsx
<ul hx-sse:connect={notesSignal(ctx, props.After)} hx-swap="afterbegin">
```

```go
func notesSignal(ctx context.Context, after int64) (string, error) {
	since, err := structpages.URLFor(ctx, NotesSince{})
	return api.Notes.Signal(since, after, "li"), err
}

func (NotesSince) ServeHTTP(w http.ResponseWriter, r *http.Request, caller charter.Caller) error {
	notes, err := api.Notes.Since(r.Context(), r.URL.Query().Get("after"), 50)
	if err != nil {
		return err
	}
	return structpages.RenderComponent(Home{}.List(notes, caller))
}
```

Each event of the stream is one hidden element that htmx puts where the items go, and that asks `NotesSince` for itself. That is a short, ordinary request: Go renders the new rows for that reader, in their language, with the buttons they may use, and they take the element's place. A thousand open pages cost the Worker nothing while nothing happens.

The API's streams hold no Go either. `Events` (an SDK client's stream) and `Lines` (a WebSocket channel) write what the log has after the position their caller gave, say where they stopped, and are done; the hub writes the rest, as Go would have: an event for each item and the terminator at the end, or a line for each. Your handler gives them the answer's `SetHeader`, which is how they say it:

```go
Notes.Events(hc.Context(), hc.SetHeader, hc.BodyWriter(), []string{in.After, in.LastEventID}, int(in.Seconds), END)
```

Twenty SDK clients watching for a quarter of a minute hold no Go runtime between them.

## Resuming

Every event's id is the item's position in the log. A browser sends the last one it was told of when it reconnects, and is told at once to ask from there; an SDK client passes `after`. The log is the truth, so nothing in between is lost.
