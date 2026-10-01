# orpc-api

[![api-check](https://github.com/joeblew999/orpc-api/actions/workflows/api-check.yml/badge.svg)](https://github.com/joeblew999/orpc-api/actions/workflows/api-check.yml)
[![sdk-check](https://github.com/joeblew999/orpc-api/actions/workflows/sdk-check.yml/badge.svg)](https://github.com/joeblew999/orpc-api/actions/workflows/sdk-check.yml)
[![dev-check](https://github.com/joeblew999/orpc-api/actions/workflows/dev-check.yml/badge.svg)](https://github.com/joeblew999/orpc-api/actions/workflows/dev-check.yml)

One notes API on Cloudflare Workers, built twice: in TypeScript with oRPC (`api/`) and in Go with Huma on workers-go (`api-go/`). Both are contract first, both have real-time (SSE + WebSockets) that doesn't lose data, and both hand Fern an OpenAPI and an AsyncAPI file, from which Fern generates typed SDKs, a CLI and a docs site for other developers. It's the base for our other API projects: copy the patterns, not the notes.

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
| The live hub (a Durable Object) | `api/src/hub.ts` (oRPC's) | `api-go/worker/hub.mjs` |
| The Worker on Cloudflare | `orpc-api` | `orpc-api-go` |
| Also runs without Cloudflare | no | yes: `mise run api-go:run` |
| Check it locally | `mise run api:check` | `mise run api-go:check` |
| Test it deployed | `mise run api:live-test`, `api:soak` | `mise run api-go:live-test`, `api-go:soak` |
| Shared by both | `test/` (the test programs), `migrations/` (the D1 schema), `sdk/` (Fern) | |

Things that are easy to mix up:

- **Fern does not write servers.** It reads the two spec files and writes clients (SDKs in Go, TypeScript and more, a CLI, docs). The server is ours, in either language.
- **"Go" means two different things here.** `api-go/` is a Go *server*. `sdk/out/*/go` is a Go *client SDK* that Fern generated, and it exists for both servers.
- **The tests and the database schema are shared.** `test/` has one set of test programs for both servers (they take a URL, and `--sdk api-go` picks the SDKs generated from the Go specs), and `migrations/` is the one D1 schema.
- **`sdk/fern/apis/` has other folders** (`petstore`, `showcase`, `modern`): sample specs for trying Fern features. They have no server here except the showcase mock in `sdk/harness`.

## Quick start

Needs [mise](https://mise.jdx.dev) and Docker (for Fern).

```sh
mise install              # node, jq, gh, TinyGo (mise.toml), Rust (rust-toolchain.toml), Go (go.work)
mise run setup            # npm packages
mise run check            # every local check: both APIs, spec drift, SDKs, the SDK inside workerd
mise run api:dev          # the oRPC API on http://localhost:5173 (first time, in another shell: mise run api:migrate:local)
mise run api-go:dev       # the Go API on http://localhost:5174 (first time: mise run api-go:migrate:local)
mise run api-go:run       # the Go API natively, no Cloudflare (in-memory store)
mise tasks                # everything else
```

Deploying needs a Cloudflare login (`api/node_modules/.bin/cf auth login`, or `CLOUDFLARE_API_TOKEN`):

```sh
mise run api:deploy       # the orpc-api Worker, then its D1 migrations
mise run api:live-test    # SSE + WebSocket + the TypeScript SDK against it
mise run api:soak         # the real-time matrix (redeploys once mid-run)
mise run api-go:deploy    # the same three for the Go Worker (orpc-api-go)
mise run api-go:live-test
mise run api-go:soak
```

On another Cloudflare account, set `API_URL`, `API_GO_URL` (and `HARNESS_URL`, `HARNESS_API_URL`) in `mise.local.toml`, which is gitignored.

## What it shows

- **One contract gives everything.** Plain REST, SSE (`notes.watch`) and a WebSocket (`notes.live`) on a Worker with D1; an OpenAPI and an AsyncAPI spec, both generated; and from those, Fern's SDKs (Go, TypeScript), Rust CLI and docs.
- **Real-time that holds up on Cloudflare.** One primitive, `follow()`, gives every client a gap-free feed across deploys, Durable Object restarts, disconnects and long idle periods. The hub hibernates. It's tested by a client × scenario matrix (`mise run api:soak`, `mise run api-go:soak`).
- **Go on Workers with the same tooling.** Huma runs under TinyGo on workers-go with three small workarounds (`api-go/humaworkers`), and Fern can't tell the Go server's specs from the oRPC ones.
- **Upstream gaps are tracked.** Every workaround names its Fern or oRPC issue, and `mise run upstream:status` shows which are fixed.

## Layout

| Path | What |
|---|---|
| [api/](api/README.md) | The oRPC Worker (TypeScript): contract, `follow()`, the AsyncAPI generator, the upstream-issues table |
| [api-go/](api-go/README.md) | The Go Worker: the Huma contract, handlers, and the packages other Go projects import (`humaworkers`, `asyncapi`, `follow`) |
| [test/](test/README.md) | The tests both servers must pass: the quick live test and the real-time soak matrix |
| `migrations/` | The D1 schema, for both Workers |
| [sdk/](sdk/README.md) | Fern: `sdk/fern/apis/<api>/` (specs plus `generators.yml`), the Workers SDK harness, CLI builds, and the real-time client rules |
| [FINDINGS.md](FINDINGS.md) | Verified results only |
| [.plans/](.plans/) | [next.md](.plans/next.md), [realtime.md](.plans/realtime.md) (the design), [asyncapi.md](.plans/asyncapi.md) |
| [dev/](dev/README.md) | The tool the tasks run, and the part other repos reuse: `go run ./dev help` |
| `.github/workflows/` | CI, deploys and releases (`api-*`, `sdk-*`, `dev-*`), written from templates by `mise run dev:workflows`. What each does, how to cut a release and how another repo gets them: [dev/README.md](dev/README.md#github-workflows) |
| `mise.toml` | Every task, one line each. Anything longer is a `dev` command |

## Using it in another project

**A TypeScript (oRPC) project:**

1. Write the contract with `openapi({...})` and `asyncapi({...})` metadata, as in `api/src/contract.ts`.
2. Copy `api/src/follow.ts`, `api/src/asyncapi.ts` and `api/src/specs.ts` unchanged, and give `follow()` your own source (`subscribe`, `since`, `latest`).
3. Copy `sdk/fern/apis/api/` as your API's Fern folder, plus the `api:*` and `sdk:*` tasks.

**A Go (workers-go) project:**

1. Write the contract as Huma operations, as in `api-go/api/contract.go`: one `humaworkers.Route` per operation, with `asyncapi.Operation(...)` around the WebSocket ones.
2. Import the packages `humaworkers`, `asyncapi` and `follow` (`go get github.com/joeblew999/orpc-api/api-go`), and copy `api-go/worker/` (the entry and the hub), `api-go/cmd/spec` and the two `platform_*.go` files.
3. Copy `sdk/fern/apis/api-go/` as your API's Fern folder, plus the `api-go:*` and `sdk:*` tasks.

**Both:** follow the client rule from [sdk/README.md](sdk/README.md#real-time-on-cloudflare-the-pattern-to-copy-plansrealtimemd): call again with `after` = the last id.
