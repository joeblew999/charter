---
title: Replace the example with your API
nav_order: 0
parent: Guides
---

# Replace the example with your API

A project made by `dev new` ([Getting started](../getting-started.md)) starts as a notes API, so every check passes before you change anything. This guide names every file that holds the notes example, and says what to do with each one when you put your own API in its place. Read it when you are ready to write your own operations.

There is no `dev new` option that starts without the notes. You remove them by hand, with this page.

## Add yours first, then take the notes out

Do it in two passes. The checks pass after each one.

```sh
mise run api-go:spec    # after every change to the contract: writes the two spec files again
mise run check          # lint, Go tests, spec drift, the Wasm build, the live and MCP tests natively and under workerd
```

1. **Add your operations beside the notes.** Write them in `api-go/api/contract.go`, their handlers in `api-go/api/handlers.go`, their storage in `api-go/api/store.go` and a new file in `migrations/`. Run the two commands above. Nothing in the project's tests fails because an operation was added: the MCP tests check that the notes tools are listed with the right schemas, not that they are the only tools.
2. **Take the notes out,** file by file, with the table below. Each test program in the table tests the notes, so it is rewritten for your API or removed together with the task that runs it.

## Which files hold the notes, and what to do with each

| File | What it holds of the notes | What to do |
|---|---|---|
| `api-go/api/contract.go` | `Note`, the input and output structs, and five routes in `Routes`: `hello`, `listNotes`, `createNote`, `watchNotes` (the SSE stream) and `liveNotes` (the WebSocket channel). Also `Title`, `Description`, `LiveTitle`, and `END` (the stream's end marker) | Write your own structs and routes. Keep `hello`, or change the `-url` of the two `api-go:test:*` tasks in `mise.toml`: they wait for `/api/hello` to answer. Set `Description`. Remove `END` when no stream is left |
| `api-go/api/handlers.go` | `Env` (what the platform gives the handlers: `Var`, `Store`, `Hub`), the handlers `list`, `create`, `watch` and `live`, the `Resolve` rule on `CreateInput`, and `feed` (what both streams read from) | Keep `Handler`, `origin` and `hello`: they serve your contract, the two specs and `/api/mcp` whatever the operations are. Replace the note handlers with yours. Remove `feed`, `watch`, `live` and `flushing` when no stream is left |
| `api-go/api/store.go` | The `Store` and `Hub` interfaces, `SQLStore` (the `notes` table through `database/sql`) and `MemStore` (the store and the hub in memory, for the native build and the Go tests) | Change `Store` to what your handlers need, and write both implementations: the SQL one runs on Cloudflare, the in-memory one runs in `go test` and `mise run api-go:run` |
| `migrations/0001_init.sql` | The `notes` table | A migration is applied once, by its file name. In a project that was never deployed, replace this file's content with your tables. In a deployed one, add a new numbered file |
| `api-go/platform_js.go` | The bindings on Cloudflare: `DB` (D1) as an `SQLStore`, and `HUB` as `hub.DurableObject[api.Note]("HUB", "notes")`, the hub of the notes feed | Keep the `Store` function. Keep the `Hub` line only with a stream (next section), with your type and a name for your feed |
| `api-go/platform_other.go` | The native build: one `MemStore` as both store and hub | Follow your `Env` |
| `api-go/cloudflare.config.ts` | The Worker's bindings: `DB`, `HUB`, and the export of the `NotesHub` class | Keep `DB` while you store anything. Keep `HUB` and `NotesHub` only with a stream |
| `api-go/worker/hub.mjs`, and the `NotesHub` line in `api-go/worker/index.mjs` | The Durable Object that sends each published message to every subscriber. Only its name says notes | Keep both with a stream, remove both without. The other files in `api-go/worker/` are not part of the example: keep them |
| `api-go/api/api_test.go` | The Go tests of the notes routes, the SSE wire format and the specs. Its helpers `server`, `do`, `spec` and `at` are general | Keep the helpers, `TestHello`, `TestUnknownPathAndMethod` and `TestTheWorkerServesBothSpecsWithItsOriginAsServer`. Write the rest for your operations |
| `api-go/api/mcp_test.go` | `tools/list` holds the three notes tools with their exact schemas; tool calls create and list notes; errors name `listNotes` and `createNote` | Keep the helpers `mcp` and `toolCall`, and `TestEveryRouteNamesItsOperation`. Rewrite the three other tests with your tools |
| `test/mcp-test.mjs` | The same through the official MCP client: the notes tools, their schemas, calls and errors. Run by `api-go:test:native`, `api-go:test:workerd` and `api-go:live-test` | Keep the connection, negotiation and transport checks (the start of the loop, and everything after it). Replace the checks that name `listNotes` and `createNote` with your tools |
| `test/live-test.mjs` | Creates a note and expects it on `/api/notes/watch` (SSE) and `/api/notes/live` (WebSocket), then resumes. Run by the same three tasks | With a stream: change the paths and the body. Without: delete the file and its `node ../test/live-test.mjs ...` part from those tasks in `mise.toml` |
| `test/sdk-live-test.mjs` | The same through the generated TypeScript SDK: `notes.create()`, `notes.watch()`, `liveNotes.connect()`. Run by `api-go:live-test` | The same choice: your SDK methods, or delete it and its part of the task |
| `test/soak.mjs`, `test/soak-go/` | The long real-time test against the deployed Worker: every client must receive every note once, in order, through a redeploy and a disconnect. Run by `api-go:soak` | With a stream: change the routes and SDK calls. Without: delete both and the `api-go:soak` task |
| `sdk/fern/apis/api-go/openapi.json`, `sdk/fern/apis/api-go/asyncapi.json` | The specs of the notes API | Never edit them. `mise run api-go:spec` writes them from your contract |
| `sdk/fern/apis/api-go/generators.yml` | Nothing of the notes: the SDK's names here come from the project's name | Leave it. The names that say notes are in the contract (next section but one) |

## The hub, and when you still need it

The hub is the Durable Object in `api-go/worker/hub.mjs`. The notes API uses it for one thing: `create` publishes the new note to it, and every open stream is woken by it. The notes themselves are read from the store, so the hub holds nothing ([realtime.md](../realtime.md)).

- **You need it if your API streams:** an SSE operation or a WebSocket channel where clients receive items as they are created. Keep `Hub` in `Env`, `feed` in `api-go/api/handlers.go`, the `Hub` line in `api-go/platform_js.go`, and the `HUB` binding and `NotesHub` export.
- **You don't need it if every operation answers once.** Remove all of those, `api-go/worker/hub.mjs`, and the `NotesHub` line in `api-go/worker/index.mjs`.
- **Not tested:** removing the hub from a Worker that was already deployed with it. Cloudflare keeps a deployed Durable Object class until it is told the class is gone, and this page does not cover that step.

## The names Fern gives the SDK

The SDK's method names come from the contract, not from Fern's settings. In `api-go/api/contract.go`:

| In the contract | In the SDK |
|---|---|
| `sdk("notes", "list", ...)` in an operation's `Extensions` | `client.notes.list()`, and `notes list` in the CLI |
| `Tags: []string{"notes"}` | The group in the API reference |
| `OperationID: "listNotes"` | The MCP tool's name |
| `asyncapi.Channel{Name: "liveNotes", ...}` | `client.liveNotes.connect()` |

Give your operations their own group and method names there. The client's own name (`BillingApiClient` for a project named `billing-api`) is in `sdk/fern/apis/api-go/generators.yml` and was set by `dev new`.

## When you are done

```sh
mise run api-go:spec                 # the specs, from your contract
mise run check                       # must pass
mise run sdk:gen api-go typescript   # Fern, in Docker: the SDK now has your methods
```

Then search the project for what is left of the example: `grep -rni note api-go test migrations mise.toml`.
