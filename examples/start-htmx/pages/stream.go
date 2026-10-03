package pages

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/joeblew999/charter/examples/start-htmx/api"
	"github.com/joeblew999/charter/go/follow"
)

// streamFor is how long one stream stays open. Then it ends, and the browser connects again from
// the last message it has (Last-Event-ID): a stream never outlives a deploy by long, and one whose
// browser has gone ends too.
const streamFor = 5 * time.Minute

// serveStream sends every message after the request's position as one SSE event: its id is the
// message's id, the rest is what fields writes. The messages come from go/follow: D1 is the log,
// the hub Durable Object only wakes the stream, so none is lost or sent twice, across reconnects
// and deploys alike. The position is the page's `after`, or the Last-Event-ID a reconnecting
// browser sends, whichever is newer.
//
// fields is the only part that knows the client: htmxEvent for htmx 4's SSE extension. Another
// (Datastar's datastar-patch-elements, say) is another fields over the same stream.
func serveStream(w http.ResponseWriter, r *http.Request, env api.Env, fields func(context.Context, api.Message) (string, error)) {
	feed, options, err := env.Feed(r.URL.Query().Get("after"), r.Header.Get("Last-Event-ID"))
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	out := flushing(w)
	// A comment first: the response starts now, not at the first message.
	if _, err := io.WriteString(out, ": \n\n"); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), streamFor)
	defer cancel()
	err = follow.Follow(ctx, feed, options, func(message api.Message) error {
		event, err := fields(ctx, message)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "id: %d\n%s\n\n", message.ID, event)
		return err
	})
	if err != nil {
		log.Printf("stream: ended: %v", err)
	}
}

// htmxEvent is a message as htmx 4's SSE extension takes it: an event with no name, whose data is
// the HTML to swap in (the list's hx-swap says where). Each line of the HTML is a data line.
func htmxEvent(ctx context.Context, message api.Message) (string, error) {
	var html bytes.Buffer
	if err := Item(message).Render(ctx, &html); err != nil {
		return "", err
	}
	return "data: " + lines.Replace(strings.TrimSpace(html.String())), nil
}

// lines makes every line break of a text (SSE takes CR, LF and CRLF alike) the start of a data line.
var lines = strings.NewReplacer("\r\n", "\ndata: ", "\r", "\ndata: ", "\n", "\ndata: ")

// flushing sends every write on at once (net/http buffers, and on Workers the answer becomes a
// stream at the first Flush).
func flushing(w http.ResponseWriter) io.Writer {
	if f, ok := w.(http.Flusher); ok {
		return flushWriter{w, f}
	}
	return w
}

type flushWriter struct {
	io.Writer
	http.Flusher
}

func (w flushWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.Flush()
	return n, err
}
