// Package fragments serves a live feed to browsers as server-sent events, each item rendered by the
// caller: an HTML fragment for htmx 4's SSE extension, a datastar-patch-elements event for Datastar
// (docs/self/pages.md). The feed is go/lib/follow's, so no item is lost or sent twice, across
// reconnects and deploys alike:
//
//	feed, options, err := env.Feed(r.URL.Query().Get("after"), r.Header.Get("Last-Event-ID"))
//	...
//	fragments.Stream[Message]{Feed: feed, Options: options, Event: htmxEvent}.ServeHTTP(w, r)
//
// Every event's id is its item's position, which the browser sends back as Last-Event-ID when it
// connects again; Options.After is where the stream starts.
package fragments

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/joeblew999/charter/go/lib/follow"
	"github.com/joeblew999/charter/go/lib/transport"
)

// streamLife is how long a stream stays open unless Stream.For says otherwise. Then it ends, and the
// browser connects again from the last item it has: a stream never outlives a deploy by long, and
// one whose browser has gone ends too.
const streamLife = 5 * time.Minute

// Stream is one SSE response over a feed.
type Stream[T follow.Item] struct {
	// Feed is the log and its wake-up; Options.After the position to start after (nil: from now on).
	Feed    follow.Source[T]
	Options follow.Options
	// Event writes an item as the fields of its event, after its id: the only part that knows the
	// client. Data builds the data lines.
	Event func(ctx context.Context, item T) (string, error)
	// For is how long the stream stays open (default streamLife: 5 minutes).
	For time.Duration
}

// ServeHTTP sends every item of the feed after Options.After as one event, flushed as it is written,
// until For has passed, the browser goes, or the feed or Event fails (logged).
func (s Stream[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lifetime := s.For
	if lifetime <= 0 {
		lifetime = streamLife
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	out := transport.Flushing(w)
	// A comment first: the response starts now, not at the first item.
	if _, err := io.WriteString(out, ": \n\n"); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), lifetime)
	defer cancel()
	err := follow.Follow(ctx, s.Feed, s.Options, func(item T) error {
		event, err := s.Event(ctx, item)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "id: %d\n%s\n\n", item.Position(), event)
		return err
	})
	if err != nil {
		log.Printf("fragments: stream ended: %v", err)
	}
}

// Data is text as data lines: each of its lines (SSE takes CR, LF and CRLF alike as line breaks)
// becomes "data: " + prefix + the line. Data("", html) is htmx's; Data("elements ", html) Datastar's.
func Data(prefix, text string) string {
	start := "data: " + prefix
	return start + strings.NewReplacer("\r\n", "\n"+start, "\r", "\n"+start, "\n", "\n"+start).Replace(text)
}
