---
title: Getting started
nav_order: 2
---

# Getting started: from `charter new` to a deployed API with an SDK

One tutorial. You end with a Go API ([Huma](https://huma.rocks)) on Cloudflare Workers, tested there, with a generated SDK. For TypeScript, read [The TypeScript (oRPC) version](guides/typescript.md) instead.

You need:

- **[mise](https://mise.jdx.dev), Go and git.** mise installs the rest at pinned versions.
- **Docker,** from step 5 on: Fern generates SDKs in containers.
- **A Cloudflare account,** from step 5 on. What a request costs there: [Benchmarks](benchmarks.md).

## 1. Create the project

```sh
go run github.com/joeblew999/charter/cmd/charter@latest new -name billing-api -module github.com/you/billing-api -subdomain you
cd billing-api && git init
```

`-name` is the folder, the Worker and its database. `-module` is the Go module path. `-subdomain` is your account's `workers.dev` subdomain: without it the Worker's URL is a placeholder, fixed in step 5 ([every flag](reference/charter.md#make-a-project)).

The project is a copy of the notes example (`examples/notes-go/` in charter), so every check passes before you change anything. The first line `new` prints is the release it pinned: the tool in `mise.toml`, the library in `go.mod`.

## 2. Install and check

```sh
mise install          # Go, Node, Rust, gh and the charter tool
mise run setup        # npm packages: cf (Cloudflare's CLI), Fern
mise run check        # lint, tests, spec drift, the TinyGo build, live tests natively and under workerd
```

`check` is what CI runs. The first build also installs TinyGo, which the tool pins.

## 3. Run it

```sh
mise run run          # natively, in memory: http://localhost:5174
```

In another shell:

```sh
curl localhost:5174/api/hello
curl -X POST localhost:5174/api/notes -H 'content-type: application/json' -d '{"body":"first"}'
curl 'localhost:5174/api/notes?limit=10'
curl -N 'localhost:5174/api/notes/watch?after=0&seconds=5'   # the SSE stream: the note, then the end marker
curl localhost:5174/api/openapi.json                         # the spec, from your contract
```

The way Cloudflare runs it (the Wasm, a local D1 database, the hub):

```sh
mise run dev              # in one shell: the same port
mise run migrate:local    # in another, the first time: creates the tables
```

## 4. Change the contract

The API is `api/contract.go`: one Huma operation per route, with Go structs for input and output. The struct tags are the validation and the schema.

```sh
mise run spec         # after every contract change: writes fern/openapi.json and fern/asyncapi.json
mise run check        # fails if you forgot the line above
```

How: [Change the contract](guides/contract.md). To remove the notes: [Make the example yours](guides/replace-the-example.md).

## 5. Deploy

```sh
./node_modules/.bin/cf auth login    # once per machine
mise run deploy                      # builds, deploys the Worker, its database and the hub, applies migrations
mise run live-test                   # SSE, WebSocket, the generated SDK and MCP, against what you deployed
```

- **If you gave no `-subdomain`:** put the URL the deploy printed into `mise.toml` as the default of `API_URL`, then `mise run spec`. The specs name the server.
- **Always run the live test after a deploy.** Some failures only exist on Cloudflare.

More: [Deploy](guides/deploy.md).

## 6. Generate an SDK

```sh
mise run sdk:gen typescript    # Fern, in Docker: sdk/out/typescript
mise run sdk:gen go            # sdk/out/go
mise run sdk:publish           # the Go SDK, checked and copied into sdk/go: commit it, and another repo can go get it
```

Other languages, the CLI and releases: [SDKs and releasing them](guides/sdks.md).

## 7. Put it on GitHub

`new` already wrote the workflows into `.github/workflows/`. With the repo pushed:

```sh
mise run cloudflare:secrets    # once: the two Cloudflare secrets the deploy workflow needs, from fnox
mise run docs:setup            # the docs site's config for docs/, and llms.txt for agents
mise run docs:pages            # once: turns GitHub Pages on
```

More: [Test and CI](guides/testing.md).

## Where next

| You want | Read |
|---|---|
| A stream of your own resource | [Streaming and real-time](guides/streaming.md) |
| OAuth, idempotency, uploads, webhooks | [Fern features](guides/fern-features.md) |
| Agents calling your API | [MCP](guides/mcp.md): it already serves `/api/mcp` |
| To know what differs from a Go server | [Huma on Cloudflare Workers](concepts/workers-go.md) |
