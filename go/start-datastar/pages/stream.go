package pages

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/joeblew999/charter/examples/start-datastar/api"
	"github.com/joeblew999/charter/go/fragments"
)

// serveStream sends every message after the request's position as one SSE event (go/fragments):
// its id is the message's id, the rest is what event writes. The messages come from go/follow: D1
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

// datastarEvent is a message as Datastar takes it: a datastar-patch-elements event that puts its
// Item at the top of the list (selector, mode prepend). Each line of the HTML is an elements line.
func datastarEvent(ctx context.Context, message api.Message) (string, error) {
	var html bytes.Buffer
	if err := Item(message).Render(ctx, &html); err != nil {
		return "", err
	}
	return "event: datastar-patch-elements\ndata: selector #messages\ndata: mode prepend\n" + fragments.Data("elements ", strings.TrimSpace(html.String())), nil
}
