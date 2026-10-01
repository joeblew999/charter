---
title: Go showcase (all Fern features)
nav_order: 5
---

# The showcase in Go: every Fern feature from a Huma contract

The showcase is one small API with every Fern feature we use. It exists twice, like the notes API: as an oRPC contract (`sdk/harness/src/contract.ts`, in [sdk.md](sdk.md#the-showcase-is-contract-first-orpc-verified-2026-10-01)) and, on this page, as a Go one. Read this page when a Go API needs OAuth, idempotency, a file upload, webhooks or a WebSocket the client also sends on: it says, feature by feature, what to write in the contract.

- **The contract is Huma operations** (`api-go/showcase/contract.go`). Both specs are generated from it into `sdk/fern/apis/showcase-go/`.
- **The server is real,** not a mock: it checks tokens, signs and sends the webhook, and validates what a WebSocket client sends.
- **It runs twice:** natively, and as TinyGo Wasm on workers-go under local workerd. It is not deployed to Cloudflare.
- **A test keeps it equal to the oRPC showcase** in everything an SDK sees.

## Layout

It lives inside the `api-go` module, beside the notes API, so it imports `humaworkers`, `asyncapi`, `transport` and `specfile` instead of copying them. It is its own program and its own Worker, so the notes API's Wasm, specs and behaviour don't change.

| Path | What it is |
|---|---|
| `api-go/showcase/contract.go` | **The API (edit this):** every operation, its input and output structs, and what Fern needs |
| `api-go/showcase/handlers.go` | The contract implemented, and the token check |
| `api-go/showcase/spec.go` | Both specs from the contract: the document-level settings and the webhook are written here |
| `api-go/showcase/*_test.go` | The handlers, and the same surface as the oRPC showcase |
| `api-go/cmd/showcase/` | The server: `main.go` and two `platform_*.go` files; `worker.mjs`, `cloudflare.config.ts`, `package.json` and `vite.config.ts` make it a Worker (`orpc-showcase-go`). cf and its packages are `api-go`'s |
| `api-go/cmd/showcase-spec/` | Writes the two spec files, or checks them (`specfile`, as `cmd/spec` does for the notes API) |
| `api-go/transport/` | **Import it.** The WebSocket adapter for the native build, and the rules both adapters follow |
| `api-go/worker/websocket.mjs` | The same adapter for Cloudflare, used by both Workers' entries |
| `sdk/fern/apis/showcase-go/` | The generated specs, `generators.yml` (the same groups as `showcase`) and the overlay |
| `test/showcase-test.mjs` | The test: the TypeScript SDK against a running server |

## Tasks (from the repo root)

```sh
mise run showcase-go:run            # natively on :5175
mise run showcase-go:dev            # under workerd on :5175 (TinyGo build, then cf dev)
mise run showcase-go:spec           # regenerate the specs after changing the contract
mise run showcase-go:check          # lint, Go tests, spec drift, TinyGo build, the SDK test natively and under workerd
mise run showcase-go:test           # the SDK test against a running showcase-go:run or showcase-go:dev
mise run sdk:check-spec showcase-go # fern check
mise run sdk:gen showcase-go go     # Fern: also typescript, typescript-public, typescript-dist, cli
mise run showcase-go:deploy         # REMOTE: deploy orpc-showcase-go (not done yet)
```

## Feature by feature

"The test" is `test/showcase-test.mjs`: the TypeScript SDK that Fern generates from the Go specs, from Node, over the network. `mise run showcase-go:check` runs it against the native build and against the Wasm under workerd. "Go tests" are `api-go/showcase/*_test.go`.

| Fern feature | How the Go contract switches it on | What the SDK gets | How it is tested |
|---|---|---|---|
| OAuth client credentials | The security scheme and `security` in `config()` (`spec.go`); `Security: []map[string][]string{}` on the token operation; its body tagged `contentType:"application/x-www-form-urlencoded"` and decoded by `humaworkers.WithForm`; `auth-schemes` in `generators.yml` names the token operation | `new ShowcaseClient({ clientId, clientSecret })`: the token is fetched once, as a form, and sent as a bearer | The test: one token call, form-encoded, and every other request carries the bearer; a call with no token or a forged one is 401. Go tests: every operation against five kinds of bad token |
| Idempotency | `x-fern-idempotency-headers` in `config()`; `"x-fern-idempotent": true` in the operation's `Extensions`; the handler reads the header through a `hidden:"true"` field | `notes.create(body, { idempotencyKey })` | The test: the header arrives, and the same key twice gives the same note |
| Cursor pagination | `x-fern-pagination` in `Extensions`; `cursor` in the input struct, `next_cursor` in the output | `for await (const note of await client.notes.list())` | The test: five notes over three requests |
| SSE streaming | `x-fern-streaming: { format: sse }` in `Extensions`; the 200 response declared as `text/event-stream` with `Chunk`'s schema; the handler returns a `huma.StreamResponse` | `for await (const chunk of await client.chat({ prompt }))` | The test: typed chunks, the last with `done` |
| Multipart file upload | `RawBody multipart.Form`, with the form's schema on the operation (`uploadForm()`) | `files.uploadFile({ file, note })` | The test: a 20 KB file and a form field. Go tests: 100 KB, a missing file is 422 |
| Webhooks | `doc.Webhooks` in `spec.go`, with the payload's schema from the `NoteCreated` struct the server sends | The `NoteCreated` type | The test: the server posts it to the test's receiver when a note is created |
| Webhook signatures | `x-fern-webhook-signature` in `config()`; `notify` signs the body with `crypto/hmac` and `crypto/sha256` | `WebhooksHelper.verifySignature(body, header, secret)` | The test: the helper accepts the server's delivery, and rejects it with another secret or a changed body |
| WebSocket, both ways | `asyncapi.Operation(...)` on the upgrade (the channel and what the server sends); `asyncapi.SendOperation(...)` on the operation that takes a client message (its body is the message) | TypeScript: `liveNotes.connect()`, a typed `sendSubscribe()`, `on("message")`. Go: the message types | The test: the SDK's client sends `subscribe` and gets the topic's three events; the same with the token as `?access_token=`; no token is refused; a message the contract rejects closes the socket with 1008 |
| Audiences | `x-fern-audiences` in `Extensions`; the group `typescript-public` in `generators.yml` has `audiences: [public]` | A public SDK without the internal `files.uploadFile` | The test: the generated public SDK has `notes` and `auth`, and no `files` |
| Overlays | `overlays.yml` beside the specs, the same file as the oRPC showcase's; the contract leaves those two names out | `notes.list` and `notes.create` instead of `notes.listNotes` and `notes.createNote` | The test calls the methods by those names. Go tests hold the two overlay files equal |

The overlay is there to show the feature. A contract of your own says the names itself, as `api-go/api/contract.go` does (`x-fern-sdk-group-name`, `x-fern-sdk-method-name`).

## What Huma has no struct tag for, and where it is written

All of it is Go code. Nothing is patched into the JSON.

| Fern needs | Huma 2.39.1 | Where it is written |
|---|---|---|
| `security`, `components.securitySchemes`, `x-fern-idempotency-headers`, `x-fern-webhook-signature` | Fields of the document (`huma.Config`), no tags | `config()` in `spec.go` |
| `security: []` on one operation; `x-fern-*` on an operation | `Operation.Security`, `Operation.Extensions` | The operation in `contract.go` |
| A form-encoded request body | The `contentType` tag writes the media type into the spec, but Huma has no format that decodes a form | `humaworkers.WithForm(config)`. Every form value is a string, so the body's fields must be strings |
| A header the SDK sends by itself (`Idempotency-Key`) | A `header` field is a parameter in the spec | `hidden:"true"` on the field: Huma still reads it |
| A multipart body | `huma.MultipartFormFiles[T]` writes the schema and decodes the form from a struct, but TinyGo can't run it (below) | `RawBody multipart.Form`, and the schema on the operation |
| OpenAPI 3.1 `webhooks` | The field exists (`OpenAPI.Webhooks`); nothing fills it | `document()` in `spec.go` |
| An SSE response whose schema is the event's data | No tag | `Responses` on the operation, as the notes API's `watch` |
| AsyncAPI, with messages both ways | Not in Huma | `api-go/asyncapi`: `Operation` and the new `SendOperation` |

One more, for Fern rather than Huma: the token request's schema is written into the operation instead of referred to by name (`Schema(type, false, "")`). With a named schema, the test Fern's Go generator writes for a form-encoded token endpoint does not compile ([upstream.md](upstream.md#found-not-filed-yet)).

## The WebSocket, both ways

workers-go can't answer a WebSocket upgrade, so Go answers it with plain HTTP and an adapter carries that over the socket: `api-go/worker/websocket.mjs` on Cloudflare, `transport.Serve` natively. For a feed like the notes one the adapter only sends. The showcase's client also sends, and for that the adapter has one more rule:

- **Each text frame from the client becomes a `POST` to the same URL,** with the upgrade request's headers and the frame as the JSON body. The lines of the answer are sent back as frames.
- **Go asks for it** with the header `X-Websocket-Messages: post` on its answer to the upgrade. Without the header, what the client sends is ignored: the notes channel.
- **A 204 answer to the upgrade means there is no feed.** The socket stays open until the client closes it. (A 200 with a body is a feed.)
- **Anything but 2xx to a message closes the socket with 1008.** A binary frame closes it with 1003.

Why this way: a message is an ordinary Huma operation. Its body is the message, so Huma validates it against the contract, the same middleware authorizes it (the upgrade's headers and query come with every message), and `asyncapi.SendOperation` writes it into the AsyncAPI spec. There is nothing to configure in JavaScript: which paths are sockets and what they take stays in Go.

The limit: on Cloudflare every call to Go is a fresh Go runtime. The feed and each message can't share memory, so a message can't change what the feed sends. What they must share goes through a binding (a Durable Object, a database), as the notes hub does. In the showcase the answer to `subscribe` is sent in the message's own reply, so the socket needs no Go runtime between messages. Messages are handled one at a time, in order.

## TinyGo: what it took

Measured on 2026-10-01 under local workerd (`cf dev`); the upstream issues are in [upstream.md](upstream.md).

- **`crypto/hmac` and `crypto/sha256` work.** Tokens are signed and checked, and the webhook is signed, in the Wasm.
- **`huma.MultipartFormFiles[T]` panics:** `unimplemented: (reflect.Value).MethodByName()`. The upload takes the plain `multipart.Form` and declares its schema on the operation.
- **Uploads over 8 KB failed** (`open /tmp/multipart-...: file does not exist`): Huma's adapter keeps 8 KB in memory and writes the rest to a temporary file, and a Worker has no disk. `humaworkers` raises the limit to 32 MB, so uploads stay in memory. 5 MB took 2.4 s.
- **A webhook is an outgoing request:** workers-go's `fetch` client, as an `*http.Client`, in `cmd/showcase/platform_js.go`.
- **The Wasm is 2.50 MB, 884 KB gzipped** (the notes Worker: 2.47 MB, 875 KB).

## Differences from the oRPC showcase

Fern sees the same API. `TestSameSurfaceAsTheORPCShowcase` compares the two generated specs: each operation's id, tags, summary, `x-fern-*` extensions, security, parameters, request body and 200 response by media type with their shapes; the webhook; the document-level settings; the channel, its operations and its messages. It names two differences, and fails if one of them goes away:

- **`limit` has bounds and a default** (1 to 100, 2) in the Go contract. The oRPC contract leaves it open.
- **The channel's `access_token` query parameter is declared,** so `connect({ access_token })` is typed. The oRPC server reads it undeclared.

Left out of the comparison, because no SDK sees them: descriptions, `additionalProperties: false`, integer formats and safe-integer bounds, and schema names where one side has none. Huma names every body (`CreateInputBody`), so the SDK's request types have those names. Huma also declares its error model on every operation, so the SDKs get a typed error.

The servers behave the same where the test looks, and differ here:

- **Tokens are checked.** Every operation but the token one needs a valid bearer token (401 otherwise). The oRPC showcase answers anyone.
- **The token is an expiry signed with HMAC,** so any request can check it without storage. The oRPC showcase gives the fixed `tok-1`.
- **The webhook is sent** when a note is created and `WEBHOOK_URL` is set. The oRPC showcase sends none.
- **Errors are Huma's** `application/problem+json`: 422 with the location for invalid input.
- **Paths start at the root** (`/notes`), not under `/api/mock`.

## Settings

Variables natively, variables and secrets on Cloudflare. The defaults are for trying it out.

| Name | Default | What |
|---|---|---|
| `CLIENT_ID`, `CLIENT_SECRET` | `id-1`, `secret-1` | The one OAuth client |
| `TOKEN_SECRET` | a fixed text | Signs the access tokens |
| `WEBHOOK_URL` | empty | Where `noteCreated` is sent. Empty: nowhere. Under `cf dev` it is read from `SHOWCASE_WEBHOOK_URL` |
| `WEBHOOK_SECRET` | `whsec` | Signs the webhook |

## Not done

- **Not deployed.** Nothing here has run on Cloudflare, where Go has behaved differently before (timers: [upstream.md](upstream.md)). The spec's server is the URL the Worker would have.
- **The Go SDK has not called the server.** It builds, vets and passes Fern's own tests (`sdk:check`); only the TypeScript SDK ran against the server.
- **The Rust CLI is not built** for this API. The `cli` group is in `generators.yml`.
- **The SDK inside a Worker** (the harness's `/api/sdk-test`) has not run against the Go server: the test runs it from Node.
