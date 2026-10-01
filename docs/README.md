# Docs

Everything written about this repo lives in this folder: one place for developers and for agents. Folder READMEs and `AGENTS.md` only point here.

| Page | What it covers |
|---|---|
| **This page** | What is what, and how to use the patterns in another project |
| [rules.md](rules.md) | The working rules. Read them before changing anything |
| [api.md](api.md) | `api/`: the oRPC Worker (TypeScript) |
| [api-go.md](api-go.md) | `api-go/`: the Go Worker (Huma on workers-go), and what Huma needs to run there |
| [sdk.md](sdk.md) | `sdk/`: Fern, the generated SDKs and CLI, and the real-time client rules |
| [testing.md](testing.md) | `test/`: the tests both servers must pass |
| [dev.md](dev.md) | `dev/`: the tool behind the tasks, the GitHub workflows, releases |
| [benchmarks.md](benchmarks.md) | Measured cost per request of both Workers, and how to measure it |
| [upstream.md](upstream.md) | Every upstream issue we work around, and drafts for ones not filed yet |
| [findings.md](findings.md) | Verified results only, newest last |
| [plans/](plans/next.md) | What comes next: [next.md](plans/next.md), [realtime.md](plans/realtime.md) (the design), [asyncapi.md](plans/asyncapi.md), [performance.md](plans/performance.md), [microservices.md](plans/microservices.md), [mcp.md](plans/mcp.md) (the contract as MCP tools) |

## What is what

The one idea: **you write a contract in code; everything else is generated from it.**

```
contract (code you write)  ->  openapi.json + asyncapi.json (generated)  ->  Fern  ->  SDKs, CLI, docs
        |
        +-> the server validates requests against the same contract
```

There are two contracts in this repo because there are two servers. They describe the same API, and a test keeps them the same. A real project has one: pick the column for its language.

| | TypeScript | Go |
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
| The gap-free feed | `api/src/follow.ts` | `api-go/follow/` |
| The live hub (a Durable Object with hibernating WebSockets) | `api/src/hub.ts` (oRPC's) | `api-go/worker/hub.mjs` |
| The Worker on Cloudflare | `orpc-api` | `orpc-api-go` |
| Also runs without Cloudflare | no | yes: `mise run api-go:run` |
| Check it locally | `mise run api:check` | `mise run api-go:check` |
| Test it deployed | `mise run api:live-test`, `api:soak` | `mise run api-go:live-test`, `api-go:soak` |
| Shared by both | `test/` (the test programs), `migrations/` (the D1 schema), `sdk/` (Fern), `dev/` (the tasks' tool) | |

Things that are easy to mix up:

- **Fern does not write servers.** It reads the two spec files and writes clients: SDKs in Go, TypeScript and more, a command-line program, docs. The server is ours, in either language.
- **"The CLI" means the command-line program Fern generates** (`orpc-api notes list`, `orpc-api notes watch`). It runs on a developer's machine and calls the API over HTTPS, like an SDK. It never runs on Cloudflare. It is not `cf` (Cloudflare's tool), `mise`, or `dev` (this repo's task tool).
- **"Go" means two different things here.** `api-go/` is a Go *server*. `sdk/out/*/go` is a Go *client SDK* that Fern generated, and it exists for both servers.
- **The tests and the database schema are shared.** `test/` has one set of test programs for both servers (they take a URL, and `--sdk api-go` picks the SDKs generated from the Go specs), and `migrations/` is the one D1 schema.
- **What is the product and what is an example.** `dev/` and the Go packages in `api-go/` (`humaworkers`, `asyncapi`, `follow`, `humamcp`) are what other projects use. `api/` and `api-go/` as Workers are the reference examples they are proven against.
- **`sdk/fern/apis/` has other folders** (`petstore`, `showcase`, `modern`): sample specs for trying Fern features. They have no server here except the showcase mock in `sdk/harness`.

## What it shows

- **One contract gives everything.** Plain REST, SSE (`notes.watch`) and a WebSocket (`notes.live`) on a Worker with D1; an OpenAPI and an AsyncAPI spec, both generated; and from those, Fern's SDKs (Go, TypeScript), Rust CLI and docs.
- **Real-time that holds up on Cloudflare.** One primitive, `follow()`, gives every client a gap-free feed across deploys, Durable Object restarts, disconnects and long idle periods. The hub hibernates. It's tested by a client × scenario matrix on both Workers ([testing.md](testing.md)).
- **Go on Workers with the same tooling.** Huma runs under TinyGo on workers-go with a handful of workarounds ([api-go.md](api-go.md)), and the SDKs Fern generates from either server's specs work against the other.
- **Upstream gaps are tracked.** Every workaround names its issue ([upstream.md](upstream.md)), and `mise run upstream:status` shows which are fixed.

## Using it in another project

**A TypeScript (oRPC) project:**

1. Write the contract with `openapi({...})` and `asyncapi({...})` metadata, as in `api/src/contract.ts`.
2. Copy `api/src/follow.ts`, `api/src/asyncapi.ts` and `api/src/specs.ts` unchanged, and give `follow()` your own source (`subscribe`, `since`, `latest`).
3. Copy `sdk/fern/apis/api/` as your API's Fern folder, plus the `api:*` and `sdk:*` tasks.

**A Go (workers-go) project:**

1. Write the contract as Huma operations, as in `api-go/api/contract.go`: one `humaworkers.Route` per operation, with `asyncapi.Operation(...)` around the WebSocket ones.
2. Import the packages `humaworkers`, `asyncapi` and `follow` (`go get github.com/joeblew999/orpc-api/api-go`), and copy `api-go/worker/` (the entry, the hub and the clock fix), `api-go/cmd/spec` and the two `platform_*.go` files.
3. Copy `sdk/fern/apis/api-go/` as your API's Fern folder, plus the `api-go:*` and `sdk:*` tasks.

**Both:**

- Follow the client rule from [sdk.md](sdk.md#real-time-on-cloudflare-the-pattern-to-copy): call again with `after` = the last id.
- Get the GitHub workflows with `go run github.com/joeblew999/orpc-api/dev@latest workflows -into .` ([dev.md](dev.md#github-workflows)).
