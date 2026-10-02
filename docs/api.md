---
title: oRPC Worker internals (api/ts/)
nav_order: 3
parent: This repository
---
# api/ts/: the notes API in TypeScript, on Cloudflare

The oRPC Worker: the notes API as an oRPC 2.0 (`2.0.0-beta.40`) Worker on D1, contract first. From the one contract, oRPC serves plain REST, an SSE stream and a WebSocket, and both specs are generated that Fern turns into SDKs, a CLI and docs ([sdk.md](sdk.md)). Read this page to run or change the oRPC Worker, to see what the notes API is (the Go Worker in [api-go.md](api-go.md) serves the same one), or to copy the pattern into a TypeScript project.

## Tasks (from the repo root)

```sh
mise run api:ts:dev               # the API under cf dev on http://localhost:5173 (PORT)
mise run api:ts:migrate:local     # first time, in another shell while api:ts:dev runs: the D1 schema
mise run api:ts:spec              # write both specs again, after changing the contract
mise run api:ts:check             # LOCAL: api:ts:typecheck, api:ts:test and api:ts:spec:check
mise run api:ts:typecheck         # cf's Worker types, then tsc
mise run api:ts:test              # the unit tests of follow() (vitest, in Node)
mise run api:ts:spec:check        # fail if a committed spec is stale against the contract
mise run api:ts:deploy            # REMOTE: deploy orpc-api, then apply pending D1 migrations
mise run api:ts:migrate           # REMOTE: apply pending migrations to orpc-api-db
mise run api:ts:live-test         # REMOTE, writes test notes: SSE and WebSocket, raw and through the TypeScript SDK
mise run api:ts:soak              # REMOTE, redeploys orpc-api: the real-time matrix; --idle 20 for the long-idle case
mise run api:ts:bench             # REMOTE, read-only: wall time per route (benchmarks.md)
```

Deploying needs a Cloudflare login (`api/ts/node_modules/.bin/cf auth login`), or `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`. The remote tasks use `API_URL`, which defaults to this repo's deployed Worker; on another account set it in `mise.local.toml` (gitignored). `api:ts:live-test` and `api:ts:soak` generate the SDKs they use first, which needs Docker ([testing.md](testing.md)).

## The notes API

Both Workers serve these routes. Fern sees the same API from either ([api-go.md](api-go.md#differences-from-the-orpc-worker) lists where the servers differ).

| Route | Contract name | What it does |
|---|---|---|
| `GET /api/hello` | `hello` | A greeting with the Worker's `APP_NAME` |
| `GET /api/notes` | `notes.list` | Notes, newest first. `limit` 1 to 100 (default 20); `cursor` is the previous page's `next_cursor`, an opaque string |
| `POST /api/notes` | `notes.create` | Creates a note from `{"body": "..."}`. The body is at least one character and must not contain `[end-of-stream]` |
| `GET /api/notes/watch` | `notes.watch` | New notes as Server-Sent Events. `after` is the last note id received; `seconds` 1 to 300 (default 30) |
| `GET /api/notes/live` | `notes.live` | New notes over a WebSocket, as plain JSON. `after` as above |
| `GET /api/openapi.json`, `GET /api/asyncapi.json` | | Both specs, generated on request with the request's origin as their server |

A note is `{ id, body, created_at }`. How the two streams stay gap-free, and what a client does when one ends, is in [realtime.md](realtime.md).

## Layout

| Path | What it is |
|---|---|
| `api/ts/src/contract.ts` | **The contract (edit this):** every route, its input and output (Zod 4), and what Fern needs (`openapi({...})`, `asyncapi({...})`) |
| `api/ts/src/index.ts` | The Worker: the contract implemented on D1, the SSE stream, the WebSocket transport, and the two spec routes |
| `api/ts/src/follow.ts` | `follow()`: the one gap-free feed both transports use ([realtime.md](realtime.md)) |
| `api/ts/src/hub.ts` | The hub (`NotesHub`): oRPC's `DurablePublisherObject`, a hibernating live fan-out |
| `api/ts/src/asyncapi.ts` | The AsyncAPI generator (oRPC has none), [below](#the-asyncapi-generator) |
| `api/ts/src/specs.ts` | Both specs from any contract: OpenAPI 3.1.1 without the channels, OpenAPI `webhooks`, AsyncAPI 3.0.0. It knows no contract, so the oRPC showcase (`sdk/harness/`) uses it too |
| `api/ts/spec.ts` | Writes the two spec files offline, into `sdk/fern/apis/api-ts/` |
| `api/ts/spec-files.ts` | The command behind `api/ts/spec.ts` (write, or `--check`), shared with `sdk/harness/spec.ts` |
| `api/ts/test/follow.test.ts` | The unit tests of `follow()` |
| `api/ts/cloudflare.config.ts` | The Worker `orpc-api`: D1 (`DB`), the hub (`HUB`), `APP_NAME`, static assets (`ASSETS`) |
| `api/ts/public/index.html` | A static page served for every path outside `/api/`. It reads `GET /api/notes` as a plain array, which the API doesn't answer, and no test covers it |
| `migrations/` | The D1 schema, shared with the Go Worker: each Worker has its own database |

## How it works

- **The contract is the source.** Each route's `openapi({ operationId, tags, spec })` metadata names the SDK method and adds Fern's `x-fern-*` extensions, so nothing is patched into the JSON afterwards. After changing it, run `mise run api:ts:spec`; `mise run api:ts:check` fails if a committed spec is stale.
- **oRPC serves REST from the router** (`OpenAPIHandler`). The WebSocket is the one route the Worker handles itself: it calls the contract's `notes.live` procedure with the query as input and sends each note it yields as a JSON frame. The Worker holds the socket.
- **New notes go through the hub.** `create` writes to D1, then publishes to `NotesHub` with oRPC's `DurablePublisher`. The hub only wakes followers; D1 is the log.
- **OpenAPI is 3.1.1,** asked for in `api/ts/src/specs.ts`: oRPC 2.0 defaults to 3.2.0, which Fern rejects ([upstream.md](upstream.md)).
- **Cursors are strings in the contract.** With a numeric `next_cursor`, the Fern CLI's `--page-all` stopped after page one ([findings.md](findings.md)).
- **The SSE response's schema is the note,** set by the route's `spec` hook. oRPC describes its own event envelope there (`event: message`, `close`, `error`), and Fern's clients ignore the SSE `event:` field ([upstream.md](upstream.md)).

## The AsyncAPI generator

`api/ts/src/asyncapi.ts` writes AsyncAPI 3.0.0 from an oRPC contract the way `@orpc/openapi` writes OpenAPI. It is built only on oRPC's public APIs (a metadata plugin, `walkProcedureContractsAsync`, the JSON Schema converters), so it needs no fork.

```ts
live: oc
  .meta(openapi({ method: "GET", path: "/api/notes/live" }))
  .meta(asyncapi({ channel: "liveNotes", address: "/api/notes/live", operationId: "receiveNote", message: "Note", summary: "..." }))
  .input(z.object({ after }))            // the channel's query parameters
  .output(asyncIteratorObject(note)),    // what the server sends
```

- **A procedure with `asyncapi({ channel, address })` is a WebSocket channel.** It stays out of `openapi.json` (the OpenAPI generator's `filter`) and appears only in AsyncAPI.
- **Its output, `asyncIteratorObject(x)`, is a `receive` operation** whose message payload is `x`.
- **An object input is the channel's query parameters** (`bindings.ws.query`). Fern's TypeScript SDK then has `liveNotes.connect({ after })`.
- **An `asyncIteratorObject(y)` input is what the client sends:** a `send` operation whose message payload is `y`, named by `send: { message }` in the metadata. The oRPC showcase uses it ([sdk.md](sdk.md#the-orpc-showcase)).
- **`spec`** in the metadata has the last word on the channel object, for extensions.

Limits: one message type each way; a channel can't have query parameters and a send side together; a procedure with more than one `.input()` or `.output()` is refused. What is planned for it (a package, an offer to oRPC) is in [plans/asyncapi.md](plans/asyncapi.md).

## Copying it into a TypeScript (oRPC) project

1. Write the contract with `openapi({...})` and `asyncapi({...})` metadata, as in `api/ts/src/contract.ts`. For OAuth, idempotency, file upload, webhooks or a WebSocket the client also sends on, see the oRPC showcase's contract (`sdk/harness/src/contract.ts`, [sdk.md](sdk.md#the-orpc-showcase)).
2. Copy `api/ts/src/follow.ts`, `api/ts/src/asyncapi.ts`, `api/ts/src/specs.ts` and `api/ts/spec-files.ts` unchanged, and give `follow()` your own source (`subscribe`, `since`, `latest`).
3. Copy `sdk/fern/apis/api-ts/` as your API's Fern folder, plus the `api:ts:*` and `sdk:*` tasks from `mise.toml`.
4. Have clients follow the client rule in [realtime.md](realtime.md#what-a-client-does).
5. Get the GitHub workflows from the `dev` tool ([dev.md](dev.md#the-workflows-in-another-repo)).

## Limits

- **It runs only on Cloudflare** (`cf dev` locally). There is no native build, unlike the Go Worker.
- **No MCP endpoint.** Only the Go Worker has one ([mcp.md](mcp.md)).
- **Invalid input is 400,** with oRPC's JSON error body. An unknown path, or a known path with the wrong method, is 404. The Go Worker answers 422 and 405 there.
- **Only `follow()` has unit tests.** The Worker itself is tested live, against a running or deployed server ([testing.md](testing.md)).
- **Every workaround in the code names its upstream issue.** The table, and what to do when one closes, is in [upstream.md](upstream.md).
