---
title: The same in TypeScript (oRPC)
nav_order: 10
parent: Guides
---

# The same in TypeScript (oRPC)

This page gets you started on the same design in TypeScript: a contract written with [oRPC](https://orpc.dev) and Zod, a Worker that serves it, and the same specs, SDKs, CLI and tests. Read it if you want the Free plan, the lowest cost per request, or a TypeScript codebase.

There is no `charter new` for it. You copy files from a checkout of [charter](https://github.com/joeblew999/charter), where the TypeScript version of the notes API lives in `examples/notes-ts/`. Every path on this page is a path in that repository.

That repository's own TypeScript Worker is checked on every push and was tested on Cloudflare. Copying it into a new repository, as described here, was not run for this page: the steps are read from the code.

## See it run first

In `examples/notes-ts` of a checkout of charter:

```sh
mise install && mise run setup
mise run check            # typecheck, unit tests, and the committed specs match the contract
mise run dev              # the API under cf dev: http://localhost:5173
mise run migrate:local    # in another shell, the first time: creates the tables
```

There is no native run: the Worker only runs under `cf dev` and on Cloudflare.

## What to copy

| Copy | What it is | Change |
|---|---|---|
| `examples/notes-ts/src/contract.ts` | The contract, as the model for yours: every route, its input and output as Zod schemas, and what Fern needs | Rewrite it for your API |
| `examples/notes-ts/src/index.ts` | The Worker: the contract implemented on D1, the SSE stream, the WebSocket, the two spec routes | Rewrite the handlers |
| `examples/notes-ts/src/follow.ts` | The one feed behind SSE and the WebSocket, which makes them gap-free | Nothing. Give it your own source of events |
| `examples/notes-ts/src/hub.ts` | The hub: a Durable Object that wakes waiting streams | Nothing |
| `examples/notes-ts/src/asyncapi.ts` | The AsyncAPI generator (oRPC has none) | Nothing |
| `examples/notes-ts/src/specs.ts` | Both specs from any contract | Nothing |
| `examples/notes-ts/spec.ts`, `examples/notes-ts/spec-files.ts` | The command that writes the two spec files, or checks them | The names it imports from your contract |
| `examples/notes-ts/cloudflare.config.ts`, `examples/notes-ts/package.json`, `examples/notes-ts/tsconfig.json`, `examples/notes-ts/vite.config.ts`, `examples/notes-ts/vitest.config.ts`, `examples/notes-ts/test/` | The Worker's settings, the pinned packages, the unit tests of the feed | The Worker's name |
| `examples/notes-ts/migrations/` | The D1 schema | Your tables |
| `examples/notes-ts/fern/` | The Fern folder: the two generated specs and `generators.yml` | The names in `generators.yml` ([Change the names](sdks.md#change-the-names)) |
| `examples/notes-ts/package.json`, `examples/notes-ts/package-lock.json`, `examples/notes-ts/fern/fern.config.json` | The Worker's packages, and Fern itself, pinned | The organization name |
| `examples/notes-go/test/live-test.mjs`, `examples/notes-go/test/sdk-live-test.mjs`, `examples/notes-go/test/soak.mjs`, `examples/notes-go/test/soak-go/` | The test programs. They only know a URL | The SDK's names |
| `examples/notes-ts/mise.toml` | The tools and the tasks | See below |

What needs care in the tasks:

- **The tasks call the `charter` tool as `go run ../../cmd/charter`.** In your repository, add the tool to `[tools]` as `"go:github.com/joeblew999/charter/cmd/charter" = "latest"` and write `charter` in place of `go run ../../cmd/charter`.
- **The tool finds the project by its `mise.toml` beside a `fern/` folder** and stops without one. Keep the folder names `fern/`, `sdk/` and `migrations/`: the tool looks for them. The live tests are run from `examples/notes-go/test/` by relative path: copy that folder too, and change the paths in the `live-test` and `soak` tasks.
- **`migrate` and `migrate:local` take the Worker's name from `API_URL`:** the first label of its host. The database is that name followed by `-db`.
- **`charter workflows -into .` writes a project's four workflows.** They name tasks a Go project has (`sdk:publish:check`, `release:tags`, `cloudflare:token`): add those you want to your `mise.toml`, or remove the steps from the written files by hand.

## Where each thing lives, in Go and in TypeScript

The left column is a project made by `charter new`; the right is charter's `examples/notes-ts/`.

| Thing | Go | TypeScript |
|---|---|---|
| The contract | `examples/notes-go/api/contract.go`: Huma operations | `examples/notes-ts/src/contract.ts`: oRPC procedures |
| A route's method and path | `Method` and `Path` of the operation | `openapi({ method, path })` |
| Input and output, and their validation | Go structs; the struct tags are the rules (`minimum:"1"`) | Zod schemas (`z.number().min(1)`) |
| A rule the schema cannot say | The input's `Resolve` method | `.refine(...)` on the schema |
| The SDK method's name | `OperationID`, `Tags`, and `Extensions` on the operation | `operationId`, `tags`, and the `spec` hook |
| Paging and streaming for the SDKs | `x-fern-pagination`, `x-fern-streaming` in `Extensions` | The same two, in the `spec` hook |
| The WebSocket channel | `asyncapi.Operation(...)` around the operation | `.meta(asyncapi({...}))` on the procedure |
| The handlers | `examples/notes-go/api/handlers.go` | `examples/notes-ts/src/index.ts` |
| The gap-free feed | The package `follow`, imported | `examples/notes-ts/src/follow.ts`, copied |
| The hub | `go/worker/hub.mjs` | `examples/notes-ts/src/hub.ts` |
| Writing the specs | `mise run spec` (`examples/notes-go/cmd/spec/`) | `mise run spec` (`examples/notes-ts/spec.ts`) |
| The Fern folder | `examples/notes-go/fern/` | `examples/notes-ts/fern/` |
| The Worker's settings | `examples/notes-go/cloudflare.config.ts` | `examples/notes-ts/cloudflare.config.ts` |
| The check | `mise run check` | `mise run check` |

From the specs on, nothing differs: the same Fern groups, the same `mise run sdk:gen`, the same test programs. The SDKs made from one server's specs pass the tests against the other server ([findings](../findings.md)).

## The differences that matter

| | Go | TypeScript (oRPC) |
|---|---|---|
| CPU per request on Cloudflare | Under 1 ms for a simple read, 1 to 2 ms with a database read, about 2 ms for a write; about 10 ms for the first request in a new isolate ([benchmarks](../benchmarks.md)) | 0 to 2 ms |
| Cloudflare plan | Free to try it, Workers Paid for production (the first requests in a new isolate reach Free's 10 ms) | Within the Free plan's 10 ms |
| Invalid input | 422, with a problem body that names the field (`errors[].location`) | 400, with oRPC's JSON error body |
| A known path with the wrong method | 405 | 404 |
| Running without Cloudflare | `mise run run`: natively, in memory | No. Only `cf dev` |
| Local run of the real-time tests | In `mise run check`, natively and under workerd | None. `mise run check` is typecheck, unit tests and spec drift; the streams are tested deployed |
| MCP endpoint for agents | `/api/mcp` | None |
| Types in the SDKs | One shared `Note` type | One type per response: oRPC writes the schema inline |
| Starting a project | `charter new` | Copying, as above |

A client that checks for a validation error must know which server it talks to: the status differs, and so does the body.

## Going further

- **How the TypeScript Worker is built,** the AsyncAPI generator and its limits: [oRPC Worker internals](../api.md).
- **OAuth, idempotency, uploads, webhooks, a WebSocket the client sends on:** charter's showcase contract, `examples/showcase-ts/src/contract.ts`, has one of each ([Fern folder internals](../sdk.md#the-orpc-showcase)).
- **What a client does when a stream ends** is the same for both servers: [What a client does](../realtime.md#what-a-client-does).
- **Deploying and testing** follow [Deploy to Cloudflare](deploy.md) and [Test locally and on Cloudflare](testing.md), with `` tasks in place of `` ones and `API_URL` in place of `API_URL`.
