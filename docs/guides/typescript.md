---
title: The same in TypeScript (oRPC)
nav_order: 10
parent: Guides
---

# The same in TypeScript (oRPC)

This page gets you started on the same design in TypeScript: a contract written with [oRPC](https://orpc.dev) and Zod, a Worker that serves it, and the same specs, SDKs, CLI and tests. Read it if you want the Free plan, the lowest cost per request, or a TypeScript codebase.

There is no `dev new` for it. You copy files from a checkout of [orpc-api](https://github.com/joeblew999/orpc-api), where the TypeScript version of the notes API lives in `api/ts/`. Every path on this page is a path in that repository.

That repository's own TypeScript Worker is checked on every push and was tested on Cloudflare. Copying it into a new repository, as described here, was not run for this page: the steps are read from the code.

## See it run first

In a checkout of orpc-api:

```sh
mise install && mise run setup
mise run api:ts:check            # typecheck, unit tests, and the committed specs match the contract
mise run api:ts:dev              # the API under cf dev: http://localhost:5173
mise run api:ts:migrate:local    # in another shell, the first time: creates the tables
```

There is no native run: the Worker only runs under `cf dev` and on Cloudflare.

## What to copy

| Copy | What it is | Change |
|---|---|---|
| `api/ts/src/contract.ts` | The contract, as the model for yours: every route, its input and output as Zod schemas, and what Fern needs | Rewrite it for your API |
| `api/ts/src/index.ts` | The Worker: the contract implemented on D1, the SSE stream, the WebSocket, the two spec routes | Rewrite the handlers |
| `api/ts/src/follow.ts` | The one feed behind SSE and the WebSocket, which makes them gap-free | Nothing. Give it your own source of events |
| `api/ts/src/hub.ts` | The hub: a Durable Object that wakes waiting streams | Nothing |
| `api/ts/src/asyncapi.ts` | The AsyncAPI generator (oRPC has none) | Nothing |
| `api/ts/src/specs.ts` | Both specs from any contract | Nothing |
| `api/ts/spec.ts`, `api/ts/spec-files.ts` | The command that writes the two spec files, or checks them | The names it imports from your contract |
| `api/ts/cloudflare.config.ts`, `api/ts/package.json`, `api/ts/tsconfig.json`, `api/ts/vite.config.ts`, `api/ts/vitest.config.ts`, `api/ts/test/` | The Worker's settings, the pinned packages, the unit tests of the feed | The Worker's name |
| `migrations/` | The D1 schema | Your tables |
| `sdk/fern/apis/api/` | The Fern folder: the two generated specs and `generators.yml` | The names in `generators.yml` ([Change the names](sdks.md#change-the-names)) |
| `sdk/package.json`, `sdk/package-lock.json`, `sdk/tsconfig.base.json`, `sdk/fern/fern.config.json` | Fern itself, pinned | The organization name |
| `test/live-test.mjs`, `test/sdk-live-test.mjs`, `test/soak.mjs`, `test/soak-go/` | The test programs. They only know a URL | The SDK's names |
| From `mise.toml`: the `[tools]` and `[env]` tables, `setup`, every `api:` task and every `sdk:` task | The tasks | See below |

What needs care in the tasks:

- **The tasks call the `dev` tool as `go run ./dev`.** In your repository, add the tool to `[tools]` as `"go:github.com/joeblew999/orpc-api/dev" = "latest"` and write `dev` in place of `go run ./dev` and `go run ../../dev`.
- **The tool finds the repository by a `go.work` file at its root** and stops without one. Keep the folder names `api/ts/`, `sdk/`, `migrations/` and `test/`: the tool looks for them.
- **`api:ts:migrate` and `api:ts:migrate:local` take `-worker` with the Worker's name.** The database is that name followed by `-db`. With a name other than `orpc-api`, `api:ts:migrate:local` also needs `-port "$PORT"`.
- **`dev workflows -into .` writes the Go API's jobs too.** In a repository with `api/ts/` and no Go API, remove those jobs from the written files by hand.

## Where each thing lives, in Go and in TypeScript

The left column is a project made by `dev new`; the right is orpc-api's `api/ts/`.

| Thing | Go | TypeScript |
|---|---|---|
| The contract | `api/go/api/contract.go`: Huma operations | `api/ts/src/contract.ts`: oRPC procedures |
| A route's method and path | `Method` and `Path` of the operation | `openapi({ method, path })` |
| Input and output, and their validation | Go structs; the struct tags are the rules (`minimum:"1"`) | Zod schemas (`z.number().min(1)`) |
| A rule the schema cannot say | The input's `Resolve` method | `.refine(...)` on the schema |
| The SDK method's name | `OperationID`, `Tags`, and `Extensions` on the operation | `operationId`, `tags`, and the `spec` hook |
| Paging and streaming for the SDKs | `x-fern-pagination`, `x-fern-streaming` in `Extensions` | The same two, in the `spec` hook |
| The WebSocket channel | `asyncapi.Operation(...)` around the operation | `.meta(asyncapi({...}))` on the procedure |
| The handlers | `api/go/api/handlers.go` | `api/ts/src/index.ts` |
| The gap-free feed | The package `follow`, imported | `api/ts/src/follow.ts`, copied |
| The hub | `api/go/worker/hub.mjs` | `api/ts/src/hub.ts` |
| Writing the specs | `mise run api:go:spec` (`api/go/cmd/spec/`) | `mise run api:ts:spec` (`api/ts/spec.ts`) |
| The Fern folder | `sdk/fern/apis/api-go/` | `sdk/fern/apis/api/` |
| The Worker's settings | `api/go/cloudflare.config.ts` | `api/ts/cloudflare.config.ts` |
| The check | `mise run api:go:check` | `mise run api:ts:check` |

From the specs on, nothing differs: the same Fern groups, the same `mise run sdk:gen`, the same test programs. The SDKs made from one server's specs pass the tests against the other server ([findings](../findings.md)).

## The differences that matter

| | Go | TypeScript (oRPC) |
|---|---|---|
| CPU per request on Cloudflare | 1 to 3 ms for a read, 4 to 6 ms for a write; about 10 ms for the first request in a new isolate ([benchmarks](../benchmarks.md)) | About 1 ms |
| Cloudflare plan | Free to try it, Workers Paid for production (the first requests in a new isolate reach Free's 10 ms) | Within the Free plan's 10 ms |
| Invalid input | 422, with a problem body that names the field (`errors[].location`) | 400, with oRPC's JSON error body |
| A known path with the wrong method | 405 | 404 |
| Running without Cloudflare | `mise run api:go:run`: natively, in memory | No. Only `cf dev` |
| Local run of the real-time tests | In `mise run api:go:check`, natively and under workerd | None. `mise run api:ts:check` is typecheck, unit tests and spec drift; the streams are tested deployed |
| MCP endpoint for agents | `/api/mcp` | None |
| Types in the SDKs | One shared `Note` type | One type per response: oRPC writes the schema inline |
| Starting a project | `dev new` | Copying, as above |

A client that checks for a validation error must know which server it talks to: the status differs, and so does the body.

## Going further

- **How the TypeScript Worker is built,** the AsyncAPI generator and its limits: [oRPC Worker internals](../api.md).
- **OAuth, idempotency, uploads, webhooks, a WebSocket the client sends on:** orpc-api's showcase contract, `sdk/harness/src/contract.ts`, has one of each ([Fern folder internals](../sdk.md#the-orpc-showcase)).
- **What a client does when a stream ends** is the same for both servers: [What a client does](../realtime.md#what-a-client-does).
- **Deploying and testing** follow [Deploy to Cloudflare](deploy.md) and [Test locally and on Cloudflare](testing.md), with `api:ts:` tasks in place of `api:go:` ones and `API_URL` in place of `API_GO_URL`.
