package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/joeblew999/charter/app"
)

// Log is a feed's source of truth: the table its items are written to, read by position.
type Log[T Item] interface {
	// Since is up to limit items after the position, oldest first.
	Since(ctx context.Context, after int64, limit int) ([]T, error)
	// Latest is the newest position, 0 for an empty log.
	Latest(ctx context.Context) (int64, error)
}

// Feed is a log that is followed live, declared once and used by everything that shows it: the pages (Fragments),
// the API's stream for SDK clients (Events) and its WebSocket channel (Lines). The log (a D1 table) is the truth; the
// hub (a Durable Object) only wakes followers, so no item is lost or sent twice, across reconnects and deploys alike.
//
//	var Notes = live.Feed[Note]{Binding: "HUB", Name: "notes", Log: func() (live.Log[Note], error) { return openStore() }}
//
//	note, err := store.Create(ctx, body)   // the write goes to the log
//	Notes.Publish(ctx, note)               // then every follower is woken
type Feed[T Item] struct {
	// Binding is the Durable Object namespace of the hub (wrangler.jsonc's durable_objects), Name the object in it:
	// one object per feed.
	Binding, Name string
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

// lifetime is how long a page's stream stays open. Then it ends, and the browser connects again from the last item
// it has: a stream never outlives a deploy by long, and one whose browser has gone ends too.
const lifetime = 5 * time.Minute

// Fragments answers a page's stream: every item as one server-sent event whose data is the HTML render gives, which
// htmx 4's SSE extension swaps in. It starts after ?after= (the newest item the page was rendered with) or the
// Last-Event-ID a reconnecting browser sends, whichever is newer.
func (f Feed[T]) Fragments(w http.ResponseWriter, r *http.Request, render func(ctx context.Context, item T) (html string, err error)) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	out := app.Flushing(w)
	// A comment first: the response starts now, not at the first item.
	if _, err := io.WriteString(out, ": \n\n"); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), lifetime)
	defer cancel()
	err := f.Follow(ctx, []string{r.URL.Query().Get("after"), r.Header.Get("Last-Event-ID")}, func(item T) error {
		html, err := render(ctx, item)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "id: %d\n%s\n\n", item.Position(), Data("", html))
		return err
	})
	if err != nil {
		log.Printf("live: %s: page stream ended: %v", f.Name, err)
	}
}

// Events writes the API's stream for SDK clients: every item as a server-sent event `message` whose data is its JSON
// and whose id is its position, for `seconds`. A planned end sends the event `close` with end as its data, the
// terminator generated clients stop at; if the feed gives up, the stream ends without it and they reconnect.
func (f Feed[T]) Events(ctx context.Context, w io.Writer, positions []string, seconds int, end string) {
	out := app.Flushing(w)
	if _, err := io.WriteString(out, ": \n\n"); err != nil {
		return
	}
	until, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	err := f.Follow(until, positions, func(item T) error {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "event: message\nretry: 1000\nid: %d\ndata: %s\n\n", item.Position(), data)
		return err
	})
	if err != nil {
		log.Printf("live: %s: API stream gave up, ending without its terminator: %v", f.Name, err)
		return
	}
	if ctx.Err() == nil {
		fmt.Fprintf(out, "event: close\ndata: %q\n\n", end)
	}
}

// Lines writes the feed as lines of JSON, one item each, until ctx ends: what a WebSocket channel answers with, each
// line sent as a frame (worker/websocket.mjs).
func (f Feed[T]) Lines(ctx context.Context, w io.Writer, positions []string) {
	out := app.Flushing(w)
	// An empty line first: the response (and so the 101) goes out now, not at the first item.
	if _, err := io.WriteString(out, "\n"); err != nil {
		return
	}
	err := f.Follow(ctx, positions, func(item T) error {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		_, err = out.Write(append(data, '\n'))
		return err
	})
	if err != nil {
		log.Printf("live: %s: channel gave up: %v", f.Name, err)
	}
}
