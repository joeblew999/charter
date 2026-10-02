---
title: The TypeScript (oRPC) version
nav_order: 10
parent: Guides
---

# The TypeScript (oRPC) version

The same design from an [oRPC](https://orpc.dev) contract: a Worker that serves it, and the OpenAPI spec, an AsyncAPI spec, SDKs and a CLI generated from it. Read it if you use oRPC, or want a TypeScript codebase. The example is `examples/notes-ts/`: the notes API on oRPC `2.0.0-beta.40` and Zod 4, the same API as the Go one.

## What oRPC users get

- **An AsyncAPI spec from the contract.** oRPC generates none; `examples/notes-ts/src/asyncapi.ts` does ([below](#the-asyncapi-generator)).
- **SDKs in other languages and a CLI,** by Fern, from the two specs. The contract carries what Fern needs, so nothing is patched into the JSON.
- **A gap-free feed** behind SSE and the WebSocket: `examples/notes-ts/src/follow.ts` ([How real-time works](../realtime.md)).
- **The same tasks and the same tests** as a Go project.

## See it run

```sh
cd examples/notes-ts
mise install && mise run setup
mise run check            # typecheck, the feed's unit tests, spec drift
mise run dev              # under cf dev: http://localhost:5173
mise run migrate:local    # in another shell, the first time
```

There is no native run: the Worker runs under `cf dev` and on Cloudflare.

## The files

In `examples/notes-ts/`: `src/contract.ts` is **the contract**; `src/index.ts` the Worker (the contract implemented on D1, the SSE stream, the WebSocket, the two spec routes); `src/follow.ts` the feed; `src/hub.ts` the hub (oRPC's `DurablePublisherObject`); `src/asyncapi.ts` the AsyncAPI generator; `src/specs.ts` both specs from any contract; `spec.ts` the command behind `mise run spec`; `fern/` the generated specs and Fern's settings.

## Go and TypeScript, side by side

| Thing | Go (Huma) | TypeScript (oRPC) |
|---|---|---|
| The contract | `api/contract.go`: Huma operations | `src/contract.ts`: oRPC procedures |
| Method and path | `Method`, `Path` on the operation | `openapi({ method, path })` |
| Input, output, validation | Go structs; the tags are the rules | Zod schemas |
| A rule the schema cannot say | The input's `Resolve` method | `.refine(...)` |
| SDK method names, pagination, streaming | `OperationID`, `Tags`, `Extensions` | `operationId`, `tags`, and the `spec` hook |
| The WebSocket channel | `asyncapi.Operation(...)` | `.meta(asyncapi({...}))` |
| The feed | The package `follow`, imported | `src/follow.ts` |
| Writing the specs | `mise run spec` (`./cmd/spec`) | `mise run spec` (`spec.ts`) |

From the specs on, nothing differs: the same Fern groups, tasks and test programs. The SDKs made from one server's specs pass the tests against the other ([Findings](../findings.md)), and a test holds the two specs to one surface (`examples/surface_test.go`).

## The differences that matter

| | Go | TypeScript |
|---|---|---|
| Invalid input | 422, a problem body that names the field | 400, oRPC's JSON error body |
| A known path with the wrong method | 405 | 404 |
| Running without Cloudflare | `mise run run` | No |
| Real-time tests in `mise run check` | Natively and under workerd | None: the streams are tested deployed |
| MCP endpoint | `/api/mcp` | None |
| Types in the SDKs | One shared `Note` type | One type per response: oRPC writes the schema inline |
| Starting a project | `charter new` | Copy the example (below) |

The cost per request is the same, measured side by side ([Benchmarks](../benchmarks.md)). A client that checks for a validation error must know which server it talks to.

## The AsyncAPI generator

`examples/notes-ts/src/asyncapi.ts` writes AsyncAPI 3.0.0 from an oRPC contract the way `@orpc/openapi` writes OpenAPI. It uses only oRPC's public APIs (a metadata plugin, `walkProcedureContractsAsync`, the JSON Schema converters), so it needs no fork.

```ts
live: oc
  .meta(openapi({ method: "GET", path: "/api/notes/live" }))
  .meta(asyncapi({ channel: "liveNotes", address: "/api/notes/live", operationId: "receiveNote", message: "Note", summary: "..." }))
  .input(z.object({ after }))            // the channel's query parameters
  .output(asyncIteratorObject(note)),    // what the server sends
```

| In the contract | In the AsyncAPI spec |
|---|---|
| `asyncapi({ channel, address })` on a procedure | A WebSocket channel. It is left out of OpenAPI |
| The output, `asyncIteratorObject(x)` | A `receive` operation whose message payload is `x` |
| An object input | The channel's query parameters: `liveNotes.connect({ after })` in the SDK |
| An `asyncIteratorObject(y)` input | A `send` operation: what the client sends. The showcase uses it |

Limits: one message type each way; a channel cannot have query parameters and a send side together; a procedure with more than one `.input()` or `.output()` is refused. oRPC asked for it as a community package first ([middleapi/orpc#2115](https://github.com/middleapi/orpc/issues/2115)); that is [planned](../plans/next.md).

## What else the contract does for Fern

- **`openapi({ operationId, tags, spec })`** names the SDK method and adds Fern's `x-fern-*` extensions.
- **OpenAPI is 3.1.1,** asked for in `src/specs.ts`: oRPC 2.0 defaults to 3.2.0, which Fern rejects.
- **The SSE response's schema is the note,** set by the `spec` hook: oRPC describes its own event envelope there.
- **The rest** (OAuth, idempotency, uploads, webhooks, a two-way WebSocket): [Fern features](fern-features.md#from-an-orpc-contract).

## Start a TypeScript project

`charter new` makes Go projects only. For TypeScript, copy `examples/notes-ts/` and change it. This was not run for this page: the steps are read from the code.

1. **Copy the folder,** and the test programs its tasks use from `examples/notes-go/test/`.
2. **Pin the tool** in `mise.toml`, under `[tools]`: `"go:github.com/joeblew999/charter/cmd/charter" = "latest"`. Write `charter` where the tasks say `go run ../../cmd/charter`, and your own paths where they say `../notes-go/test/`.
3. **Rename the Worker** in `cloudflare.config.ts`, `package.json` and the default of `API_URL`.
4. **Write your contract** in `src/contract.ts` and your handlers in `src/index.ts`. Leave `follow.ts`, `asyncapi.ts`, `specs.ts` and `spec-files.ts` as they are.
5. **Run `charter workflows`** for the GitHub workflows, then `mise run spec` and `mise run check`.

## Limits

- **Only the feed has unit tests.** The Worker is tested live, against a deployed server.
- **The static page** (`examples/notes-ts/public/index.html`) reads `GET /api/notes` as a plain array, which the API does not answer. No test covers it.
- **oRPC is a beta** (`2.0.0-beta.40`). Moving to 2.0.0 final is [planned](../plans/next.md).
