package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/lib/auth"
	"github.com/joeblew999/charter/go/lib/follow"
	"github.com/joeblew999/charter/go/lib/transport"
)

// Env is what the platform supplies: variables and secrets (bindings on Cloudflare, the environment
// natively), the notes' log and hub, and an HTTP client for webhooks. The API keeps nothing in
// memory between requests, because on Cloudflare every request may be a Go runtime of its own
// (workers-go): the notes are in the store (D1), and the hub (a Durable Object) wakes the feeds.
type Env struct {
	// Var reads a setting; empty means the default below.
	Var func(name string) string
	// Store and Hub are opened per request: a binding belongs to the request's environment.
	Store func() (Store, error)
	Hub   func() (Hub, error)
	// HTTP sends the webhooks. Nil: none are sent.
	HTTP *http.Client
}

// The settings and their defaults. The client and the webhook secret default to what the oRPC
// conformance in ts/conformance uses, so the same test runs against either; a real deployment sets them
// all as secrets.
var defaults = map[string]string{
	"CLIENT_ID":      "id-1",
	"CLIENT_SECRET":  "secret-1",
	"TOKEN_SECRET":   "conformance-token-secret", // signs access tokens
	"WEBHOOK_SECRET": "whsec",                    // signs webhooks
	"WEBHOOK_URL":    "",                         // where noteCreated is sent; empty: nowhere
}

func (env Env) setting(name string) string {
	if env.Var != nil {
		if value := env.Var(name); value != "" {
			return value
		}
	}
	return defaults[name]
}

// trusted is who may do what, by every way in: the API's tokens (READ_TOKEN may only read), Cloudflare
// Access (a person writes, a machine's service token reads), an OpenID Connect issuer (its token's
// scope claim), and the OAuth tokens /api/oauth/token issued, which read and write. An unset secret or
// setting trusts no one that way.
func (env Env) trusted() []auth.Trusted {
	return []auth.Trusted{
		auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
		auth.Token{Secret: "READ_TOKEN", Scopes: []string{"read"}},
		auth.Access{People: []string{"read", "write"}, Machines: []string{"read"}},
		auth.OIDC{},
		auth.Issued{Scheme: OAuth, TokenPath: "/api/oauth/token", Check: func(token string) ([]string, bool) {
			return []string{"read", "write"}, env.validToken(token, time.Now())
		}},
	}
}

// Handler serves the contract on env the standard way (go/lib/serve): each call allowed by any of the
// four ways in (auth) and limited as its operation says, per caller (ratelimit), the specs at
// /api/openapi.json and /api/asyncapi.json, the operations as MCP tools at /api/mcp (a tool call runs
// the same operation as the REST route, with the credentials the MCP request carries), and how to get
// in at /.well-known/.
func Handler(env Env) http.Handler {
	return API(env).Handler()
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

// token is the client-credentials grant (RFC 6749, 4.4): the client's credentials from the form or
// from a Basic header, checked in constant time.
func (env Env) token(_ context.Context, in *TokenInput) (*TokenOutput, error) {
	if in.Body.GrantType != "" && in.Body.GrantType != "client_credentials" {
		return nil, huma.Error400BadRequest("unsupported_grant_type: the one grant is client_credentials")
	}
	clientID, clientSecret := in.Body.ClientID, in.Body.ClientSecret
	if basicID, basicSecret, ok := basicCredentials(in.Authorization); ok {
		clientID, clientSecret = basicID, basicSecret
	}
	id := hmac.Equal([]byte(clientID), []byte(env.setting("CLIENT_ID")))
	secret := hmac.Equal([]byte(clientSecret), []byte(env.setting("CLIENT_SECRET")))
	if !id || !secret {
		return nil, huma.Error401Unauthorized("bad client")
	}
	out := &TokenOutput{}
	out.Body.AccessToken, out.Body.TokenType = env.newToken(time.Now()), "Bearer"
	out.Body.ExpiresIn = int32(tokenLife / time.Second)
	return out, nil
}

// basicCredentials are a client's id and secret from an Authorization: Basic header, each form-encoded
// before the base64 (RFC 6749, 2.3.1).
func basicCredentials(authorization string) (string, string, bool) {
	scheme, encoded, ok := strings.Cut(authorization, " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", "", false
	}
	id, secret, ok := strings.Cut(string(raw), ":")
	if !ok {
		return "", "", false
	}
	if unescaped, err := url.QueryUnescape(id); err == nil {
		id = unescaped
	}
	if unescaped, err := url.QueryUnescape(secret); err == nil {
		secret = unescaped
	}
	return id, secret, true
}

// ---- notes: cursor pagination, idempotency, the webhook, the hub ----

func (env Env) list(ctx context.Context, in *ListInput) (*ListOutput, error) {
	// Cursors are opaque strings to callers (here, how many notes came before).
	start := 0
	if in.Cursor != "" {
		n, err := strconv.Atoi(in.Cursor)
		if err != nil || n < 0 {
			return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "query.cursor", Message: "not a cursor from next_cursor", Value: in.Cursor})
		}
		start = n
	}
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	// One more than the page: whether there is a next one.
	entries, err := store.Page(ctx, start, int(in.Limit)+1)
	if err != nil {
		return nil, err
	}
	out := &ListOutput{}
	out.Body.Data = []Note{}
	for _, entry := range entries[:min(len(entries), int(in.Limit))] {
		out.Body.Data = append(out.Body.Data, entry.Note())
	}
	if len(entries) > int(in.Limit) {
		out.Body.NextCursor = strconv.Itoa(start + int(in.Limit))
	}
	return out, nil
}

// create is idempotent by its key: the note's id is the Idempotency-Key, and the store keeps the
// first note under it, so a retry gets the first answer and announces nothing. A new note goes to
// the hub, which wakes every feed, and to the webhook.
func (env Env) create(ctx context.Context, in *CreateInput) (*NoteOutput, error) {
	if strings.Contains(in.Body.Body, END) {
		return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body.body", Message: "a note may not contain " + END, Value: in.Body.Body})
	}
	note := Note{ID: in.IdempotencyKey, Body: in.Body.Body}
	if note.ID == "" {
		// rand.Text would do, but TinyGo's crypto/rand lacks it.
		random := make([]byte, 8)
		rand.Read(random)
		note.ID = "note-" + hex.EncodeToString(random)
	}
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	entry, created, err := store.Put(ctx, note)
	if err != nil {
		return nil, err
	}
	if !created {
		return &NoteOutput{Body: entry.Note()}, nil
	}
	// The hub is only a wake-up: if it misses this note, followers find it in the log (Recheck).
	if hub, err := env.Hub(); err != nil {
		log.Printf("create: no hub, note %s is not announced: %v", entry.ID, err)
	} else if err := hub.Publish(ctx, entry); err != nil {
		log.Printf("create: note %s is not announced: %v", entry.ID, err)
	}
	if err := env.notify(ctx, NoteCreated{Event: "note.created", Note: entry.Note()}); err != nil {
		log.Printf("create: note %s: the webhook was not delivered: %v", entry.ID, err)
	}
	return &NoteOutput{Body: entry.Note()}, nil
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

// chat streams the reply as SSE, framed as the oRPC conformance frames it: a comment first, one
// `message` event per word, each a Chunk, the last with done, then a `close` event.
func (env Env) chat(_ context.Context, in *ChatInput) (*huma.StreamResponse, error) {
	words := strings.Split("echo "+in.Body.Prompt, " ")
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "text/event-stream")
		w := transport.Flushing(hc.BodyWriter())
		if _, err := io.WriteString(w, ": \n\n"); err != nil {
			return
		}
		for i, word := range words {
			data, err := json.Marshal(Chunk{Text: word, Done: i == len(words)-1})
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", data); err != nil {
				return
			}
			select {
			case <-hc.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
		io.WriteString(w, "event: close\n\n")
	}}, nil
}

// ---- the WebSocket, both ways ----

// topic is the one topic the channel has.
const topic = "notes"

// feed is the notes' log behind follow.Follow: the store is the log, the hub only wakes followers.
type feed struct {
	store Store
	hub   Hub
}

func (f feed) Subscribe(listener func(Entry), onError func(error)) (func(), error) {
	return f.hub.Subscribe(listener, onError)
}
func (f feed) Since(ctx context.Context, after int64, limit int) ([]Entry, error) {
	return f.store.Since(ctx, after, limit)
}
func (f feed) Latest(ctx context.Context) (int64, error) { return f.store.Latest(ctx) }

// log opens the notes' log and its hub, for follow.Follow.
func (env Env) feed() (feed, error) {
	store, err := env.Store()
	if err != nil {
		return feed{}, err
	}
	hub, err := env.Hub()
	if err != nil {
		return feed{}, err
	}
	return feed{store, hub}, nil
}

// watch is SSE over Follow: every event's id is its place in the log, so `after` and Last-Event-ID
// are the same position (a reconnecting SDK resends its original `after` plus a newer Last-Event-ID:
// the newer wins). A planned end (`seconds`) sends END, the terminator; if Follow gives up (hub
// down) the stream just ends without it, so the SDKs reconnect by themselves.
func (env Env) watch(_ context.Context, in *WatchInput) (*huma.StreamResponse, error) {
	notes, err := env.feed()
	if err != nil {
		return nil, err
	}
	options := follow.Options{After: follow.Newest(in.After, in.LastEventID), OnBroken: func(err error) { log.Printf("watch: hub subscription broken, resubscribing: %v", err) }}
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "text/event-stream")
		w := transport.Flushing(hc.BodyWriter())
		// A comment first: the response starts now, not at the first note.
		if _, err := io.WriteString(w, ": \n\n"); err != nil {
			return
		}
		until, cancel := context.WithTimeout(hc.Context(), time.Duration(in.Seconds)*time.Second)
		defer cancel()
		err := follow.Follow(until, notes, options, func(entry Entry) error {
			data, err := json.Marshal(entry.Note())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(w, "event: message\nretry: 1000\nid: %d\ndata: %s\n\n", entry.Seq, data)
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

// live answers the upgrade with the feed, the way the notes example does (docs/self/streaming.md):
// every note from the log's start, then each new one as the hub announces it, one JSON event per
// line, which the adapter sends as a frame. The stream ends only when Follow gives up (the hub is
// down); the adapter then closes the socket with 1011. The channel takes messages too (subscribe).
func (env Env) live(_ context.Context, in *LiveInput) (*huma.StreamResponse, error) {
	if !strings.EqualFold(in.Upgrade, "websocket") {
		return nil, huma.NewError(http.StatusUpgradeRequired, "expected a WebSocket upgrade")
	}
	notes, err := env.feed()
	if err != nil {
		return nil, err
	}
	auth := in.Authorization
	if auth == "" && in.AccessToken != "" {
		auth = "Bearer " + in.AccessToken
	}
	return &huma.StreamResponse{Body: func(hc huma.Context) {
		hc.SetHeader("Content-Type", "application/x-ndjson")
		hc.SetHeader(transport.MessagesHeader, transport.MessagesPost)
		w := transport.Flushing(hc.BodyWriter())
		// An empty line first: the response (and so the 101) goes out now, not at the first note.
		if _, err := io.WriteString(w, "\n"); err != nil {
			return
		}
		start := int64(0)
		options := follow.Options{After: &start, OnBroken: func(err error) { log.Printf("live: hub subscription broken, resubscribing: %v", err) }}
		err := follow.Follow(hc.Context(), notes, options, func(entry Entry) error {
			data, err := json.Marshal(NoteEvent{Event: topic + ".created", ID: entry.ID, Body: entry.Body, Auth: auth})
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

// subscribe is one `subscribe` message from the client. On Cloudflare it runs in a Go runtime of its
// own, apart from the feed (transport), so it can only answer: the one topic is taken (204), any
// other is refused (422), and the adapter then closes the socket with 1008.
func (env Env) subscribe(_ context.Context, in *SubscribeInput) (*struct{}, error) {
	if in.Body.Topic != topic {
		return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body.topic", Message: "the one topic is " + topic, Value: in.Body.Topic})
	}
	return nil, nil
}
