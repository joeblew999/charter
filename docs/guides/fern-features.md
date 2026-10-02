---
title: Fern features
nav_order: 4
parent: Guides
---

# Fern features: OAuth, idempotency, uploads, webhooks and more

How to switch on what a production API needs so that the SDKs [Fern](https://buildwithfern.com) generates handle it for the caller. Every feature here works in one small API, the showcase, in Go ([Huma](https://huma.rocks)) and in TypeScript ([oRPC](https://orpc.dev)). A new project does not contain it: read it in this repo and copy the lines you need.

| | Go | TypeScript |
|---|---|---|
| The showcase | `examples/showcase-go/` | `examples/showcase-ts/` |
| The contract | `examples/showcase-go/api/contract.go` | `examples/showcase-ts/src/contract.ts` |
| The document around the operations | `config()` and `document()` in `examples/showcase-go/api/spec.go` | `document` in the contract |
| The handlers | `examples/showcase-go/api/handlers.go` | `examples/showcase-ts/src/showcase.ts` |
| Fern's settings | `examples/showcase-go/fern/generators.yml` | `examples/showcase-ts/fern/generators.yml` |

Everything is switched on through standard options: OpenAPI, `x-fern-*` extensions and `generators.yml`. Nothing is patched into the JSON. After each change: `mise run spec`, then `mise run check`.

## Feature by feature, from a Huma contract

| Feature | In the Go contract | What the caller gets |
|---|---|---|
| OAuth client credentials | The security scheme and `Security` in `config()`. On the token operation: `Security: []map[string][]string{}`, and a body tagged `contentType:"application/x-www-form-urlencoded"`, decoded by `humaworkers.WithForm`. `auth-schemes` in `generators.yml` names the token operation | `new ShowcaseClient({ clientId, clientSecret })`: the token is fetched once and sent as a bearer |
| Idempotency | `x-fern-idempotency-headers` in `config()`; `"x-fern-idempotent": true` on the operation; a `hidden:"true"` header field reads the key | `notes.create(body, { idempotencyKey })` |
| Cursor pagination | `x-fern-pagination` on the operation ([Change the contract](contract.md#paginate-a-list)) | `for await (const note of await client.notes.list())` |
| SSE streaming | `x-fern-streaming` on the operation; the 200 response declared as `text/event-stream`; the handler returns a `huma.StreamResponse` ([Streaming](streaming.md)) | `for await (const chunk of await client.chat({ prompt }))` |
| Multipart upload | `RawBody multipart.Form` in the input, and the form's schema on the operation (`uploadForm()`) | `files.uploadFile({ file, note })` |
| Webhooks | `doc.Webhooks` filled in `document()`, with the payload's schema from the struct the server sends | The `NoteCreated` type |
| Webhook signatures | `x-fern-webhook-signature` in `config()`; the handler signs the body with `crypto/hmac` | `WebhooksHelper.verifySignature(body, header, secret)` |
| A WebSocket, both ways | `asyncapi.Operation` on the upgrade, `asyncapi.SendOperation` on the operation that takes a client message ([below](#a-websocket-the-client-also-sends-on)) | TypeScript: `liveNotes.connect()`, a typed `sendSubscribe()`, `on("message")`. Go: the message types only |
| Audiences | `x-fern-audiences` on each operation; a group with `audiences: [public]` in `generators.yml` | A public SDK without the internal operations |
| Overlays | `overlays.yml` beside the specs, named in `generators.yml` | Other SDK names without touching the contract. Only for a spec you do not own: your own contract says the names itself |
| Retries | Nothing: built in | Go: `option.WithMaxAttempts` |

## What to know before you copy

- **The spec only describes: the server must enforce.** The showcase checks every request in a Huma middleware (`authorize`): an operation whose `Security` is empty passes, any other needs a valid bearer token and gets 401 otherwise.
- **Tokens need no storage:** the showcase's is an expiry signed with HMAC, which any request can check. Set real secrets: its defaults (`id-1`, `secret-1`) are for trying it out.
- **Idempotency is your handler's job.** The spec says the header exists; with a store, keep the first answer under the key.
- **The token request's schema is written into the operation,** not referred to by name: with a named one, the test Fern's Go generator writes does not compile ([Upstream issues](../upstream.md#found-not-filed)).
- **An upload is a whole value in memory,** up to 32 MB. `huma.MultipartFormFiles[T]` does not run under TinyGo.
- **A webhook is an outgoing request:** on Cloudflare, workers-go's `fetch` client (`examples/showcase-go/platform_js.go`).
- **Keep SDK methods in a group.** A paginated method on the root client broke the Rust build of the CLI (seen 2026-09-29, not filed).

## A WebSocket the client also sends on

Two operations on one path. The GET is the upgrade; the POST is one message from the client, so Huma validates it and the middleware authorizes it.

```go
huma.Register(api, asyncapi.Operation(huma.Operation{
	OperationID: "liveNotes", Method: http.MethodGet, Path: livePath,
	DefaultStatus: http.StatusNoContent,
}, asyncapi.Channel{Name: "liveNotes", OperationID: "receiveNoteEvent", Payload: NoteEvent{}}), env.live)

huma.Register(api, asyncapi.SendOperation(huma.Operation{
	OperationID: "subscribe", Method: http.MethodPost, Path: livePath,
}, asyncapi.Send{Channel: "liveNotes"}), env.subscribe)
```

- **The upgrade handler answers 204** with the header `X-Websocket-Messages: post`. Each text frame from the client then becomes a POST to the same path, and the lines of the answer go back as frames ([the protocol](../reference/packages.md#transport)).
- **A message cannot change what a feed sends through memory:** share state through a binding.
- **A client that cannot set handshake headers** (a browser, a Worker) sends the token as `?access_token=`: the showcase accepts either.

## From an oRPC contract

The same features, from `examples/showcase-ts/src/contract.ts`. What oRPC's generators (`2.0.0-beta.40`) cannot say is added in code:

| Fern needs | Where it is added |
|---|---|
| `x-fern-*`, or `security: []`, on one operation | The operation's `spec` hook in `openapi({...})` |
| A form-encoded request body | The same hook renames the media type |
| `security`, `securitySchemes`, `x-fern-idempotency-headers`, `x-fern-webhook-signature` | `document` in the contract, passed to the generator as `base` |
| OpenAPI `webhooks` | `openapiSpec({ webhooks })` in `examples/notes-ts/src/specs.ts`: one procedure per webhook |
| An SSE response whose schema is the event's data | The operation's `spec` hook |
| AsyncAPI | `examples/notes-ts/src/asyncapi.ts` ([The TypeScript version](typescript.md#the-asyncapi-generator)) |

A multipart upload needs nothing: `z.file()` in the input is enough. The oRPC showcase server checks no tokens and sends no webhook.

## How it is tested

- **`mise run check` in `examples/showcase-go/`:** the TypeScript SDK made from the Go specs, from Node, against the native build and the Wasm under workerd. 12 of 12 on 2026-10-01; the same program passed against the deployed Worker, but for the webhook delivery, which needs a receiver the Worker can reach.
- **`mise run check` in `examples/showcase-ts/`:** the compiled SDK inside a Worker under workerd. 9 checks pass, also deployed (2026-10-01).
- **`TestSameSurfaceAsTheORPCShowcase`:** Fern sees the same API from the Go and the oRPC contract.
- **Not done:** the Go SDK has not called the showcase server, and its CLI is not built.

## The TypeScript SDK inside a Worker

`examples/showcase-ts/` runs it there. It takes `guardProcessEnvAccess: true` (Workers have no `process`); the compiled SDK (the group `typescript-dist`), because the `.ts` sources clash with Cloudflare's Worker types; the WebSocket token as a query parameter; and two Workers for the WebSocket test, because a Worker cannot call its own URL (Cloudflare error 1042).
