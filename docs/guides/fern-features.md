---
title: Auth, idempotency, uploads, webhooks
nav_order: 3
parent: Guides
---

# Auth, idempotency, uploads, webhooks

This page gets you the things a production API needs, written so that the SDKs Fern generates handle them for the caller: OAuth tokens fetched and sent by themselves, idempotency keys, file uploads, signed webhooks, a WebSocket the client also sends on, a public SDK that leaves out internal operations, and renamed SDK methods. Each section says what the caller gets, the lines to add to your contract, and how to check.

## The working example

Every feature here exists, working, in a small API in the orpc-api repo: `api/go/showcase/` in [github.com/joeblew999/orpc-api](https://github.com/joeblew999/orpc-api). It is deployed at https://orpc-showcase-go.gedw99.workers.dev. Your project from `dev new` does not contain it: read it in that repo, copy the lines you need.

Where things go in your project:

| What | File in your project | In the showcase |
|---|---|---|
| Operations and their structs | `api/go/api/contract.go` | `api/go/showcase/contract.go` |
| The document around the operations (security scheme, `x-fern-*` at the top level) | `config()` in `api/go/api/spec.go` | `config()` in `api/go/showcase/spec.go` |
| Handlers | `api/go/api/handlers.go` | `api/go/showcase/handlers.go` |
| SDK settings | `sdk/fern/apis/api-go/generators.yml` | `sdk/fern/apis/showcase-go/generators.yml` |

After each change: `mise run api:go:spec`, then `mise run check`. To see the SDK, `mise run sdk:gen api-go typescript` (Docker).

The showcase's own test generates the TypeScript SDK from its specs and calls the running server with it, from Node. In the orpc-api repo:

```sh
mise run showcase:go:check   # lint, Go tests, spec check, then the SDK test against the native build and under workerd
```

and against the deployed Worker, `node test/showcase-test.mjs "$SHOWCASE_GO_URL" showcase-go`. What has run where is in that repo's [findings](../findings.md). The Go tests of the showcase (`go test ./showcase/` in `api/go/`) passed when this page was written, and I ran the server natively and called it with `curl` for the outputs below. I did not run Fern for this page, so what an SDK looks like below is from the repo's own docs and test, not from my run.

## OAuth client credentials

**The caller gets** an SDK that takes `clientId` and `clientSecret`, fetches a token once, sends it as a bearer on every call, and fetches another when it expires:

```ts
const client = new ShowcaseClient({ clientId, clientSecret });
```

**Add to the contract.** The security scheme and its requirement, on the document (`config()`):

```go
config := humaworkers.WithForm(humaworkers.Config(Title, Version))
config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{OAuth: {
	Type:  "oauth2",
	Flows: &huma.OAuthFlows{ClientCredentials: &huma.OAuthFlow{TokenURL: "/oauth/token", Scopes: map[string]string{}}},
}}
requirement := map[string][]string{OAuth: {}}
config.Security = []map[string][]string{requirement}
```

`humaworkers.WithForm` is there because the token request is a form (`application/x-www-form-urlencoded`, RFC 6749) and Huma has no decoder for it. It makes a body tagged with that content type decode like a JSON one. Every form value is a string, so the body's fields must be strings.

The token operation, whose body is a form and which needs no token itself (`Security` empty):

```go
type TokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type TokenInput struct {
	Body TokenRequest `contentType:"application/x-www-form-urlencoded"`
}
```

```go
huma.Register(api, huma.Operation{
	OperationID: "getToken", Method: http.MethodPost, Path: "/oauth/token",
	Summary: "OAuth client-credentials token (used by the SDK itself)", Tags: []string{"auth"},
	Security: []map[string][]string{},
	RequestBody: &huma.RequestBody{Content: map[string]*huma.MediaType{humaworkers.FormContentType: {
		Schema: api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[TokenRequest](), false, ""),
	}}},
}, env.token)
```

The body's schema is written into the operation, not referred to by name, because with a named one the test that Fern's Go generator writes does not compile (the showcase's comment names it).

Tell Fern which operation is the token endpoint, in `sdk/fern/apis/api-go/generators.yml`:

```yaml
auth-schemes:
  OAuth:
    scheme: oauth
    type: client-credentials
    client-id-env: BILLING_CLIENT_ID
    client-secret-env: BILLING_CLIENT_SECRET
    get-token:
      endpoint: "POST /oauth/token"
      request-properties:
        client-id: $request.client_id
        client-secret: $request.client_secret
      response-properties:
        access-token: $response.access_token
        expires-in: $response.expires_in

api:
  auth: OAuth
```

(The `*-env` names are the environment variables the SDK reads when no credentials are passed; the showcase uses `SHOWCASE_CLIENT_ID` and `SHOWCASE_CLIENT_SECRET`. The `api:` block sits above the `specs` list that is already in the file.)

**The server must enforce it.** The scheme in the spec only describes. The showcase checks every request in a Huma middleware (`authorize` in `api/go/showcase/handlers.go`, registered with `routes.UseMiddleware(env.authorize(routes))`): an operation whose `Security` is empty passes, any other needs `Authorization: Bearer <token>` and gets 401 with `WWW-Authenticate: Bearer` otherwise. Its token is an expiry signed with HMAC, so any request can check it with no storage, which a Worker needs (every request is a fresh Go runtime). Your project's notes API has no auth: copy `authorize`, `newToken` and `validToken` and set your own secrets.

**Check it.** Run against the showcase natively (`mise run showcase:go:run` in the orpc-api repo, port 5175; the output is from another port):

```sh
curl -si localhost:5175/notes
# HTTP/1.1 401 Unauthorized
# Www-Authenticate: Bearer
# {"title":"Unauthorized","status":401,"detail":"a valid access token is required: get one from POST /oauth/token"}

curl -s -X POST localhost:5175/oauth/token -d 'client_id=id-1&client_secret=secret-1'
# {"access_token":"1790854911.9a1e5c6e...","expires_in":3600}

curl -s -X POST localhost:5175/oauth/token -d 'client_id=x&client_secret=y'
# {"title":"Unauthorized","status":401,"detail":"bad client"}
```

The showcase's defaults (`id-1`, `secret-1`) are for trying it out: a real deployment sets `CLIENT_ID`, `CLIENT_SECRET`, `TOKEN_SECRET` and `WEBHOOK_SECRET` as secrets (`mise run cloudflare:secrets` is the task your project has for secrets).

## Idempotency

**The caller gets** an option on the operations you mark: `client.notes.create(body, { idempotencyKey })`. The SDK sends it as the `Idempotency-Key` header, and a retry with the same key must give the same answer.

**Add to the contract.** The header, declared once on the document:

```go
idempotency := map[string]string{"header": "Idempotency-Key", "name": "idempotency_key"}
config.Extensions = map[string]any{
	"x-fern-idempotency-headers": []map[string]string{idempotency},
}
```

On each operation, the marker, and a hidden field in the input that reads the header (hidden so it is not a parameter in the spec; Huma still reads it):

```go
Extensions: map[string]any{"x-fern-idempotent": true},
```

```go
type CreateInput struct {
	IdempotencyKey string `header:"Idempotency-Key" hidden:"true"`
	Body           struct {
		Body string `json:"body"`
	}
}
```

The spec only says the header exists. Making the operation idempotent is your handler's job: with a store, keep the first answer under the key and return it again. The showcase has no store, so its note's id is the key: the same key gives the same note.

**Check it.**

```sh
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Idempotency-Key: k-1' -H 'content-type: application/json' -d '{"body":"hi"}' localhost:5175/notes
# {"id":"k-1","body":"hi"}
```

## Multipart file upload

**The caller gets** `files.uploadFile({ file, note })` with a file and a text field.

**Add to the contract.** Take the form as `multipart.Form` and write its schema on the operation:

```go
type UploadInput struct {
	RawBody multipart.Form
}
```

```go
RequestBody: &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"multipart/form-data": {
	Schema: &huma.Schema{
		Type: "object",
		Properties: map[string]*huma.Schema{
			"file": {Type: "string", Format: "binary"},
			"note": {Type: "string"},
		},
		Required: []string{"file"},
	},
}}},
```

The handler reads `in.RawBody.File["file"]` and `in.RawBody.Value["note"]`.

**The limits, from the showcase's measurements under local workerd.** Huma's typed multipart (`huma.MultipartFormFiles`) panics under TinyGo, so use `RawBody` as above (`tinygo-org/tinygo#3862`). A Worker has no disk, so `humaworkers` keeps uploads in memory with a 32 MB limit; a 5 MB upload took 2.4 s there. A Worker's own request limits apply on top. The file is a whole value in memory, not a stream.

**Check it.**

```sh
curl -s -X POST -H "Authorization: Bearer $TOKEN" -F file=@f.txt -F note=x localhost:5175/files
# {"id":"f.txt:x","size":5}
```

## Webhooks with a signature

**The caller gets** a typed payload (`NoteCreated`) and a helper that checks the signature: `WebhooksHelper.verifySignature(body, header, secret)`.

**Add to the contract.** A webhook is something your server sends, so no route describes it. Huma's document has a `Webhooks` field that nothing fills, so fill it where the spec is made (`document()` in `api/go/showcase/spec.go`):

```go
doc := api.OpenAPI()
doc.Webhooks = map[string]*huma.PathItem{"noteCreated": {Post: &huma.Operation{
	OperationID: "noteCreatedWebhook",
	Summary:     "Sent to you when a note is created",
	Tags:        []string{"notes"},
	RequestBody: &huma.RequestBody{Content: map[string]*huma.MediaType{
		"application/json": {Schema: doc.Components.Schemas.Schema(reflect.TypeFor[NoteCreated](), true, "")},
	}},
	Responses: map[string]*huma.Response{"200": {Description: "received"}},
}}}
```

How the signature is made, on the document (`config()`):

```go
config.Extensions["x-fern-webhook-signature"] = map[string]string{"type": "hmac", "header": SignatureHeader, "algorithm": "sha256", "encoding": "hex"}
```

(`SignatureHeader` is `x-webhook-signature`.) Then your handler must send it that way: `notify` in `api/go/showcase/handlers.go` posts the JSON body with the header set to the HMAC-SHA256 of the exact bytes, in hex, under `WEBHOOK_SECRET`. `crypto/hmac` and `crypto/sha256` work under TinyGo. The send is an outgoing request from an `*http.Client`; on Cloudflare that is workers-go's `fetch` client.

**Check it.** The SDK test starts a receiver and checks that the helper accepts the server's delivery and rejects a changed body or another secret; it only runs where the receiver can be reached, so on a deployed Worker the delivery has not been checked. Natively: `mise run showcase:go:test:native` in the orpc-api repo.

## A WebSocket the client also sends on

**The caller gets** `liveNotes.connect()`, a typed `sendSubscribe(...)` and `on("message")`. The feed-only socket of [Real-time](streaming.md) is `asyncapi.Operation`; this adds the client's side with `asyncapi.SendOperation`.

**Add to the contract.** Two operations on one path: the GET is the upgrade and the POST is one message from the client. The POST's body is the message, so Huma validates it:

```go
huma.Register(api, asyncapi.Operation(huma.Operation{
	OperationID: "liveNotes", Method: http.MethodGet, Path: livePath,
	Summary:       "Note events over a WebSocket: send `subscribe` with a topic, receive that topic's events",
	DefaultStatus: http.StatusNoContent,
}, asyncapi.Channel{Name: "liveNotes", OperationID: "receiveNoteEvent", Payload: NoteEvent{}}), env.live)

huma.Register(api, asyncapi.SendOperation(huma.Operation{
	OperationID: "subscribe", Method: http.MethodPost, Path: livePath,
	Summary: "Subscribe to a topic's events (a message on the liveNotes WebSocket)",
}, asyncapi.Send{Channel: "liveNotes"}), env.subscribe)
```

The upgrade handler answers 204 with the header `X-Websocket-Messages: post` (`transport.MessagesPost`), which tells the Worker's adapter that the channel takes messages. Each text frame from the client then becomes a POST to the same path, with the upgrade's headers, and the lines of the answer go back as frames. Anything but 2xx to a message closes the socket with 1008.

Two limits. On Cloudflare every call to Go may be in a Go runtime of its own, so a message cannot change what a feed in another call sends; share state through a binding (a database, a Durable Object). Messages are handled one at a time, in order. Details: the repo's [showcase page](../showcase-go.md#the-websocket-both-ways).

**Check it.** The two operations are in the AsyncAPI spec, and neither is in OpenAPI (a run of the showcase):

```sh
curl -s localhost:5175/asyncapi.json | python3 -c "import sys,json; print({k:v['action'] for k,v in json.load(sys.stdin)['operations'].items()})"
# {'receiveNoteEvent': 'receive', 'sendSubscribe': 'send'}
```

I did not open the socket for this page; the showcase's SDK test does (it sends `subscribe` and gets three events back).

## Audiences: a public SDK without the internal operations

**The caller gets** an SDK that has only what you tagged public; the internal operations are not in it.

**Add to the contract.** Tag each operation with who may see it:

```go
Extensions: map[string]any{"x-fern-audiences": []string{"public", "internal"}},   // in the SDK for either audience
```

An operation with only `[]string{"internal"}` (the showcase's upload) is left out of a group that asks for `public`. In `sdk/fern/apis/api-go/generators.yml`, a group with an `audiences` list:

```yaml
groups:
  typescript-public:
    audiences:
      - public
    generators:
      - name: fernapi/fern-typescript-sdk
        # version, output and config as in your typescript group
```

Operations with no `x-fern-audiences` are, as far as I read the showcase, in no audience-filtered group; tag everything you want in the public SDK. I did not run Fern to confirm what an untagged one does.

**Check it.** Generate the group (`mise run sdk:gen api-go typescript-public`) and look for the operation in `sdk/out/api-go/typescript-public`. The showcase's test checks that its public SDK has `notes` and `auth` and no `files`.

## Overlays: rename without touching the contract

**The caller gets** the SDK names you choose. When the contract is yours, say them in the contract (`x-fern-sdk-group-name` and `x-fern-sdk-method-name`, [Define your API](contract.md#name-the-sdk-methods)). An overlay is for a spec you do not own, or a rename you want to keep out of the Go code. It is a file named `overlays.yml` beside the specs, in `sdk/fern/apis/api-go/`:

```yaml
overlay: 1.0.0
info:
  title: SDK naming
  version: 1.0.0
actions:
  - target: $.paths['/api/notes'].get
    update:
      x-fern-sdk-group-name: notes
      x-fern-sdk-method-name: list
```

and `generators.yml` says which spec it applies to:

```yaml
api:
  specs:
    - openapi: openapi.json
      overlays: overlays.yml
    - asyncapi: asyncapi.json
```

The showcase's overlay is `sdk/fern/apis/showcase-go/overlays.yml` in the orpc-api repo. Never edit `openapi.json` by hand: `mise run api:go:spec` writes it again from the contract.

## Where the full table is

The orpc-api repo's [showcase page](../showcase-go.md) has every feature with how the contract switches it on, what the SDK gets and how it is tested; its SDK page has the Fern feature table.
