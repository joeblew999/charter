package pages

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/joeblew999/charter/examples/start-htmx/api"
	"github.com/joeblew999/charter/fragments"
)

// serveStream sends every message after the request's position as one SSE event (charter/fragments):
// its id is the message's id, the rest is what event writes. The messages come from charter/follow: D1
// is the log, the hub Durable Object only wakes the stream, so none is lost or sent twice, across
// reconnects and deploys alike. The position is the page's `after`, or the Last-Event-ID a
// reconnecting browser sends, whichever is newer.
func serveStream(w http.ResponseWriter, r *http.Request, env api.Env, event func(context.Context, api.Message) (string, error)) {
	feed, options, err := env.Feed(r.URL.Query().Get("after"), r.Header.Get("Last-Event-ID"))
	if err != nil {
		fail(w, err)
		return
	}
	fragments.Stream[api.Message]{Feed: feed, Options: options, Event: event}.ServeHTTP(w, r)
}

// htmxEvent is a message as htmx 4's SSE extension takes it: an event with no name, whose data is
// the HTML to swap in (the list's hx-swap says where). Each line of the HTML is a data line.
func htmxEvent(ctx context.Context, message api.Message) (string, error) {
	var html bytes.Buffer
	if err := Item(message).Render(ctx, &html); err != nil {
		return "", err
	}
	return fragments.Data("", strings.TrimSpace(html.String())), nil
}
