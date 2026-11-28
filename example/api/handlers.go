package api

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/env"
	"github.com/joeblew999/charter/live"
)

// Env is what the handlers take from the Worker: its variables.
type Env struct {
	Var func(name string) string
}

// Notes is the feed of notes, declared once: the table is the log, the Durable Object HUB wakes whoever follows.
// The page's live list, the API's stream and its WebSocket channel are all this.
var Notes = live.Feed[Note]{Binding: env.HUB, Name: "notes", Log: func() (live.Log[Note], error) { return open() }}

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

// OnPost is told of each note written, after it is kept: what else the app does with one (package built queues it
// for the activity page). nil: nothing.
var OnPost func(ctx context.Context, note Note)

// Leaving is what goes with someone's account when they delete it (the account page): the notes they wrote, and
// whatever else the app keeps of theirs (OnLeave). The app's People are told to do this first.
func Leaving(ctx context.Context, who charter.Person) error {
	store, err := open()
	if err != nil {
		return err
	}
	if err := store.RemoveBy(ctx, who.ID); err != nil {
		return err
	}
	removed(ctx)
	if OnLeave != nil {
		return OnLeave(ctx, who)
	}
	return nil
}

// OnLeave is told of someone deleting their account, after their notes are gone: what else of theirs the app
// keeps (package built takes away the files they attached). nil: nothing.
var OnLeave func(ctx context.Context, who charter.Person) error

// MayRemove is the rule for taking a note away, said once: whoever wrote it may, and so may whoever may moderate.
// The page asks it to show the button, the page's route and the API's deleteNote ask it before they act.
func MayRemove(who charter.Caller, note Note) bool {
	return who.May(Moderate) || (who.SignedIn() && who.Subject != "" && who.Subject == note.AuthorID)
}

// Remove takes a note away for whoever is asking, if they may: what the API's deleteNote and the page's button
// both do.
func Remove(ctx context.Context, store Store, by charter.Caller, id int64) error {
	note, err := store.Get(ctx, id)
	if err != nil {
		return app.NotFound("no such note")
	}
	if !MayRemove(by, note) {
		return app.Status(http.StatusForbidden, by.Name()+" may not remove this note: it is someone else's, and removing those needs moderate")
	}
	if err := store.Remove(ctx, id); err != nil {
		return err
	}
	removed(ctx)
	return nil
}

// removed tells whoever shows the notes that some are gone: every open page of them, and what else the app shows
// of a note (OnRemove).
func removed(ctx context.Context) {
	Notes.Changed(ctx)
	if OnRemove != nil {
		OnRemove(ctx)
	}
}

// OnRemove is told when notes are taken away, after they are gone: what else of the app shows something of a note
// (package built's activity page, whose rows of them go too). nil: nothing.
var OnRemove func(ctx context.Context)

// What someone may do: each scope is declared here, once, and named everywhere else (a page's Needs, an
// operation's Security, Caller.May). A scope is a value, not a word, so one that is misspelt does not compile.
// charter.Admin is charter's own: whoever has it says who is what, and sees the admin screen.
var (
	Read     = charter.NewScope("read")
	Write    = charter.NewScope("write")
	Moderate = charter.NewScope("moderate")
)

// Roles are what the app's people may be, and what each may do. Someone who signs up is a member. A moderator
// may also take anyone's note away; an admin may also say who is what.
var Roles = map[string][]charter.Scope{
	"member":    {Read, Write},
	"moderator": {Read, Write, Moderate},
	"admin":     {Read, Write, Moderate, charter.Admin},
}

// Trusted is who may do what (reads are public, writing needs write, removing someone else's note moderate,
// saying who is what admin):
//   - the app's own people, who sign in on its pages with a passkey, each with a role (Roles): on the pages and,
//     with a token they make on their account page, through the API;
//   - ADMIN_TOKEN, the operator's: how the first admin is made (setRole), and what a test promotes people with;
//   - the Worker's secrets (wrangler secret put; .dev.vars locally), each with its scopes: READ_TOKEN may not write;
//   - Cloudflare Access, once it is in front of the Worker: a person who
//     logged in writes, a machine's service token reads (and writes with the write token beside it);
//   - an OpenID Connect issuer (OIDC_ISSUER, OIDC_AUDIENCE): what each token's scope claim says.
//
// An unset secret or setting trusts no one.
var Trusted = []charter.Trusted{
	charter.People{Name: "Notes", Accounts: charter.D1Accounts(env.DB), Roles: Roles, Role: "member", Leaving: Leaving},
	charter.Token{Secret: env.ADMIN_TOKEN, Scopes: Roles["admin"]},
	charter.Token{Secret: env.WRITE_TOKEN, Scopes: []charter.Scope{Read, Write}},
	charter.Token{Secret: env.READ_TOKEN, Scopes: []charter.Scope{Read}},
	charter.Access{People: []charter.Scope{Read, Write}, Machines: []charter.Scope{Read}},
	charter.OIDC{},
}

// Auth is that, for the whole app: the API's operations are checked by it (API), and so are the pages (main.go
// gives it to app.App). So a page that needs Write and the operation that needs Write ask the same thing of
// the same caller.
var Auth = charter.NewAuth(cloud.Var, Trusted...)

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
	store, err := open()
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
	store, err := open()
	if err != nil {
		return nil, err
	}
	who, _ := charter.CallerOf(ctx)
	note, err := Post(ctx, store, who, in.Body.Body)
	if err != nil {
		return nil, err
	}
	return &NoteOutput{Body: note}, nil
}

func (env Env) remove(ctx context.Context, in *NoteInput) (*struct{}, error) {
	store, err := open()
	if err != nil {
		return nil, err
	}
	who, _ := charter.CallerOf(ctx)
	return nil, problem(Remove(ctx, store, who, in.ID))
}

func (env Env) people(ctx context.Context, _ *struct{}) (*PeopleOutput, error) {
	everyone, err := Auth.Everyone(Auth.For(ctx))
	if err != nil {
		return nil, problem(err)
	}
	out := &PeopleOutput{}
	out.Body.Data = make([]Person, len(everyone))
	for i, p := range everyone {
		out.Body.Data[i] = Person{ID: p.ID, Name: p.Name, Role: p.Role}
	}
	return out, nil
}

func (env Env) setRole(ctx context.Context, in *RoleInput) (*PersonOutput, error) {
	if err := Auth.SetRole(Auth.For(ctx), in.ID, in.Body.Role); err != nil {
		return nil, problem(err)
	}
	return &PersonOutput{Body: Person{ID: in.ID, Role: in.Body.Role}}, nil
}

// problem is an error of the app's as the API says it: with its status.
func problem(err error) error {
	if status, message, ok := app.StatusOf(err); ok {
		return huma.NewError(status, message)
	}
	return err
}

// watch is the feed as server-sent events for SDK clients: every event's id is the note id, so `after` and Last-Event-ID are the
// same position. A planned end (`seconds`) sends END, the terminator; if Follow gives up (hub down)
// the stream just ends without it, so the SDKs reconnect by themselves. No error events: generated
// clients would read them as notes.
// Upstream: fern-api/fern#17938 (when fixed: an error event could tell clients why the stream ended)
func (env Env) watch(_ context.Context, in *WatchInput) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "text/event-stream")
		Notes.Events(hc.Context(), hc.SetHeader, hc.BodyWriter(), []string{in.After, in.LastEventID}, int(in.Seconds), END)
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
		Notes.Lines(hc.Context(), hc.SetHeader, hc.BodyWriter(), []string{in.After})
	}}, nil
}
