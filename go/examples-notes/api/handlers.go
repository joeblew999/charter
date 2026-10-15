package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/lib/auth"
	"github.com/joeblew999/charter/go/lib/follow"
	"github.com/joeblew999/charter/go/lib/transport"
)

// Env is what the platform supplies: bindings on Cloudflare (platform_js.go), memory elsewhere.
// Store and Hub are opened per request: a binding belongs to the request's environment.
type Env struct {
	Var   func(name string) string
	Store func() (Store, error)
	Hub   func() (Hub, error)
}

// Trusted is who may do what (reads are public, writing needs write):
//   - the Worker's secrets (secrets.ts), each with its scopes: READ_TOKEN may not write;
//   - Cloudflare Access, once it is in front of the Worker: a person who
//     logged in writes, a machine's service token reads (and writes with the write token beside it);
//   - an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): what each token's scope claim says.
//
// An unset secret or setting trusts no one.
var Trusted = []auth.Trusted{
	auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
	auth.Token{Secret: "READ_TOKEN", Scopes: []string{"read"}},
	auth.Access{People: []string{"read", "write"}, Machines: []string{"read"}},
	auth.OIDC{},
}

// Handler serves the contract on env the standard way (go/lib/serve): each call allowed (auth) and
// limited (ratelimit) as its operation says, the specs at /api/openapi.json and /api/asyncapi.json,
// the operations as MCP tools at /api/mcp, and how to get in at /.well-known/.
func Handler(env Env) http.Handler {
	return API(env).Handler()
}

func (env Env) hello(context.Context, *struct{}) (*HelloOutput, error) {
	out := &HelloOutput{}
	out.Body.Message = "Hello from " + env.Var("APP_NAME")
	return out, nil
}

func (env Env) list(ctx context.Context, in *ListInput) (*ListOutput, error) {
	// Cursors are opaque strings to callers (here, the last id seen).
	before := int64(math.MaxInt64)
	if in.Cursor != "" {
		id, err := strconv.ParseInt(in.Cursor, 10, 64)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "query.cursor", Message: "not a cursor from next_cursor", Value: in.Cursor})
		}
		before = id
	}
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	limit := int(in.Limit)
	notes, err := store.Before(ctx, before, limit+1)
	if err != nil {
		return nil, err
	}
	out := &ListOutput{}
	if len(notes) > limit {
		notes = notes[:limit]
		out.Body.NextCursor = strconv.FormatInt(notes[len(notes)-1].ID, 10)
	}
	out.Body.Data = notes
	return out, nil
}

// Resolve is the body rule Huma's tags can't say: no note may contain the stream terminator.
func (in *CreateInput) Resolve(huma.Context) []error {
	if strings.Contains(in.Body.Body, END) {
		return []error{&huma.ErrorDetail{Location: "body.body", Message: "must not contain " + END, Value: in.Body.Body}}
	}
	return nil
}

func (env Env) create(ctx context.Context, in *CreateInput) (*NoteOutput, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	note, err := store.Create(ctx, in.Body.Body)
	if err != nil {
		return nil, err
	}
	// The hub is only a wake-up: if it misses this note, followers find it in the log (Recheck).
	if hub, err := env.Hub(); err != nil {
		log.Printf("create: no hub, note %d is not announced: %v", note.ID, err)
	} else if err := hub.Publish(ctx, note); err != nil {
		log.Printf("create: note %d is not announced: %v", note.ID, err)
	}
	return &NoteOutput{Body: note}, nil
}

// feed is the one feed both transports serve (docs/self/streaming.md): the store is the log, the hub
// only wakes followers.
type feed struct {
	store Store
	hub   Hub
}

func (f feed) Subscribe(listener func(Note), onError func(error)) (func(), error) {
	return f.hub.Subscribe(listener, onError)
}
func (f feed) Since(ctx context.Context, after int64, limit int) ([]Note, error) {
	return f.store.Since(ctx, after, limit)
}
func (f feed) Latest(ctx context.Context) (int64, error) { return f.store.Latest(ctx) }

// feed opens the feed and reads the resume position: the newest of the positions given (a
// reconnecting SDK resends its original `after` plus a newer Last-Event-ID). None: from now on.
func (env Env) feed(positions ...string) (feed, follow.Options, error) {
	options := follow.Options{After: follow.Newest(positions...), OnBroken: func(err error) { log.Printf("follow: hub subscription broken, resubscribing: %v", err) }}
	store, err := env.Store()
	if err != nil {
		return feed{}, options, err
	}
	hub, err := env.Hub()
	if err != nil {
		return feed{}, options, err
	}
	return feed{store, hub}, options, nil
}

// watch is SSE over Follow: every event's id is the note id, so `after` and Last-Event-ID are the
// same position. A planned end (`seconds`) sends END, the terminator; if Follow gives up (hub down)
// the stream just ends without it, so the SDKs reconnect by themselves. No error events: generated
// clients would read them as notes.
// Upstream: fern-api/fern#17938 (when fixed: an error event could tell clients why the stream ended)
func (env Env) watch(_ context.Context, in *WatchInput) (*huma.StreamResponse, error) {
	notes, options, err := env.feed(in.After, in.LastEventID)
	if err != nil {
		return nil, err
	}
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "text/event-stream")
		w := transport.Flushing(hc.BodyWriter())
		// A comment first: the response starts now, not at the first note.
		if _, err := io.WriteString(w, ": \n\n"); err != nil {
			return
		}
		until, cancel := context.WithTimeout(hc.Context(), time.Duration(in.Seconds)*time.Second)
		defer cancel()
		err := follow.Follow(until, notes, options, func(note Note) error {
			data, err := json.Marshal(note)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(w, "event: message\nretry: 1000\nid: %d\ndata: %s\n\n", note.ID, data)
			return err
		})
		if err != nil {
			log.Printf("watch: follow gave up, ending without terminator: %v", err)
			return
		}
		if hc.Context().Err() == nil {
			fmt.Fprintf(w, "event: close\ndata: %q\n\n", END)
		}
	}}, nil
}

// live is the WebSocket channel's feed (AsyncAPI channel liveNotes). workers-go can't answer a
// WebSocket upgrade itself, so it answers with lines, one JSON note each, and the WebSocket adapter
// (go/lib/transport) sends each line as a frame. The stream only ends when Follow gives up (hub
// down); the adapter then closes the socket with 1011, so the client reconnects with `after`.
func (env Env) live(_ context.Context, in *LiveInput) (*huma.StreamResponse, error) {
	if !strings.EqualFold(in.Upgrade, "websocket") {
		return nil, huma.NewError(http.StatusUpgradeRequired, "expected a WebSocket upgrade")
	}
	notes, options, err := env.feed(in.After)
	if err != nil {
		return nil, err
	}
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "application/x-ndjson")
		w := transport.Flushing(hc.BodyWriter())
		// An empty line first: the response (and so the 101) goes out now, not at the first note.
		if _, err := io.WriteString(w, "\n"); err != nil {
			return
		}
		err := follow.Follow(hc.Context(), notes, options, func(note Note) error {
			data, err := json.Marshal(note)
			if err != nil {
				return err
			}
			_, err = w.Write(append(data, '\n'))
			return err
		})
		if err != nil {
			log.Printf("live: follow gave up: %v", err)
		}
	}}, nil
}
