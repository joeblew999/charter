// Package showcase is every Fern feature we use, in one small API, contract first in Go: the
// counterpart of the oRPC showcase contract (sdk/harness/src/contract.ts) and its implementation
// there. Each operation is a Huma operation whose input and output are Go structs; from this one
// definition come the handlers' validation, the OpenAPI spec and the AsyncAPI spec (spec.go), and
// from those Fern makes the SDKs. docs/showcase-go.md says, feature by feature, what switches it
// on here.
//
// The SDK's method names for the two notes operations are not here: the overlay beside the specs
// (sdk/fern/apis/showcase-go/overlays.yml) gives them, to show that feature, as the oRPC showcase
// does. A contract of your own says them itself, as api/contract.go does (x-fern-sdk-*).
package showcase

import (
	"mime/multipart"
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/api-go/asyncapi"
	"github.com/joeblew999/orpc-api/api-go/humaworkers"
)

// What the specs say about the API as a whole.
const (
	Title       = "Showcase"
	Version     = "1.0.0"
	Description = "Every Fern feature we care about, from a Huma contract (Go): standard OpenAPI + x-fern-* extensions."
	LiveTitle   = "Showcase live"
)

// OAuth is the security scheme's name: generators.yml's auth-schemes names it too.
const OAuth = "OAuth"

// The webhook's signature: x-fern-webhook-signature in the spec, and what notify sends.
const SignatureHeader = "x-webhook-signature"

// Note is the one resource.
type Note struct {
	ID   string `json:"id" example:"note-1"`
	Body string `json:"body" example:"Buy milk"`
}

// Chunk is one piece of a chat reply: the `data:` of one SSE event.
type Chunk struct {
	Text string `json:"text" example:"echo"`
	// Sent on every chunk, but not required of one: true on the last.
	Done bool `json:"done" required:"false"`
}

// NoteCreated is the webhook's payload: what the server sends to you.
type NoteCreated struct {
	Event string `json:"event" example:"note.created"`
	Note  Note   `json:"note"`
}

// Subscribe is what the client sends on the WebSocket.
type Subscribe struct {
	Topic string `json:"topic" example:"notes"`
}

// NoteEvent is what the server sends on the WebSocket.
type NoteEvent struct {
	Event string `json:"event" example:"notes.created"`
	ID    string `json:"id" example:"note-1"`
	Body  string `json:"body,omitempty" example:"Buy milk"`
	Auth  string `json:"auth,omitempty" doc:"The Authorization the server saw on this socket"`
}

// TokenRequest is the OAuth token request: a form, not JSON (RFC 6749).
type TokenRequest struct {
	ClientID     string `json:"client_id" example:"id-1"`
	ClientSecret string `json:"client_secret" example:"secret-1"`
}

type TokenInput struct {
	// humaworkers.WithForm decodes the form.
	Body TokenRequest `contentType:"application/x-www-form-urlencoded"`
}

type TokenOutput struct {
	Body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int32  `json:"expires_in" example:"3600"`
	}
}

type ListInput struct {
	Cursor string `query:"cursor" example:"2" doc:"Opaque cursor from the previous page's next_cursor"`
	Limit  int32  `query:"limit" minimum:"1" maximum:"100" default:"2" example:"2"`
}

type ListOutput struct {
	Body struct {
		Data       []Note `json:"data"`
		NextCursor string `json:"next_cursor,omitempty" example:"4" doc:"Pass as cursor for the next page; absent on the last page"`
	}
}

type CreateInput struct {
	// Not a parameter in the spec: the document's x-fern-idempotency-headers declares it once, and
	// the SDKs then offer it on every operation marked x-fern-idempotent.
	IdempotencyKey string `header:"Idempotency-Key" hidden:"true"`
	Body           struct {
		Body string `json:"body" example:"Buy milk"`
	}
}

type NoteOutput struct {
	Body Note
}

type UploadInput struct {
	// The form as net/http parsed it. Its schema is on the operation (uploadForm), written out.
	// Upstream: tinygo-org/tinygo#3862 (when fixed: huma.MultipartFormFiles[T] writes the schema and decodes the form from a struct)
	RawBody multipart.Form
}

// uploadForm is the multipart body: a file, and a text field beside it.
func uploadForm() *huma.RequestBody {
	return &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"multipart/form-data": {
		Schema: &huma.Schema{
			Type: "object",
			Properties: map[string]*huma.Schema{
				"file": {Type: "string", Format: "binary"},
				"note": {Type: "string"},
			},
			Required: []string{"file"},
		},
	}}}
}

type UploadOutput struct {
	Body struct {
		ID string `json:"id" example:"file-1"`
		// int32, not int64: Fern's Go SDK then types it int, as it does for a plain integer.
		Size int32 `json:"size" example:"1024"`
	}
}

type ChatInput struct {
	Body struct {
		Prompt string `json:"prompt" example:"Say hello"`
	}
}

type LiveInput struct {
	AccessToken string `query:"access_token" doc:"The OAuth token, for clients that can't send WebSocket headers (browsers, Workers). Others send Authorization"`
	// The WebSocket adapter passes the upgrade request on; a plain GET is refused.
	Upgrade string `header:"Upgrade" hidden:"true"`
}

// LiveOutput answers the upgrade: no feed (204), and the channel takes messages (api-go/transport).
type LiveOutput struct {
	Messages string `header:"X-Websocket-Messages"`
}

type SubscribeInput struct {
	// The upgrade's token, which the adapter sends again with every message.
	AccessToken   string `query:"access_token"`
	Authorization string `header:"Authorization" hidden:"true"`
	Body          Subscribe
}

// audiences is x-fern-audiences: a generator group with `audiences: [public]` leaves out every
// operation not tagged public.
func audiences(names ...string) map[string]any {
	return map[string]any{"x-fern-audiences": names}
}

// with adds extensions to others.
func with(extensions map[string]any, others ...map[string]any) map[string]any {
	for _, other := range others {
		for key, value := range other {
			extensions[key] = value
		}
	}
	return extensions
}

// The channel's path: one GET (the upgrade) and one POST (a message from the client).
const livePath = "/notes/live"

// Routes is the contract with its implementation on env.
func Routes(env Env) []humaworkers.Route {
	return []humaworkers.Route{
		// OAuth client credentials: generators.yml's auth-schemes points the SDKs at this operation,
		// and they call it themselves. It is the one operation that needs no token.
		{Method: http.MethodPost, Path: "/oauth/token", OperationID: "getToken", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "getToken", Method: http.MethodPost, Path: "/oauth/token",
				Summary: "OAuth client-credentials token (used by the SDK itself)", Tags: []string{"auth"},
				Security: []map[string][]string{},
				// Wrong credentials. The other operations' 401 and 422 are declared for every route
				// (humaworkers): what Huma and the token check answer with before a handler runs.
				Errors: []int{http.StatusUnauthorized},
				// The body's schema is written into the operation, not referred to by name. With a
				// named one, the test Fern's Go generator (1.64.0) writes for a form-encoded token
				// endpoint doesn't compile (it names a type `Request` that doesn't exist). Not filed
				// yet: docs/upstream.md.
				RequestBody: &huma.RequestBody{Content: map[string]*huma.MediaType{humaworkers.FormContentType: {
					Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[TokenRequest](), false, ""),
				}}},
				Extensions: audiences("public", "internal"),
			}, env.token)
		}},
		{Method: http.MethodGet, Path: "/notes", OperationID: "listNotes", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "listNotes", Method: http.MethodGet, Path: "/notes",
				Summary: "List notes (cursor pagination)", Tags: []string{"notes"},
				Extensions: with(audiences("public", "internal"), map[string]any{
					"x-fern-pagination": map[string]any{"cursor": "$request.cursor", "next_cursor": "$response.next_cursor", "results": "$response.data"},
				}),
			}, env.list)
		}},
		{Method: http.MethodPost, Path: "/notes", OperationID: "createNote", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "createNote", Method: http.MethodPost, Path: "/notes",
				Summary: "Create a note (idempotent: safe to retry with an Idempotency-Key)", Tags: []string{"notes"},
				Extensions: with(audiences("public", "internal"), map[string]any{"x-fern-idempotent": true}),
			}, env.create)
		}},
		// Multipart upload, and the one operation that is not public (audiences).
		{Method: http.MethodPost, Path: "/files", OperationID: "uploadFile", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "uploadFile", Method: http.MethodPost, Path: "/files",
				Summary: "Upload a file (multipart)", Tags: []string{"files"},
				RequestBody: uploadForm(),
				Extensions:  audiences("internal"),
			}, env.upload)
		}},
		{Method: http.MethodPost, Path: "/chat", OperationID: "chat", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "chat", Method: http.MethodPost, Path: "/chat",
				Summary: "Stream a reply (Server-Sent Events)", Tags: []string{"chat"},
				// An SSE stream whose `data:` payloads are chunks.
				Responses: map[string]*huma.Response{"200": {
					Description: "OK",
					Content:     map[string]*huma.MediaType{"text/event-stream": {Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[Chunk](), true, "")}},
				}},
				Extensions: with(audiences("public", "internal"), map[string]any{
					"x-fern-streaming": map[string]any{"format": "sse"},
				}),
			}, env.chat)
		}},
		// The WebSocket channel, both ways: in the AsyncAPI spec, not in OpenAPI. The GET is the
		// upgrade; the POST is one message from the client (its body is the message, so Huma
		// validates it), answered with the events to send back.
		{Method: http.MethodGet, Path: livePath, OperationID: "liveNotes", Register: func(api huma.API) {
			huma.Register(api, asyncapi.Operation(huma.Operation{
				OperationID: "liveNotes", Method: http.MethodGet, Path: livePath,
				Summary:       "Note events over a WebSocket: send `subscribe` with a topic, receive that topic's events",
				DefaultStatus: http.StatusNoContent,
				Errors:        []int{http.StatusUpgradeRequired},
			}, asyncapi.Channel{Name: "liveNotes", OperationID: "receiveNoteEvent", Payload: NoteEvent{}}), env.live)
		}},
		{Method: http.MethodPost, Path: livePath, OperationID: "subscribe", Register: func(api huma.API) {
			huma.Register(api, asyncapi.SendOperation(huma.Operation{
				OperationID: "subscribe", Method: http.MethodPost, Path: livePath,
				Summary: "Subscribe to a topic's events (a message on the liveNotes WebSocket)",
			}, asyncapi.Send{Channel: "liveNotes"}), env.subscribe)
		}},
	}
}
