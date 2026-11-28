package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
)

// Log is a feed's source of truth: the table its items are written to, read by position.
type Log[T Item] interface {
	// Since is up to limit items after the position, oldest first.
	Since(ctx context.Context, after int64, limit int) ([]T, error)
	// Latest is the newest position, 0 for an empty log.
	Latest(ctx context.Context) (int64, error)
}

// Feed is a log that is followed live, declared once and used by everything that shows it: the pages (Stream and Follows),
// the API's stream for SDK clients (Events) and its WebSocket channel (Lines). The log (a D1 table) is the truth; the
// hub (a Durable Object) only wakes followers, so no item is lost or sent twice, across reconnects and deploys alike.
//
//	var Notes = live.Feed[Note]{Binding: env.HUB, Name: "notes", Log: func() (live.Log[Note], error) { return openStore() }}
//
//	note, err := store.Create(ctx, body)   // the write goes to the log
//	Notes.Publish(ctx, note)               // then every follower is woken
type Feed[T Item] struct {
	// Binding is the Durable Object namespace of the hub (wrangler.jsonc's durable_objects: the app's env.HUB),
	// Name the object in it: one object per feed.
	Binding cloud.Object
	Name    string
	// Log opens the log. It is called per request: a binding belongs to the request's environment.
	Log func() (Log[T], error)
	// Hub replaces the Durable Object, for a test: a Memory.
	Hub Hub[T]
}

func (f Feed[T]) hub() (Hub[T], error) {
	if f.Hub != nil {
		return f.Hub, nil
	}
	return DurableObject[T](f.Binding, f.Name)
}

// Publish wakes every follower with item, which the log already has. A hub that cannot be reached is logged, not
// returned: followers find the item in the log a moment later, and the write has happened.
func (f Feed[T]) Publish(ctx context.Context, item T) {
	hub, err := f.hub()
	if err == nil {
		err = hub.Publish(ctx, item)
	}
	if err != nil {
		log.Printf("live: %s: item %d is not announced: %v", f.Name, item.Position(), err)
	}
}

// source is the feed as Follow reads it.
type source[T Item] struct {
	Log[T]
	hub Hub[T]
}

func (s source[T]) Subscribe(listener func(T), onError func(error)) (func(), error) {
	return s.hub.Subscribe(listener, onError)
}

// Follow calls emit for every item after the newest of positions (a client's `after`, a reconnecting one's
// Last-Event-ID; none: from now on), in order, until ctx ends or emit fails.
func (f Feed[T]) Follow(ctx context.Context, positions []string, emit func(T) error) error {
	feedLog, err := f.Log()
	if err != nil {
		return err
	}
	hub, err := f.hub()
	if err != nil {
		return err
	}
	options := Options{After: Newest(positions...), OnBroken: func(err error) {
		log.Printf("live: %s: hub subscription broken, resubscribing: %v", f.Name, err)
	}}
	return Follow(ctx, source[T]{feedLog, hub}, options, emit)
}

// Changed tells every open page that what it shows has changed, for a change that is no new item: one that was
// taken away, one that was altered. A new item is Publish's, which tells the pages too. Like Publish, a hub that
// cannot be reached is logged and not returned: the change has happened.
//
//	if err := store.Remove(ctx, id); err != nil { … }
//	Notes.Changed(ctx)
func (f Feed[T]) Changed(ctx context.Context) {
	hub, err := f.hub()
	if err == nil {
		err = hub.Changed(ctx)
	}
	if err != nil {
		log.Printf("live: %s: a change is not announced: %v", f.Name, err)
	}
}

// SignalPath begins the address of a page's stream: the Worker's JavaScript (worker/go.mjs) gives what is under it
// to the hub, and no Go runtime waits on it.
const SignalPath = "/__charter/live/"

// A page that shows a feed keeps itself current with two spreads, both from here:
//
//	<div { api.Notes.Stream()... }>
//		<ul
//			id={ Home.List |> id }
//			hx-get={ Home{} |> url }
//			hx-target={ Home.List |> target }
//			{ live.Follows()... }
//		>
//			<p.List notes={ props.Notes } reader={ props.Caller }/>
//		</ul>
//	</div>
//
// Stream is the stream, on an element around the part of the page that shows the feed. Follows makes that part
// ask for itself again when the stream says something changed, and put what comes in place of what it had. The
// part is a component of its page (Home.List), so what it asks for is the page, with itself as the target, and
// the page's Props answers for it (target.Is(p.List)): there is no route of its own. Its id, its target and the
// page's address are the tree's, written in the markup as every other is, where structpages' linter sees them.
//
// The page is told that something changed, never what. So an item added, one taken away and one altered reach it
// the same way; nothing is shown twice or left behind; and when changes come together, the last request is the one
// that counts, and the page ends as the log is. What comes is morphed into place, so a row the reader has open, or
// is typing in, stays as it is.
//
// The stream is on its own element because htmx opens a stream on that element's own trigger, and replaces nothing
// there: what is replaced is inside it, so the stream stays open across every change.
//
// A page's stream is not a Go handler, on purpose. A Go runtime holds some 13 MB or more of a Worker's 128 MB for
// as long as it serves a request, and a stream is a request that lasts minutes: a handful of open pages took a
// Worker over its memory. What a reader gets is still theirs alone: the request the page makes is an ordinary one.

// changedEvent is the one event a page's stream sends (the hub's CHANGED, app/worker/hub.mjs): what has changed is
// not said, only that something has.
const changedEvent = "changed"

// Stream is the attributes of the element around the part of a page that shows this feed: the page's stream, which
// the hub holds. Spread on that element: <div { api.Notes.Stream()... }>.
func (f Feed[T]) Stream() gsx.Attrs {
	return gsx.Attrs{{Key: "hx-sse:connect", Value: f.Signal()}}
}

// Follows is what makes the part of a page that shows a feed keep itself current, inside the element that has the
// feed's Stream: spread on that part, beside its own id, hx-get (its page) and hx-target (itself).
func Follows() gsx.Attrs {
	return gsx.Attrs{
		// When: the stream, which belongs to an element around this one, says something changed. (htmx takes a
		// source with a space in it in quotes.)
		{Key: "hx-trigger", Value: changedEvent + ` from:"closest [hx-sse\:connect]"`},
		// A morph: a row the reader has open stays open, and each row is matched by its id.
		{Key: "hx-swap", Value: "innerMorph"},
		// Of requests that cross, the last is the one that counts.
		{Key: "hx-sync", Value: "this:replace"},
		// What it reads must be the newest: a copy of the database that is behind would show the page as it was
		// before the change it was just told of (cloud.NewestHeader, which app.Serve reads).
		{Key: "hx-headers", Value: `{"` + cloud.NewestHeader + `":"1"}`},
	}
}

// Signal is the address of a page's stream of this feed: what Stream connects to. It says when the page was
// rendered, so a change between that and the stream's opening is told at once.
func (f Feed[T]) Signal() string {
	return SignalPath + url.PathEscape(f.Binding.Name()) + "/" + url.PathEscape(f.Name) + "?at=" + strconv.FormatInt(time.Now().UnixMilli(), 10)
}

// FollowHeader is the header of an answer whose stream the hub carries on (worker/go.mjs joins it): what Events
// and Lines set on Cloudflare, where no Go runtime waits on a stream.
const FollowHeader = "X-Charter-Follow"

// Events writes the API's stream for SDK clients: every item as a server-sent event `message` whose data is its JSON
// and whose id is its position, for `seconds`. A planned end sends the event `close` with end as its data, the
// terminator generated clients stop at; if the feed gives up, the stream ends without it and they reconnect.
//
// On Cloudflare it writes what the log has after the positions and is done: header sets FollowHeader on the answer
// (a huma.Context's SetHeader), and the hub writes the rest, for the seconds and with the terminator. A Go runtime
// held for a stream's seconds, one for each client, is what takes a Worker over its memory.
func (f Feed[T]) Events(ctx context.Context, header func(name, value string), w io.Writer, positions []string, seconds int, end string) {
	event := func(out io.Writer, item T) error {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "event: message\nretry: 1000\nid: %d\ndata: %s\n\n", item.Position(), data)
		return err
	}
	if f.Hub == nil {
		f.handOver(ctx, header, w, positions, "events", seconds, fmt.Sprintf("%q", end), ": \n\n", event)
		return
	}
	out := app.Flushing(w)
	if _, err := io.WriteString(out, ": \n\n"); err != nil {
		return
	}
	until, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	if err := f.Follow(until, positions, func(item T) error { return event(out, item) }); err != nil {
		log.Printf("live: %s: API stream gave up, ending without its terminator: %v", f.Name, err)
		return
	}
	if ctx.Err() == nil {
		fmt.Fprintf(out, "event: close\ndata: %q\n\n", end)
	}
}

// Lines writes the feed as lines of JSON, one item each, until ctx ends: what a WebSocket channel answers with, each
// line sent as a frame (worker/websocket.mjs). On Cloudflare it writes what the log has and hands the rest to the
// hub, as Events does: a socket stays open for as long as its client likes, and no Go runtime waits on it.
func (f Feed[T]) Lines(ctx context.Context, header func(name, value string), w io.Writer, positions []string) {
	line := func(out io.Writer, item T) error {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		_, err = out.Write(append(data, '\n'))
		return err
	}
	// An empty line first: the response (and so the 101) goes out now, not at the first item.
	if f.Hub == nil {
		f.handOver(ctx, header, w, positions, "lines", 0, "", "\n", line)
		return
	}
	out := app.Flushing(w)
	if _, err := io.WriteString(out, "\n"); err != nil {
		return
	}
	if err := f.Follow(ctx, positions, func(item T) error { return line(out, item) }); err != nil {
		log.Printf("live: %s: channel gave up: %v", f.Name, err)
	}
}

// handOver writes the start of a stream and gives the rest to the hub: the header that says where the hub takes
// over (set before anything is written), what begins the stream, and every item the log has after the positions,
// in pages. The hub then sends what is published after the last of them.
func (f Feed[T]) handOver(ctx context.Context, header func(name, value string), w io.Writer, positions []string, as string, seconds int, end, begin string, write func(io.Writer, T) error) {
	feedLog, err := f.Log()
	if err != nil {
		log.Printf("live: %s: the log is not there: %v", f.Name, err)
		return
	}
	// Where the stream starts: after the newest position its caller has, or from what is new. It is read where
	// the database is newest: the hub carries on from where this ends, so what a copy that is behind left out
	// would never reach the stream's reader.
	cloud.Newest()
	var last int64
	if after := Newest(positions...); after != nil {
		last = *after
	} else if last, err = feedLog.Latest(ctx); err != nil {
		log.Printf("live: %s: %v", f.Name, err)
		return
	}
	// The log's items first, read before a byte is written: the header must say where they end.
	var items []T
	for {
		page, err := feedLog.Since(ctx, last, 100)
		if err != nil {
			log.Printf("live: %s: %v", f.Name, err)
			return
		}
		for _, item := range page {
			if item.Position() > last {
				last = item.Position()
				items = append(items, item)
			}
		}
		if len(page) < 100 {
			break
		}
	}
	follow := url.Values{"binding": {f.Binding.Name()}, "name": {f.Name}, "after": {strconv.FormatInt(last, 10)}, "as": {as}}
	if seconds > 0 {
		follow.Set("seconds", strconv.Itoa(seconds))
		follow.Set("end", end)
	}
	header(FollowHeader, follow.Encode())
	if _, err := io.WriteString(w, begin); err != nil {
		return
	}
	for _, item := range items {
		if err := write(w, item); err != nil {
			return
		}
	}
}
