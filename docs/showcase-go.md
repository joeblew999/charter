---
title: Go showcase internals
nav_order: 4
parent: This repository
---
# The Go showcase: every Fern feature from a Huma contract

The showcase is one small API with every Fern feature we use. It exists twice, like the notes API: as an oRPC contract (the oRPC showcase, in [sdk.md](sdk.md#the-orpc-showcase)) and, on this page, as a Go one. Read this page when a Go API needs OAuth, idempotency, a file upload, webhooks or a WebSocket the client also sends on: it says, feature by feature, what to write in the contract.

- **The contract is Huma operations** (`api/go/showcase/contract.go`). Both specs are generated from it into `sdk/fern/apis/showcase-go/`.
- **The server is real,** not a mock: it checks tokens, signs and sends the webhook, and validates what a WebSocket client sends.
- **It runs three ways:** natively, as TinyGo Wasm on workers-go under local workerd, and deployed as the Worker `orpc-showcase-go`.
- **A test keeps it equal to the oRPC showcase** in everything an SDK sees.

## Tasks (from the repo root)

```sh
mise run showcase:go:run            # natively on :5175 (SHOWCASE_GO_PORT). WEBHOOK_URL=<url> to get the webhook
mise run showcase:go:dev            # under workerd on :5175: the TinyGo build, then cf dev. SHOWCASE_WEBHOOK_URL=<url> to get the webhook
mise run showcase:go:build          # the Wasm into api/go/cmd/showcase/build (fails over 3 MB gzipped)
mise run showcase:go:spec           # write both specs again, after changing the contract
mise run showcase:go:check          # LOCAL: lint, Go tests, spec:check, test:native and test:workerd
mise run showcase:go:lint           # go vet of the server for Wasm (api:go:lint covers gofmt and the host)
mise run showcase:go:spec:check     # fail if a committed spec is stale against the contract
mise run showcase:go:test:native    # the SDK test against the native build, with the webhook
mise run showcase:go:test:workerd   # the same against the TinyGo Wasm under workerd
mise run showcase:go:test           # the SDK test against a running showcase:go:run or showcase:go:dev (no webhook check)
mise run sdk:check-spec showcase-go # fern check
mise run sdk:gen showcase-go go     # Fern (Docker); the other groups: typescript, typescript-public, typescript-dist, cli
mise run showcase:go:deploy         # REMOTE: build and deploy orpc-showcase-go. It has no storage
```

The test tasks generate the TypeScript SDK they use the first time, which needs Docker. There is no task for the test against the deployed Worker: `node test/showcase-test.mjs "$SHOWCASE_GO_URL" showcase-go` (the variable is set by mise; it defaults to this repo's deployed Worker).

## Layout

It lives inside the `api/go` module, beside the notes API, so it imports `humaworkers`, `asyncapi`, `transport` and `specfile` instead of copying them. It is its own program and its own Worker, so the notes API's Wasm, specs and behaviour don't depend on it.

| Path | What it is |
|---|---|
| `api/go/showcase/contract.go` | **The contract (edit this):** every operation, its input and output structs, and what Fern needs |
| `api/go/showcase/handlers.go` | The contract implemented, the token check, and the settings' defaults |
| `api/go/showcase/spec.go` | Both specs from the contract: the document-level settings and the webhook are written here |
| `api/go/showcase/showcase_test.go`, `api/go/showcase/surface_test.go` | The handlers, and the same surface as the oRPC showcase |
| `api/go/cmd/showcase/` | The server: `main.go` and two `platform_*.go` files; `worker.mjs`, `cloudflare.config.ts`, `package.json` and `vite.config.ts` make it a Worker (`orpc-showcase-go`). cf and its packages are `api/go/`'s |
| `api/go/cmd/showcase-spec/` | Writes the two spec files, or checks them (`specfile`, as `api/go/cmd/spec/` does for the notes API) |
| `go/transport/` | The WebSocket adapter for the native build, and the rules both adapters follow |
| `go/worker/websocket.mjs` | The same adapter for Cloudflare, used by both Workers' entries |
| `sdk/fern/apis/showcase-go/` | The generated specs, `generators.yml` (the same groups as `sdk/fern/apis/showcase-ts/`) and the overlay |
| `test/showcase-test.mjs` | The SDK test: the TypeScript SDK against a running server ([testing.md](testing.md)) |

## The routes

Paths start at the root. Every route but the token one needs a valid bearer token.

| Route | Operation | What it does |
|---|---|---|
| `POST /oauth/token` | `getToken` | OAuth client credentials, as a form. The token is good for one hour |
| `GET /notes` | `listNotes` | Five fixed notes, two per page (`limit` 1 to 100, default 2) |
| `POST /notes` | `createNote` | Answers a note whose id is the `Idempotency-Key`, and sends the webhook. Nothing is stored |
| `POST /files` | `uploadFile` | A multipart upload: `file` and an optional `note` field |
| `POST /chat` | `chat` | An SSE stream: `echo` and the prompt, one word per event |
| `GET /notes/live` | `liveNotes` | The WebSocket. The client sends `{"topic": "..."}` and gets three events back |
| `GET /openapi.json`, `GET /asyncapi.json` | | Both specs, with the request's origin as their server |

## Feature by feature

"The SDK test" is `test/showcase-test.mjs`: the TypeScript SDK that Fern generates from the Go specs, from Node, over the network. `mise run showcase:go:check` runs it against the native build and against the Wasm under workerd. "Go tests" are the two test files in `api/go/showcase/`. In the table, `config()` and `document()` are in `api/go/showcase/spec.go`.

| Fern feature | How the Go contract switches it on | What the SDK gets | How it is tested |
|---|---|---|---|
| OAuth client credentials | The security scheme and `security` in `config()`; `Security: []map[string][]string{}` on the token operation; its body tagged `contentType:"application/x-www-form-urlencoded"` and decoded by `humaworkers.WithForm`; `auth-schemes` in `generators.yml` names the token operation | `new ShowcaseClient({ clientId, clientSecret })`: the token is fetched once, as a form, and sent as a bearer | The SDK test: one token call, form-encoded, and every other request carries the bearer; a call with no token or a forged one is 401. Go tests: every operation against five kinds of bad token |
| Idempotency | `x-fern-idempotency-headers` in `config()`; `"x-fern-idempotent": true` in the operation's `Extensions`; the handler reads the header through a `hidden:"true"` field | `notes.create(body, { idempotencyKey })` | The SDK test: the header arrives, and the same key twice gives the same note |
| Cursor pagination | `x-fern-pagination` in `Extensions`; `cursor` in the input struct, `next_cursor` in the output | `for await (const note of await client.notes.list())` | The SDK test: five notes over three requests |
| SSE streaming | `x-fern-streaming: { format: sse }` in `Extensions`; the 200 response declared as `text/event-stream` with `Chunk`'s schema; the handler returns a `huma.StreamResponse` | `for await (const chunk of await client.chat({ prompt }))` | The SDK test: typed chunks, the last with `done` |
| Multipart file upload | `RawBody multipart.Form`, with the form's schema on the operation (`uploadForm()`) | `files.uploadFile({ file, note })` | The SDK test: a 20 KB file and a form field. Go tests: 100 KB, and a missing file is 422 |
| Webhooks | `doc.Webhooks` in `document()`, with the payload's schema from the `NoteCreated` struct the server sends | The `NoteCreated` type | The SDK test: the server posts it to the test's receiver when a note is created (local runs only) |
| Webhook signatures | `x-fern-webhook-signature` in `config()`; `notify` signs the body with `crypto/hmac` and `crypto/sha256` | `WebhooksHelper.verifySignature(body, header, secret)` | The SDK test: the helper accepts the server's delivery, and rejects it with another secret or a changed body |
| WebSocket, both ways | `asyncapi.Operation(...)` on the upgrade (the channel and what the server sends); `asyncapi.SendOperation(...)` on the operation that takes a client message (its body is the message) | TypeScript: `liveNotes.connect()`, a typed `sendSubscribe()`, `on("message")`. Go: the message types | The SDK test: the SDK's client sends `subscribe` and gets the topic's three events; the same with the token as `?access_token=`; no token is refused; a message the contract rejects closes the socket with 1008 |
| Audiences | `x-fern-audiences` in `Extensions`; the group `typescript-public` in `generators.yml` has `audiences: [public]` | A public SDK without the internal `files.uploadFile` | The SDK test: the generated public SDK has `notes` and `auth`, and no `files` |
| Overlays | `overlays.yml` beside the specs, the same file as the oRPC showcase's; the contract leaves those two names out | `notes.list` and `notes.create` instead of `notes.listNotes` and `notes.createNote` | The SDK test calls the methods by those names. Go tests hold the two overlay files equal |

The overlay is there to show the feature. A contract of your own says the names itself, as `api/go/api/contract.go` does (`x-fern-sdk-group-name`, `x-fern-sdk-method-name`).

## What Huma has no struct tag for, and where it is written

All of it is Go code. Nothing is patched into the JSON.

| Fern needs | Huma 2.39.1 | Where it is written |
|---|---|---|
| `security`, `components.securitySchemes`, `x-fern-idempotency-headers`, `x-fern-webhook-signature` | Fields of the document (`huma.Config`), no tags | `config()` |
| `security: []` on one operation; `x-fern-*` on an operation | `Operation.Security`, `Operation.Extensions` | The operation in `api/go/showcase/contract.go` |
| A form-encoded request body | The `contentType` tag writes the media type into the spec, but Huma has no format that decodes a form | `humaworkers.WithForm(config)`. Every form value is a string, so the body's fields must be strings |
| A header the SDK sends by itself (`Idempotency-Key`) | A `header` field is a parameter in the spec | `hidden:"true"` on the field: Huma still reads it |
| A multipart body | `huma.MultipartFormFiles[T]` writes the schema and decodes the form from a struct, but TinyGo can't run it ([below](#tinygo-what-it-took)) | `RawBody multipart.Form`, and the schema on the operation |
| OpenAPI 3.1 `webhooks` | The field exists (`OpenAPI.Webhooks`); nothing fills it | `document()` |
| An SSE response whose schema is the event's data | No tag | `Responses` on the operation, as the notes API's `watch` |
| AsyncAPI, with messages both ways | Not in Huma | `go/asyncapi/`: `Operation` and `SendOperation` |

One more, for Fern rather than Huma: the token request's schema is written into the operation instead of referred to by name (`Schema(type, false, "")`). With a named schema, the test Fern's Go generator writes for a form-encoded token endpoint does not compile ([upstream.md](upstream.md#found-not-filed)).

## The WebSocket, both ways

workers-go can't answer a WebSocket upgrade, so Go answers it with plain HTTP and an adapter carries that over the socket: `go/worker/websocket.mjs` on Cloudflare, `transport.Serve` natively. For a feed like the notes one the adapter only sends. The showcase's client also sends, and for that the adapter has one more rule:

- **Each text frame from the client becomes a `POST` to the same URL,** with the upgrade request's headers and the frame as the JSON body. The lines of the answer are sent back as frames.
- **Go asks for it** with the header `X-Websocket-Messages: post` on its answer to the upgrade. Without the header, what the client sends is ignored: the notes channel.
- **A 204 answer to the upgrade means there is no feed.** The socket stays open until the client closes it. (A 200 with a body is a feed.)
- **Anything but 2xx to a message closes the socket with 1008.** A binary frame closes it with 1003.

Why this way: a message is an ordinary Huma operation. Its body is the message, so Huma validates it against the contract, the same middleware authorizes it (the upgrade's headers and query come with every message), and `asyncapi.SendOperation` writes it into the AsyncAPI spec. There is nothing to configure in JavaScript: which paths are sockets and what they take stays in Go.

The limit: on Cloudflare every call to Go may be in a Go runtime of its own. The feed and each message can't count on shared memory, so a message can't change what the feed sends. What they must share goes through a binding (a Durable Object, a database), as the notes hub does. In the showcase the answer to `subscribe` is sent in the message's own reply, so the socket needs no Go runtime between messages. Messages are handled one at a time, in order.

## TinyGo: what it took

Measured on 2026-10-01 under local workerd (`cf dev`); the upstream issues are in [upstream.md](upstream.md).

- **`crypto/hmac` and `crypto/sha256` work.** Tokens are signed and checked, and the webhook is signed, in the Wasm.
- **`huma.MultipartFormFiles[T]` panics:** `unimplemented: (reflect.Value).MethodByName()`. The upload takes the plain `multipart.Form` and declares its schema on the operation.
- **Uploads over 8 KB failed** (`open /tmp/multipart-...: file does not exist`): Huma's adapter keeps 8 KB in memory and writes the rest to a temporary file, and a Worker has no disk. `humaworkers` raises the limit to 32 MB, so uploads stay in memory. 5 MB took 2.4 s.
- **A webhook is an outgoing request:** workers-go's `fetch` client, as an `*http.Client`, in `api/go/cmd/showcase/platform_js.go`.
- **The Wasm is 2.50 MB, 884 KB gzipped** (the notes Worker: 2.47 MB, 875 KB).

## Differences from the oRPC showcase

Fern sees the same API. `TestSameSurfaceAsTheORPCShowcase` compares the two generated specs: each operation's id, tags, summary, `x-fern-*` extensions, security, parameters, request body and 200 response by media type with their shapes; the webhook; the document-level settings; the channel, its operations and its messages. It names two differences, and fails if one of them goes away:

- **`limit` has bounds and a default** (1 to 100, 2) in the Go contract. The oRPC contract leaves it open.
- **The channel's `access_token` query parameter is declared,** so `connect({ access_token })` is typed. The oRPC server reads it undeclared.

Left out of the comparison, because no SDK sees them: descriptions, `additionalProperties: false`, integer formats and safe-integer bounds, and schema names where one side has none. Huma names every body (`CreateInputBody`), so the SDK's request types have those names. The Go spec also declares Huma's error model by status (422 and 401 on every operation that can answer them, from `humaworkers`), so the Go SDK gets a typed error ([api-go.md](api-go.md#how-it-fits-together)).

The servers behave the same where the SDK test looks, and differ here:

- **Tokens are checked.** Every operation but the token one needs a valid bearer token (401 otherwise). The oRPC showcase answers anyone, so the SDK test runs against it with `--open`.
- **The token is an expiry signed with HMAC,** so any request can check it without storage. The oRPC showcase gives the fixed `tok-1`.
- **The webhook is sent** when a note is created and `WEBHOOK_URL` is set. The oRPC showcase sends none.
- **Errors are Huma's** `application/problem+json`: 422 with the location for invalid input.
- **Paths start at the root** (`/notes`), not under `/api/mock`.

## Settings

Environment variables natively, variables and secrets on Cloudflare. The defaults are for trying it out, and they are what the SDK test uses.

| Name | Default | What |
|---|---|---|
| `CLIENT_ID`, `CLIENT_SECRET` | `id-1`, `secret-1` | The one OAuth client |
| `TOKEN_SECRET` | `showcase-token-secret` | Signs the access tokens |
| `WEBHOOK_URL` | empty | Where `noteCreated` is sent. Empty: nowhere. For `cf dev` and a deploy it is read from `SHOWCASE_WEBHOOK_URL` |
| `WEBHOOK_SECRET` | `whsec` | Signs the webhook |

`api/go/cmd/showcase/cloudflare.config.ts` binds only `WEBHOOK_URL`. The other four keep their defaults unless they are set as secrets on the Worker, so a deployment that guards anything must set them.

## What has run where

- **Locally (2026-10-01):** the SDK test passes 12/12 against the native build and against the Wasm under workerd.
- **On Cloudflare (2026-10-01):** deployed to https://orpc-showcase-go.gedw99.workers.dev, the SDK test passes every check it runs there. The one check not run is the server posting a webhook to the test's own receiver, which only exists on the machine running the test.

Both are in [findings.md](findings.md). Not done:

- **The webhook delivery from Cloudflare** has not been received anywhere.
- **The Go SDK has not called the server.** It builds, vets and passes Fern's own tests (`mise run sdk:check`); only the TypeScript SDK ran against the server.
- **The Fern CLI is not built** for this API. The `cli` group is in `generators.yml`.
- **The SDK inside a Worker** (the harness Worker's `/api/sdk-test`) has not run against the Go server: the SDK test runs it from Node.
