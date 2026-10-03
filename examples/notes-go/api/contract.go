// Package api is the notes API, contract first, in Go. Every route is a Huma operation whose input and output are Go structs; the
// struct tags are the schema (as Zod is for oRPC). From this one definition come the handlers'
// validation, the OpenAPI spec and the AsyncAPI spec (spec.go), from which Fern makes SDKs, a CLI
// and docs. Everything the SDKs need is said here too: OperationID and Tags name the SDK methods,
// and Extensions carry Fern's x-fern-*. The `example` tags are what the SDKs' READMEs and references
// show: without one Fern makes a value up, which a pattern or a bound then refuses
// (TestExamplesAreThereAndValid).
package api

import (
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/asyncapi"
	"github.com/joeblew999/charter/go/auth"
	"github.com/joeblew999/charter/go/humaworkers"
)

// What the specs say about the API as a whole.
const (
	Title       = "charter-notes-go"
	Version     = "1.0.0"
	Description = "Notes API: Huma contract (Go) -> OpenAPI -> Fern."
	LiveTitle   = "charter-notes-go live"
)

// END is what a planned stream end returns: SSE `event: close` with `data: "[end-of-stream]"`,
// Fern's terminator for watch. Fern matches the terminator as a substring of each event's data, so
// no note may contain it (note bodies reject it), and it is plain text because Fern's Rust generator
// pastes it into source unescaped.
// Upstream: fern-api/fern#17936 (when fixed: terminators match whole data values, so the body rule can go)
// Upstream: fern-api/fern#17939 (when fixed: the Rust generator escapes it, so any text would do)
const END = "[end-of-stream]"

// Note is the one resource.
type Note struct {
	ID        int64  `json:"id" example:"42"`
	Body      string `json:"body" example:"Buy milk"`
	CreatedAt string `json:"created_at" example:"2026-10-01 12:00:00"`
}

// Position is the note's place in the log: the only position anywhere (docs/guides/streaming.md, rule 1).
func (n Note) Position() int64 { return n.ID }

// The resume position in every stream: a note id, as an opaque string like list's cursor.
const afterDoc = "Resume after this note id (the id of the last note you received). Absent: only notes created from now on"

type HelloOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello from charter-notes-go"`
	}
}

type ListInput struct {
	// Opaque string cursors: the generated CLI's --page-all stops on numeric ones.
	Cursor string `query:"cursor" example:"42" doc:"Opaque cursor from the previous page's next_cursor"`
	// int32, not int: Huma writes `format`, and Fern's Go SDK then types it as int, as a TypeScript contract's spec gives.
	Limit int32 `query:"limit" minimum:"1" maximum:"100" default:"20" example:"20"`
}

type ListOutput struct {
	Body struct {
		Data       []Note `json:"data"`
		NextCursor string `json:"next_cursor,omitempty" example:"41" doc:"Pass as cursor for the next page; absent on the last page"`
	}
}

type WatchInput struct {
	After   string `query:"after" pattern:"^\\d+$" example:"42" doc:"Resume after this note id (the id of the last note you received). Absent: only notes created from now on"`
	Seconds int32  `query:"seconds" minimum:"1" maximum:"300" default:"30" example:"30" doc:"How long to keep the stream open"`
	// What a browser's EventSource sends when it reconnects: the same position as After. Not in the
	// spec: generated clients use After.
	LastEventID string `header:"Last-Event-ID" hidden:"true"`
}

type LiveInput struct {
	After string `query:"after" pattern:"^\\d+$" example:"42" doc:"Resume after this note id (the id of the last note you received). Absent: only notes created from now on"`
	// The Worker's entry (worker.mjs) passes the upgrade request on; a plain GET is refused.
	Upgrade string `header:"Upgrade" hidden:"true"`
}

type CreateInput struct {
	Body struct {
		Body string `json:"body" minLength:"1" example:"Buy milk"`
	}
}

type NoteOutput struct {
	Body Note
}

// sdk is Fern's names for the SDK method: client.<group>.<method>() and `cli <group> <method>`.
func sdk(group, method string, extra map[string]any) map[string]any {
	extensions := map[string]any{"x-fern-sdk-group-name": group, "x-fern-sdk-method-name": method}
	for key, value := range extra {
		extensions[key] = value
	}
	return extensions
}

// Routes is the contract with its implementation on env.
func Routes(env Env) []humaworkers.Route {
	return []humaworkers.Route{
		{Method: http.MethodGet, Path: "/api/hello", OperationID: "hello", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "hello", Method: http.MethodGet, Path: "/api/hello",
				Summary: "Say hello", Tags: []string{"meta"},
				Extensions: sdk("meta", "hello", nil),
			}, env.hello)
		}},
		{Method: http.MethodGet, Path: "/api/notes", OperationID: "listNotes", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "listNotes", Method: http.MethodGet, Path: "/api/notes",
				Summary: "List notes, newest first (cursor pagination)", Tags: []string{"notes"},
				Extensions: sdk("notes", "list", map[string]any{
					"x-fern-pagination": map[string]any{"cursor": "$request.cursor", "next_cursor": "$response.next_cursor", "results": "$response.data"},
				}),
			}, env.list)
		}},
		{Method: http.MethodGet, Path: "/api/notes/watch", OperationID: "watchNotes", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "watchNotes", Method: http.MethodGet, Path: "/api/notes/watch",
				Summary:     "Stream notes as they are created (Server-Sent Events). The stream ends after `seconds`; call again with `after` = the last note id to continue without gaps",
				Description: "Each event's SSE id is the note id, so a browser EventSource resumes by itself (Last-Event-ID).",
				Tags:        []string{"notes"},
				// An SSE stream whose `data:` payloads are notes.
				Responses: map[string]*huma.Response{"200": {
					Description: "OK",
					Content:     map[string]*huma.MediaType{"text/event-stream": {Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[Note](), true, "")}},
				}},
				// resumable: the SDKs reconnect by themselves on a drop, sending Last-Event-ID (= the note id).
				// The terminator marks a planned end, so they only reconnect on genuine drops.
				Extensions: sdk("notes", "watch", map[string]any{
					"x-fern-streaming": map[string]any{"format": "sse", "terminator": END, "resumable": true},
				}),
			}, env.watch)
		}},
		// The WebSocket channel: in the AsyncAPI spec, not in OpenAPI. Plain JSON notes; `after` (a
		// query parameter) resumes, the same position as watch.
		{Method: http.MethodGet, Path: "/api/notes/live", OperationID: "liveNotes", Register: func(api huma.API) {
			huma.Register(api, asyncapi.Operation(huma.Operation{
				OperationID: "liveNotes", Method: http.MethodGet, Path: "/api/notes/live",
				Summary: "New notes over a WebSocket, as plain JSON. On close, reconnect with `after` = the last note id to continue without gaps",
				Errors:  []int{http.StatusUpgradeRequired},
			}, asyncapi.Channel{Name: "liveNotes", OperationID: "receiveNote", Message: "Note", Payload: Note{}}), env.live)
		}},
		{Method: http.MethodPost, Path: "/api/notes", OperationID: "createNote", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "createNote", Method: http.MethodPost, Path: "/api/notes",
				Summary: "Create a note", Tags: []string{"notes"},
				Security:   auth.Needs("write"),
				Extensions: sdk("notes", "create", nil),
			}, env.create)
		}},
	}
}
