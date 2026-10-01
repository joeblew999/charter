---
title: Getting started
nav_order: 2
---

# Getting started: from nothing to a deployed API with an SDK

About fifteen minutes. You end with a Go API running on Cloudflare Workers, tested there, with a generated TypeScript SDK. Every command below was run as written.

You need [mise](https://mise.jdx.dev) and Go. For step 5 you need a Cloudflare account on the Workers Paid plan: the Go Worker uses 40 to 70 ms of CPU per request, over the Free plan's 10 ms ([what it costs](README.md#before-you-choose-go-what-it-costs-to-run)). For step 6 you need Docker.

## 1. Create the project

```sh
dev() { go run github.com/joeblew999/orpc-api/dev@latest "$@"; }   # the tool, straight from GitHub
dev new -name billing-api                                            # creates ./billing-api
cd billing-api && git init
```

`-name` becomes the Worker's name, its database and the SDK's names (`BillingApiClient`). The Go module defaults to `github.com/<your GitHub login>/billing-api`; pass `-module` to choose.

The project starts as a small notes API, so everything works before you change anything.

## 2. Install and check

```sh
mise install          # Go, TinyGo, Node and the dev tool, at pinned versions
mise run setup        # npm packages
mise run check        # lint, tests, spec drift, the Wasm build, and live tests natively and under workerd
```

`check` takes about half a minute and must pass. It is the same command CI runs.

## 3. Run it

```sh
mise run api-go:run   # natively, in-memory store: http://localhost:5174
```

In another shell:

```sh
curl localhost:5174/api/hello
curl -X POST localhost:5174/api/notes -H 'content-type: application/json' -d '{"body":"first"}'
curl 'localhost:5174/api/notes?limit=10'
curl -N 'localhost:5174/api/notes/watch?after=0&seconds=5'      # the SSE stream: the note, then the end marker
curl localhost:5174/api/openapi.json                             # the spec, generated from your contract
```

To run it the way Cloudflare does (the real Wasm, a local D1 database and the hub):

```sh
mise run api-go:dev             # in one shell
mise run api-go:migrate:local   # in another, the first time: creates the tables
```

## 4. Make it yours

The whole API is one file: `api-go/api/contract.go`. Each operation is a few lines: its path, its input struct, its output struct. Struct tags are the validation rules and the schema.

Change it, then:

```sh
mise run api-go:spec    # regenerates the OpenAPI and AsyncAPI files from the contract
mise run check          # fails if you forgot the line above, or broke something
```

How to add operations, validation and errors: [Define your API](guides/contract.md).

## 5. Deploy

```sh
api-go/node_modules/.bin/cf auth login   # once: your Cloudflare account
mise run api-go:deploy                   # the Worker, its D1 database, the migrations
mise run api-go:live-test                # SSE, WebSocket, the SDK and MCP against what you just deployed
```

The deploy prints the Worker's URL. If it is not `https://billing-api.gedw99.workers.dev` (another account), put yours in `mise.local.toml`:

```toml
[env]
API_GO_URL = "https://billing-api.<your-subdomain>.workers.dev"
```

Then `mise run api-go:spec`, so the specs name the right server. Always run the live test after a deploy: some failures only exist on Cloudflare itself. More: [Deploy to Cloudflare](guides/deploy.md).

## 6. Generate an SDK

```sh
mise run sdk:gen api-go typescript   # Fern, in Docker -> sdk/out/api-go/typescript
mise run sdk:gen api-go go           # the same for Go; also: cli
```

That is a typed client for your API, with pagination, the SSE stream and the WebSocket built in. What to do with it: [Generate SDKs and a CLI](guides/sdks.md).

## 7. Put it on GitHub

With the repo pushed to GitHub:

```sh
mise run dev:workflows   # GitHub workflows that run the same checks on every push
mise run docs:setup      # a docs site for docs/, and llms.txt for agents
mise run docs:pages      # once: turns the site on
```

More: [CI and releases](guides/ci-releases.md), [A docs site](guides/docs-site.md).

## Where next

- [Real-time: SSE and WebSocket](guides/streaming.md): how the stream you saw in step 3 stays gap-free.
- [Auth, idempotency, uploads, webhooks](guides/fern-features.md).
- [Expose the API to AI agents](guides/mcp.md): your API is already an MCP server at `/api/mcp`.
- [Go on Cloudflare Workers](concepts/workers-go.md): what is different from a normal Go server, including what it costs.
