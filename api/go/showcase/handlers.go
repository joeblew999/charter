package showcase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/api/go/humaworkers"
	"github.com/joeblew999/orpc-api/api/go/transport"
)

// Env is what the platform supplies: variables and secrets (bindings on Cloudflare, the environment
// natively) and an HTTP client for webhooks. Nothing else: the API keeps no state between requests,
// because on Cloudflare every request is a fresh Go runtime (workers-go).
type Env struct {
	// Var reads a setting; empty means the default below.
	Var func(name string) string
	// HTTP sends the webhooks. Nil: none are sent.
	HTTP *http.Client
}

// The settings and their defaults. The client and the webhook secret default to what the oRPC
// showcase in sdk/harness uses, so the same test runs against either; a real deployment sets them
// all as secrets.
var defaults = map[string]string{
	"CLIENT_ID":      "id-1",
	"CLIENT_SECRET":  "secret-1",
	"TOKEN_SECRET":   "showcase-token-secret", // signs access tokens
	"WEBHOOK_SECRET": "whsec",                 // signs webhooks
	"WEBHOOK_URL":    "",                      // where noteCreated is sent; empty: nowhere
}

func (env Env) setting(name string) string {
	if env.Var != nil {
		if value := env.Var(name); value != "" {
			return value
		}
	}
	return defaults[name]
}

// Handler serves the contract on env, plus the two specs with the request's origin as their server.
func Handler(env Env) http.Handler {
	routes := humaworkers.New(config(), Routes(env))
	routes.UseMiddleware(env.authorize(routes))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var spec func(server string) ([]byte, error)
		switch r.URL.Path {
		case "/openapi.json":
			spec = OpenAPI
		case "/asyncapi.json":
			spec = AsyncAPI
		default:
			routes.ServeHTTP(w, r)
			return
		}
		body, err := spec(origin(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
}

// origin is the request's scheme and host: absolute in r.URL on Workers, from Host on net/http.
func origin(r *http.Request) string {
	if r.URL.Scheme != "" && r.URL.Host != "" {
		return r.URL.Scheme + "://" + r.URL.Host
	}
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

// ---- OAuth client credentials ----

// tokenLife is how long an access token is good for.
const tokenLife = time.Hour

// sign is HMAC-SHA256 in hex: what signs both the access tokens and the webhooks.
func sign(secret string, message []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}

// newToken makes an access token that needs no storage: its expiry, signed. Any request can check
// it, which is what a server with no memory between requests needs.
func (env Env) newToken(now time.Time) string {
	expiry := strconv.FormatInt(now.Add(tokenLife).Unix(), 10)
	return expiry + "." + sign(env.setting("TOKEN_SECRET"), []byte("access-token."+expiry))
}

func (env Env) validToken(token string, now time.Time) bool {
	expiry, signature, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(sign(env.setting("TOKEN_SECRET"), []byte("access-token."+expiry)))) {
		return false
	}
	until, err := strconv.ParseInt(expiry, 10, 64)
	return err == nil && now.Unix() < until
}

func (env Env) token(_ context.Context, in *TokenInput) (*TokenOutput, error) {
	id := hmac.Equal([]byte(in.Body.ClientID), []byte(env.setting("CLIENT_ID")))
	secret := hmac.Equal([]byte(in.Body.ClientSecret), []byte(env.setting("CLIENT_SECRET")))
	if !id || !secret {
		return nil, huma.Error401Unauthorized("bad client")
	}
	out := &TokenOutput{}
	out.Body.AccessToken = env.newToken(time.Now())
	out.Body.ExpiresIn = int32(tokenLife / time.Second)
	return out, nil
}

// authorize is the contract's security, enforced: an operation needs a valid access token unless
// its Security says otherwise (getToken's is empty). The token comes as `Authorization: Bearer`; on
// the WebSocket also as ?access_token=, for clients that can't set headers on a handshake.
func (env Env) authorize(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		op := ctx.Operation()
		if op.Security != nil && len(op.Security) == 0 {
			next(ctx)
			return
		}
		token := bearer(ctx.Header("Authorization"))
		if token == "" && op.Path == livePath {
			token = ctx.Query("access_token")
		}
		if !env.validToken(token, time.Now()) {
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "a valid access token is required: get one from POST /oauth/token")
			return
		}
		next(ctx)
	}
}

func bearer(authorization string) string {
	scheme, token, ok := strings.Cut(authorization, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// ---- notes: cursor pagination, idempotency, the webhook ----

// The notes are fixed: there is no store, so every request sees the same five.
var notes = []Note{{"n1", "note n1"}, {"n2", "note n2"}, {"n3", "note n3"}, {"n4", "note n4"}, {"n5", "note n5"}}

func (env Env) list(_ context.Context, in *ListInput) (*ListOutput, error) {
	// Cursors are opaque strings to callers (here, how many notes came before).
	start := 0
	if in.Cursor != "" {
		n, err := strconv.Atoi(in.Cursor)
		if err != nil || n < 0 || n > len(notes) {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "query.cursor", Message: "not a cursor from next_cursor", Value: in.Cursor})
		}
		start = n
	}
	next := min(start+int(in.Limit), len(notes))
	out := &ListOutput{}
	out.Body.Data = notes[start:next]
	if next < len(notes) {
		out.Body.NextCursor = strconv.Itoa(next)
	}
	return out, nil
}

// create is idempotent without a store: the note's id is the Idempotency-Key, so a retry with the
// same key gets the same answer. With a store, keep the first answer under the key instead.
func (env Env) create(ctx context.Context, in *CreateInput) (*NoteOutput, error) {
	note := Note{ID: in.IdempotencyKey, Body: in.Body.Body}
	if note.ID == "" {
		note.ID = "new"
	}
	if err := env.notify(ctx, NoteCreated{Event: "note.created", Note: note}); err != nil {
		log.Printf("create: note %s: the webhook was not delivered: %v", note.ID, err)
	}
	return &NoteOutput{Body: note}, nil
}

// notify sends the noteCreated webhook to WEBHOOK_URL, signed: the signature header carries the
// HMAC-SHA256 of the exact body, in hex, under WEBHOOK_SECRET. The receiver checks it with the
// SDK's WebhooksHelper.verifySignature.
func (env Env) notify(ctx context.Context, event NoteCreated) error {
	url := env.setting("WEBHOOK_URL")
	if url == "" || env.HTTP == nil {
		return nil
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, sign(env.setting("WEBHOOK_SECRET"), body))
	res, err := env.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("%s answered HTTP %d", url, res.StatusCode)
	}
	return nil
}

// ---- files: multipart ----

func (env Env) upload(_ context.Context, in *UploadInput) (*UploadOutput, error) {
	files := in.RawBody.File["file"]
	if len(files) == 0 {
		return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body.file", Message: "expected required file to be present"})
	}
	note := ""
	if values := in.RawBody.Value["note"]; len(values) > 0 {
		note = values[0]
	}
	out := &UploadOutput{}
	out.Body.ID = files[0].Filename + ":" + note
	out.Body.Size = int32(files[0].Size)
	return out, nil
}

// ---- chat: SSE ----

// chat streams the reply as SSE: one `data:` event per word, each a Chunk, the last with done.
func (env Env) chat(_ context.Context, in *ChatInput) (*huma.StreamResponse, error) {
	words := strings.Split("echo "+in.Body.Prompt, " ")
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "text/event-stream")
		w := flushing(hc.BodyWriter())
		for i, word := range words {
			data, err := json.Marshal(Chunk{Text: word, Done: i == len(words)-1})
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			select {
			case <-hc.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}}, nil
}

// ---- the WebSocket, both ways ----

// live answers the upgrade. This channel has no feed of its own: everything the server sends is the
// answer to a message, so the socket needs no Go runtime between messages.
func (env Env) live(_ context.Context, in *LiveInput) (*LiveOutput, error) {
	if !strings.EqualFold(in.Upgrade, "websocket") {
		return nil, huma.NewError(http.StatusUpgradeRequired, "expected a WebSocket upgrade")
	}
	return &LiveOutput{Messages: transport.MessagesPost}, nil
}

// subscribe is one `subscribe` message from the client: it answers with the topic's events, one
// JSON event per line, and the adapter sends each line as a frame.
func (env Env) subscribe(_ context.Context, in *SubscribeInput) (*huma.StreamResponse, error) {
	auth := in.Authorization
	if auth == "" && in.AccessToken != "" {
		auth = "Bearer " + in.AccessToken
	}
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "application/x-ndjson")
		w := flushing(hc.BodyWriter())
		for _, note := range notes[:3] {
			data, err := json.Marshal(NoteEvent{Event: in.Body.Topic + ".created", ID: note.ID, Body: note.Body, Auth: auth})
			if err != nil {
				return
			}
			if _, err := w.Write(append(data, '\n')); err != nil {
				return
			}
		}
	}}, nil
}

// flushing sends every write on at once (net/http buffers; workers-go's writer does not).
func flushing(w io.Writer) io.Writer {
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
