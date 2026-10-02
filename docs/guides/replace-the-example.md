---
title: Make the example yours
nav_order: 1
parent: Guides
---

# Make the example yours

A project made by `charter new` starts as the notes API, so every check passes before you change anything. This page names each file that holds the notes, and says what to do with it when you put your own API in. There is no option that starts without the notes: you take them out by hand, with this page.

## Add yours first, then take the notes out

Two passes. `mise run check` passes after each.

1. **Add your operations beside the notes** ([Change the contract](contract.md)). No test fails because an operation was added.
2. **Take the notes out,** file by file, with the table below.

```sh
mise run spec     # after every contract change
mise run check    # lint, tests, spec drift, the TinyGo build, the live and MCP tests natively and under workerd
```

## Which files hold the notes

| File | What it holds of the notes | What to do |
|---|---|---|
| `api/contract.go` | `Note`, the input and output structs, five routes (`hello`, `listNotes`, `createNote`, `watchNotes`, `liveNotes`), `Title`, `Description`, `LiveTitle`, `END` | Write your structs and routes. Keep `hello`, or change the `-url` that `test:native` and `test:workerd` wait for in `mise.toml`, and the warm paths in `worker.mjs`. Remove `END` when no stream is left |
| `api/handlers.go` | `Env` (`Var`, `Store`, `Hub`), the handlers, the `Resolve` rule on `CreateInput`, and `feed` (what both streams read) | Keep `Handler` and `origin`: they serve any contract, its two specs and `/api/mcp`. Replace the note handlers. Remove `feed`, `watch`, `live` and `flushing` when no stream is left |
| `api/store.go`, `api/store_js.go` | The `Store` and `Hub` interfaces; `D1Store` (what runs on Cloudflare), `SQLStore` (`database/sql`), `MemStore` (memory, for the native build and `go test`) | Change `Store` to what your handlers need. Write `D1Store` and `MemStore` for it |
| `migrations/0001_init.sql` | The `notes` table | Never deployed: replace its content. Deployed: add a new numbered file |
| `platform_js.go` | The bindings on Cloudflare: `DB` as a `D1Store`, `HUB` as the hub of the notes feed | Keep `Store`. Keep `Hub` only with a stream, with your type and feed name |
| `platform_other.go` | The native build: one `MemStore` as store and hub | Follow your `Env` |
| `cloudflare.config.ts` | The bindings `DB` and `HUB`, and the `Hub` class export | Keep `DB` while you store anything. Keep `HUB` and `Hub` only with a stream |
| `worker.mjs` | The `Hub` export line | Keep it with a stream |
| `api/api_test.go` | The Go tests of the notes routes. The helpers `server`, `do`, `spec` and `at` are general | Keep the helpers, `TestHello`, `TestUnknownPathAndMethod` and `TestTheWorkerServesBothSpecsWithItsOriginAsServer`. Write the rest for your operations |
| `api/mcp_test.go` | The notes tools, their schemas, calls and errors | Keep `mcp`, `toolCall` and `TestEveryRouteNamesItsOperation`. Rewrite the three others |
| `api/contract_test.go` | Nothing of the notes: it holds for any contract | Leave it |
| `test/mcp-test.mjs` | The notes tools through the official MCP client | Replace the checks that name `listNotes` and `createNote` |
| `test/live-test.mjs`, `test/sdk-live-test.mjs` | A note must arrive on the SSE stream and the WebSocket, raw and through the TypeScript SDK | With a stream: change paths and SDK calls. Without: delete them and their part of `test:native`, `test:workerd` and `live-test` |
| `test/soak.mjs`, `test/soak-go/` | The long real-time test | With a stream: change routes and SDK calls. Without: delete both and the `soak` task |
| `fern/openapi.json`, `fern/asyncapi.json` | The specs of the notes API | Never edit. `mise run spec` writes them |

## The hub: keep it only if you stream

The hub is the Durable Object that wakes open streams when an item is created. It stores nothing ([How real-time works](../realtime.md)).

- **Your API streams** (an SSE operation or a WebSocket channel): keep `Hub` in `Env`, `feed`, the `Hub` line in `platform_js.go`, the `HUB` binding and the `Hub` export.
- **Every operation answers once:** remove all of those.
- **Not tested:** removing the hub from a Worker already deployed with it. Cloudflare keeps a deployed Durable Object class until it is told the class is gone.

## The names in the SDKs

`charter new` renames the Worker, its URL, the Go module and Fern's organisation. These still say notes:

| Where | What it names | In a new project |
|---|---|---|
| `sdk(group, method, ...)` on each operation in `api/contract.go` | The SDK method: `client.notes.list()`, `notes list` in the CLI | the notes groups |
| `OperationID` | The MCP tool's name | `listNotes` and the others |
| `asyncapi.Channel{Name: ...}` | The WebSocket client: `client.liveNotes.connect()` | `liveNotes` |
| `namespaceExport` in `fern/generators.yml` | The TypeScript client class, with `Client` added | `NotesClient` |
| `packageName`, `binaryName` in `fern/generators.yml` | The Go package of the SDK, the CLI's binary | `notes` |
| `title` in `fern/docs.yml` | The reference site's title | `Notes API` |

The test programs import the SDK by those names: `NotesClient` in `test/sdk-live-test.mjs` and `test/soak.mjs`, the binary in `test/soak.mjs`, the Go package in `test/soak-go/main.go`. Change them together. The Go SDK's module path (`config.module.path` of the `go` group) must stay `<your module>/sdk/go`: `mise run sdk:publish` fails otherwise.

## When you are done

```sh
mise run spec                  # the specs, from your contract
mise run check                 # must pass
mise run sdk:gen typescript    # Docker: the SDK now has your methods
grep -rni note api test migrations fern/generators.yml mise.toml   # what is left of the example
```
