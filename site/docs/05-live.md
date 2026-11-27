# The live feed

A feed is declared once and serves everything that shows it.

```go file=../example/api/handlers.go#Notes
// Notes is the feed of notes, declared once: the table is the log, the Durable Object HUB wakes whoever follows.
// The page's live list, the API's stream and its WebSocket channel are all this.
var Notes = live.Feed[Note]{Binding: "HUB", Name: "notes", Log: func() (live.Log[Note], error) { return open() }}
```

The log, a D1 table, is the truth. The hub, a Durable Object, only wakes whoever follows. So no item is lost or sent twice, across reconnects and deploys alike.

A change that is no new item is said too, so that every open page shows it:

```go
if err := store.Remove(ctx, id); err != nil {
	return err
}
Notes.Changed(ctx)
```

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
| An open page | `Notes.Signal()`, `live.OnChange` and `live.Again(address)` | "what you show has changed": the page asks for that part of itself again, as that reader sees it |
| An SDK client | `Notes.Events(ctx, setHeader, w, positions, seconds, end)` | each item as JSON over server-sent events |
| A WebSocket | `Notes.Lines(ctx, setHeader, w, positions)` | each item as a line of JSON |

Write a note on [the home page](../) in one tab and it appears in another; take it away and it goes from both. Create one through the SDK and it appears there too.

## A page's stream holds no Go

A Go runtime takes 13 MB or more of a Worker's [128 MB](https://developers.cloudflare.com/workers/platform/limits/#memory) for as long as it serves a request, and a stream is a request that lasts minutes. Served by Go, a handful of open pages took a Worker over its memory, and Cloudflare ended it with every request it was answering (a 503 for each; `exceededMemory` in its analytics).

So a page's stream is the hub's, a small Durable Object in JavaScript, and it says one thing: what you show has changed. The part of the page that shows the feed then asks for itself again, and what comes takes the place of what it had.

```gsx
<div hx-sse:connect={api.Notes.Signal()}>
	<ul
		id={Home.List |> id}
		hx-trigger={live.OnChange}
		hx-get={again(ctx, Home{})}
		hx-target={Home.List |> target}
		hx-swap="innerMorph"
		hx-sync="this:replace"
	>
		<p.List notes={props.Notes} reader={props.Caller}/>
	</ul>
</div>
```

```go
// again is a page's own address as the part of it that shows a feed asks it when the feed says something changed.
func again(ctx context.Context, page any) (string, error) {
	address, err := structpages.URLFor(ctx, page)
	return live.Again(address), err
}
```

There is no route for it: the list is a component of its page, so the request is for the page with the list as its target, and structpages renders the list alone (`target.Is(p.List)` in the page's `Props`). It is a short, ordinary request: Go renders the rows for that reader, in their language, with the buttons they may use.

Why it is told that something changed, and never what:

- **Added, taken away or altered, it is one thing.** The page shows what the log has, so nothing is shown twice or left behind.
- **Changes that come together end right.** The last request is the one that counts (`hx-sync="this:replace"`): however many arrive at once, the page ends as the log is.
- **What the reader was doing stays.** The swap is a morph (`innerMorph`): a row they have opened stays open, and each row is matched by its id.
- **It reads what is newest.** `live.Again` marks the request (`?newest=1`), so it is answered by the database's primary: a copy that is behind would show the page as it was before the change it was just told of ([reads near the reader](06-database.md#reads-near-the-reader)).

The stream is on an element of its own, around the list: htmx opens a stream on that element's trigger, and what is replaced is inside it, so the stream stays open across every change.

Nothing is lost when a stream breaks, which every one does when the Worker is deployed: the hub restarts and holds nothing. A page says when it was rendered, and a stream that connects again says when it was last told; the hub answers "changed" at once when something changed since, or when it started since and cannot know.

The API's streams hold no Go either. `Events` (an SDK client's stream) and `Lines` (a WebSocket channel) write what the log has after the position their caller gave, say where they stopped, and are done; the hub writes the rest, as Go would have: an event for each item and the terminator at the end, or a line for each. Your handler gives them the answer's `SetHeader`, which is how they say it:

```go
Notes.Events(hc.Context(), hc.SetHeader, hc.BodyWriter(), []string{in.After, in.LastEventID}, int(in.Seconds), END)
```

Twenty SDK clients watching for a quarter of a minute hold no Go runtime between them.

## Resuming

An API stream's events each have the item's position in the log as their id: an SDK client passes `after`, or its last event's id, and is sent what came after. A page's stream has the time as its id: a browser sends the last one when it connects again, and is told at once if anything changed since. The log is the truth, so nothing in between is lost.
