---
title: Start here
nav_order: 1
permalink: /
---

# Docs

Everything written about this repo lives in this folder: one place for developers and for agents. Folder READMEs and `AGENTS.md` only point here. This page is the index of the pages and says what is what: each part, its name, and where it lives. GitHub Pages renders the folder at https://joeblew999.github.io/orpc-api/ straight from these files, with no build step ([dev.md](dev.md#the-docs-site)).

| Page | What it covers |
|---|---|
| **This page** | The index, and what is what |
| [rules.md](rules.md) | The working rules. Read them before changing anything |
| [api.md](api.md) | `api/`: the oRPC Worker (TypeScript), the notes API's routes, and how to copy the pattern into a TypeScript project |
| [api-go.md](api-go.md) | `api-go/`: the Go Worker (Huma on workers-go), what Huma needs to run there, and how to start a Go project from it |
| [mcp.md](mcp.md) | The Go Worker's MCP endpoint: the contract as tools (`api-go/humamcp/`) |
| [sdk.md](sdk.md) | `sdk/`: Fern, the generated SDKs and the Fern CLI, the oRPC showcase, and the harness Worker |
| [showcase-go.md](showcase-go.md) | The Go showcase: every Fern feature from a Huma contract, feature by feature |
| [realtime.md](realtime.md) | The real-time design both servers share: the rules, the feed, and what a client does |
| [testing.md](testing.md) | `test/`: the tests both servers must pass |
| [dev.md](dev.md) | `dev/`: the tool behind the tasks, the GitHub workflows, releases, a new project, the docs site |
| [benchmarks.md](benchmarks.md) | Measured cost per request of both Workers, and how to measure it |
| [upstream.md](upstream.md) | Every upstream issue we work around, and ones found and not filed |
| [findings.md](findings.md) | Verified results only, newest last |
| [plans/](plans/next.md) | What is not built yet: [next.md](plans/next.md) (the list), [realtime.md](plans/realtime.md), [asyncapi.md](plans/asyncapi.md), [mcp.md](plans/mcp.md), [performance.md](plans/performance.md), [microservices.md](plans/microservices.md) |
| [writing.md](writing.md) | The rules a page in `docs/` is held to, and how they are checked |

## What is what

The one idea: **you write a contract in code; everything else is generated from it.**

```mermaid
flowchart LR
    C["Contract<br/>(code you write)"] --> S["openapi.json + asyncapi.json<br/>(generated)"]
    S --> F[Fern]
    F --> K["SDKs: Go, TypeScript"]
    F --> L["CLI"]
    F --> D["API docs"]
    C --> V["The server validates requests<br/>against the same contract"]
    C --> M["MCP tools<br/>(Go Worker)"]
```

### The notes API, twice

There are two contracts for the notes API because there are two servers. They describe the same API, and a test keeps them the same. A real project has one: pick the column for its language.

| | The oRPC Worker (TypeScript) | The Go Worker |
|---|---|---|
| **The contract (the source; you edit this)** | `api/src/contract.ts` (oRPC + Zod) | `api-go/api/contract.go` (Huma: Go structs and their tags) |
| The server | `api/src/index.ts` | `api-go/api/handlers.go` |
| OpenAPI generator | `@orpc/openapi` | Huma |
| AsyncAPI generator (the WebSocket) | `api/src/asyncapi.ts` (ours) | `api-go/asyncapi/` (ours, the same design) |
| Write the specs | `mise run api:spec` | `mise run api-go:spec` |
| **The specs (generated; never edit)** | `sdk/fern/apis/api/*.json` | `sdk/fern/apis/api-go/*.json` |
| Fern's settings for that API | `sdk/fern/apis/api/generators.yml` | `sdk/fern/apis/api-go/generators.yml` |
| Generate an SDK | `mise run sdk:gen api <group>` | `mise run sdk:gen api-go <group>` |
| Fern's output (gitignored) | `sdk/out/api/` | `sdk/out/api-go/` |
| The feed (gap-free real-time) | `follow()` in `api/src/follow.ts` | `Follow` in `api-go/follow/` |
| The hub (a Durable Object with hibernating WebSockets) | `api/src/hub.ts` (oRPC's) | `api-go/worker/hub.mjs` |
| The Worker on Cloudflare | `orpc-api` | `orpc-api-go` |
| Deployed at | https://orpc-api.gedw99.workers.dev/api/openapi.json | https://orpc-api-go.gedw99.workers.dev/api/openapi.json |
| MCP endpoint | no | `/api/mcp` |
| Also runs without Cloudflare | no | yes: `mise run api-go:run` |
| Check it locally | `mise run api:check` | `mise run api-go:check` |
| Test it deployed | `mise run api:live-test`, `mise run api:soak` | `mise run api-go:live-test`, `mise run api-go:soak` |
| Its page | [api.md](api.md) | [api-go.md](api-go.md) |

Shared by both: `test/` (the test programs), `migrations/` (the D1 schema), `sdk/` (Fern), `dev/` (the tool the tasks run).

### The showcase, twice

The showcase is a second, smaller API whose only job is to use every Fern feature: OAuth, idempotency, pagination, SSE, file upload, signed webhooks, a WebSocket both ways, audiences and an overlay. A test keeps its two contracts the same too.

| | The oRPC showcase | The Go showcase |
|---|---|---|
| **The contract** | `sdk/harness/src/contract.ts` | `api-go/showcase/contract.go` |
| The server | `sdk/harness/src/showcase.ts`, served by the harness Worker under `/api/mock` | `api-go/showcase/handlers.go`, run by `api-go/cmd/showcase/` |
| Write the specs | `mise run showcase:spec` | `mise run showcase-go:spec` |
| **The specs (generated; never edit)** | `sdk/fern/apis/showcase/*.json` | `sdk/fern/apis/showcase-go/*.json` |
| The Worker on Cloudflare | `orpc-sdk-harness` and `orpc-sdk-harness-api` | `orpc-showcase-go` |
| Check it locally | `mise run showcase:check`, `mise run sdk:harness:test` | `mise run showcase-go:check` |
| Its page | [sdk.md](sdk.md#the-orpc-showcase) | [showcase-go.md](showcase-go.md) |

### The names used in these pages

| Name | What it is |
|---|---|
| The contract | The code that defines an API's routes and schemas. The source of everything else |
| The specs | `openapi.json` and `asyncapi.json`, generated from a contract into a Fern folder |
| A Fern folder | `sdk/fern/apis/<api>/`: an API's specs and `generators.yml`. A group in that file is one SDK to generate |
| The oRPC Worker, the Go Worker | The two servers of the notes API: `api/` and `api-go/` |
| The feed | `follow()` / `Follow`: the one primitive that gives a client every note once, in order ([realtime.md](realtime.md)) |
| The hub | The `NotesHub` Durable Object: live fan-out only, stores nothing |
| The harness Worker | `sdk/harness/`: serves the oRPC showcase and runs Fern's TypeScript SDK inside workerd |
| The Fern CLI | The command-line program Fern generates for an API (`orpc-api notes list`). It runs on a developer's machine and calls the API over HTTPS, like an SDK |
| The `dev` tool | `dev/`: the Go program behind the mise tasks |
| `cf` | Cloudflare's own CLI, which the tasks use to run and deploy Workers |

Things that are easy to mix up:

- **Fern does not write servers.** It reads the two spec files and writes clients: SDKs in Go, TypeScript and more, a command-line program, docs. The server is ours, in either language.
- **"The CLI" is the Fern CLI.** It never runs on Cloudflare. It is not `cf`, `mise`, or the `dev` tool.
- **"Go" means two different things here.** `api-go/` is a Go *server*. A folder `go` under `sdk/out/` is a Go *client SDK* that Fern generated, and it exists for both servers.
- **The tests and the database schema are shared.** `test/` has one set of test programs for both servers (they take a URL, and `--sdk api-go` picks the SDKs generated from the Go specs), and `migrations/` is the one D1 schema.
- **What is the product and what is an example.** The `dev` tool and the Go packages in `api-go/` (`humaworkers`, `asyncapi`, `follow`, `humamcp`, `transport`, `specfile`) are what other projects use. `api/` and `api-go/` as Workers are the reference examples they are proven against.
- **`sdk/fern/apis/` has two more folders,** `petstore` and `modern`: hand-written sample specs with no server, for trying Fern.

## Starting a project of your own

- **TypeScript (oRPC):** copy the pattern, in five steps: [api.md](api.md#copying-it-into-a-typescript-orpc-project).
- **Go (workers-go):** one command makes the project: [dev.md](dev.md#a-new-project-dev-new). What you then own is in [api-go.md](api-go.md#starting-a-go-workers-go-project-from-it).
- **Both:** clients follow one rule ([realtime.md](realtime.md#what-a-client-does)), and the GitHub workflows come from the `dev` tool ([dev.md](dev.md#the-workflows-in-another-repo)).
