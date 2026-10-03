---
title: Getting started
nav_order: 2
---

# Getting started: from `charter new` to a deployed API with an SDK

One tutorial. You end with a Go API ([Huma](https://huma.rocks)) on Cloudflare Workers, tested there, with a generated SDK. For oRPC: [In TypeScript](guides/replace-the-example.md#in-typescript).

You need [mise](https://mise.jdx.dev), Go and git (macOS, Linux or Windows: [what is covered on Windows](reference/tasks.md#windows-and-macos)), and from step 5 Docker and a Cloudflare account.

## 1. Create the project

```sh
go run github.com/joeblew999/charter/cmd/charter@latest new -name billing-api -module github.com/you/billing-api -subdomain you
cd billing-api && git init
```

`-name` is the folder, the Worker and its database; `-subdomain` your account's `workers.dev` subdomain ([every flag](reference/charter.md#make-a-project)). The project is a copy of the notes example, so every check passes before you change anything.

## 2. Install and check

```sh
mise install          # Go, Node, Rust, gh and the charter tool
mise run setup        # npm packages: cf (Cloudflare's CLI), Fern
mise run check        # what CI runs: lint, tests, spec drift, the TinyGo build, live tests natively and under workerd
```

## 3. Run it

```sh
mise run run          # natively, in memory: http://localhost:5174
curl -X POST localhost:5174/api/notes -H 'content-type: application/json' -d '{"body":"first"}'
curl -N 'localhost:5174/api/notes/watch?after=0&seconds=5'   # the SSE stream
```

`mise run dev` runs it the way Cloudflare does (the Wasm, a local D1); the first time, `mise run migrate:local` in another shell creates the tables.

## 4. Change the contract

The API is `api/contract.go`. After every change:

```sh
mise run spec         # writes fern/openapi.json and fern/asyncapi.json
mise run check        # fails if you forgot the line above
```

How, and how to take the notes out: [Your API](guides/replace-the-example.md).

## 5. Deploy

```sh
npx cf auth login     # once per machine
mise run deploy       # builds, deploys the Worker, its database and the hub, applies migrations
mise run live-test    # always after a deploy: some failures exist only on Cloudflare
```

If you gave no `-subdomain`, put the URL the deploy printed into `mise.toml` as the default of `API_URL`, then `mise run spec`. More: [Deploy and CI](guides/deploy.md).

## 6. Generate an SDK

```sh
mise run sdk:gen typescript    # Fern, in Docker: sdk/out/typescript
mise run sdk:publish           # the Go SDK into sdk/go: commit it, and another repo can go get it
```

More: [SDKs](guides/sdks.md).

## 7. Put it on GitHub

`new` already wrote the workflows. With the repo pushed:

```sh
mise run cloudflare:secrets    # once: the two Cloudflare secrets the deploy workflow needs
mise run docs:pages            # once: turns GitHub Pages on for docs/
```

Next: a stream of your own ([Streaming](guides/streaming.md)), OAuth or webhooks ([SDKs](guides/sdks.md#fern-features)), agents ([MCP](guides/replace-the-example.md#agents-mcp): it already serves `/api/mcp`), cost ([Performance](guides/performance.md)).
