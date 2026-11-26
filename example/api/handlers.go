package api

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/live"
)

// Env is what the handlers take from the Worker: its variables.
type Env struct {
	Var func(name string) string
}

// Notes is the feed of notes, declared once: the table is the log, the Durable Object HUB wakes whoever follows.
// The page's live list, the API's stream and its WebSocket channel are all this.
var Notes = live.Feed[Note]{Binding: "HUB", Name: "notes", Log: func() (live.Log[Note], error) { return Open() }}

// Post writes a note and wakes every follower: what the API's createNote and the page's form both do.
func Post(ctx context.Context, body string) (Note, error) {
	store, err := Open()
	if err != nil {
		return Note{}, err
	}
	note, err := store.Create(ctx, body)
	if err != nil {
		return Note{}, err
	}
	Notes.Publish(ctx, note)
	return note, nil
}

// Trusted is who may do what (reads are public, writing needs write):
//   - the Worker's secrets (wrangler secret put; .dev.vars locally), each with its scopes: READ_TOKEN may not write;
//   - Cloudflare Access, once it is in front of the Worker: a person who
//     logged in writes, a machine's service token reads (and writes with the write token beside it);
//   - an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): what each token's scope claim says.
//
// An unset secret or setting trusts no one.
var Trusted = []charter.Trusted{
	charter.Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
	charter.Token{Secret: "READ_TOKEN", Scopes: []string{"read"}},
	charter.Access{People: []string{"read", "write"}, Machines: []string{"read"}},
	charter.OIDC{},
}

// Handler serves the contract on env the standard way (charter/serve): each call allowed (auth) and
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
	store, err := Open()
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
	note, err := Post(ctx, in.Body.Body)
	if err != nil {
		return nil, err
	}
	return &NoteOutput{Body: note}, nil
}

// watch is the feed as server-sent events for SDK clients: every event's id is the note id, so `after` and Last-Event-ID are the
// same position. A planned end (`seconds`) sends END, the terminator; if Follow gives up (hub down)
// the stream just ends without it, so the SDKs reconnect by themselves. No error events: generated
// clients would read them as notes.
// Upstream: fern-api/fern#17938 (when fixed: an error event could tell clients why the stream ended)
func (env Env) watch(_ context.Context, in *WatchInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "text/event-stream")
		Notes.Events(hc.Context(), hc.BodyWriter(), []string{in.After, in.LastEventID}, int(in.Seconds), END)
	}}, nil
}

// live is the WebSocket channel's feed (AsyncAPI channel liveNotes). workers-go can't answer a WebSocket upgrade
// itself, so it answers with lines, one JSON note each, and charter's WebSocket adapter sends each line as a frame.
// The stream only ends when the feed gives up; the adapter then closes the socket with 1011, so the client reconnects
// with `after`.
func (env Env) live(_ context.Context, in *LiveInput) (*huma.StreamResponse, error) {
	if !strings.EqualFold(in.Upgrade, "websocket") {
		return nil, huma.NewError(http.StatusUpgradeRequired, "expected a WebSocket upgrade")
	}
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "application/x-ndjson")
		Notes.Lines(hc.Context(), hc.BodyWriter(), []string{in.After})
	}}, nil
}
