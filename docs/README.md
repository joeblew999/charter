---
title: Home
nav_order: 1
permalink: /
---

# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

**Add charter to the repo you have,** in one command: `npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.4.0/charter-ts-0.4.0.tgz charter-ts adopt`. It keeps what is yours: what your `CLAUDE.md` or `AGENTS.md` says moves into `docs/rules.md` ([Any repo](guides/any-repo.md)). **Start a new API** with `charter new`, below, or in TypeScript with `charter-ts new` ([TypeScript](guides/typescript.md)).

Contract-first APIs on Cloudflare Workers, with [Huma](https://huma.rocks) in Go or [oRPC](https://orpc.dev) in TypeScript. Start with [Getting started](getting-started.md).

A repo with no API takes the parts that are not about one (the docs site, the repo on GitHub, the rules, the issue forms, the upstream issues): [Any repo](guides/any-repo.md).

## What you get

- **One contract** (`api/contract.go`), from which the validation, the OpenAPI and AsyncAPI specs, the MCP tools and the SDKs follow.
- **A Go server on Workers** at the CPU cost of a TypeScript one ([Benchmarks](benchmarks.md)), that also runs natively.
- **SDKs** in any language [Fern](https://buildwithfern.com) supports, and **a CLI** for every OS if you want one ([Add the CLI](guides/sdks.md#add-the-cli)).
- **SSE and WebSocket streams** that lose nothing across deploys.
- **Server-rendered pages** in the same Worker, with gsx and htmx 4 or Datastar, live over those streams ([Server-rendered pages](guides/pages.md)).
- **Every step is an npm script:** `npm run <script>`, the same locally and on GitHub.

## What is where

`charter-ts/` is the tool, at the top: it keeps this repo (and any repo that adopts it) and runs a TypeScript project's steps. `ts/` holds the TypeScript projects, `go/` the Go side. The root `package.json` is the npm workspace of `charter-ts/` and `ts/`, and its scripts are this repo's steps: `npm ci`, then `npm run check` is every local check ([Tasks](charter/tasks.md) lists them in order). mise only installs node and fnox (`mise.toml`).

| Name | Where | What it is |
|---|---|---|
| charter-ts | `charter-ts/` | The `charter-ts` command and its library, installed from its GitHub Release. For any repo: `adopt` (the kit, `charter-ts/kit/`), `docs:lint`, `issue`, `upstream:status`, `repo`. For a project: `new`, `dev`, `lint`, `spec`, `test:workerd`, `deploy`, `live-test` ([TypeScript](guides/typescript.md)) |
| The TypeScript projects | `ts/start/` (one route), `ts/notes/` (the notes API: auth, streams, rate limits), `ts/showcase/` (every Fern feature, the SDK tested inside a Worker) | Each: the contract (`src/contract.ts`), `charter.config.ts`, `test/` (typed, checked against the contract's schemas), `fern/`. `charter-ts new` copies start or notes |
| The Go library | `go/lib/` | What a Go project imports ([Go packages](reference/packages.md)), with its Worker glue in `go/lib/worker/` |
| The Go projects | `go/notes/`, `go/start/`, `go/start-htmx/`, `go/start-datastar/`, `go/showcase/` | The notes API, the start projects (with server-rendered pages: [Server-rendered pages](guides/pages.md)) and the conformance project, in Go |

## What is generated

Never edit these: change the source and run the step.

| Path | Written by |
|---|---|
| `fern/openapi.json`, `fern/asyncapi.json` | `npm run spec` (TypeScript), `npm run spec` (Go), from the contract |
| `pages/*.x.go` (committed) | `go tool gsx generate`, from the gsx components (`go/start-htmx/`, `go/start-datastar/`); `npm run check` fails while they are stale |
| `sdk/go/` (committed), `sdk/out/` | `npm run sdk:publish`, `npm run sdk --`; in TypeScript, `npm run sdk -- <group>` |
| `build/`, `charter-ts/dist/` | `npx charter-ts build` in a Go project (`dev`, `check` and `deploy` run it); `npm ci` or `npm run build -w charter-ts` |
| `.github/` (but `workflows/check.yml` and `deploy.yml`, this repo's own), `renovate.json`, `AGENTS.md`, `CLAUDE.md` | `npm run adopt`, from `charter-ts/kit/` |
| `docs/charter/`, `docs/_config.yml`, `docs/llms.txt`, `docs/_sass/` | `npm run adopt`, from `charter-ts/kit/` |
| `docs/charter/tasks.md`, `docs/charter/using.md` | `npm run adopt`, from `package.json`'s scripts and from charter-ts's commands |

## Every page

| Section | Pages |
|---|---|
| Start | [Getting started](getting-started.md) |
| [Guides](guides.md) | [Your API](guides/replace-the-example.md) (the contract, MCP, TypeScript), [Streaming](guides/streaming.md), [SDKs](guides/sdks.md) (Fern features, languages, the Go SDK for other repos), [Deploy and CI](guides/deploy.md), [Auth](guides/auth.md) (tokens, Cloudflare Access, OpenID Connect), [Repos that use each other](guides/repos.md) (who pins what, breaking changes, Renovate), [Server-rendered pages](guides/pages.md) (gsx, htmx 4, Datastar), [TypeScript](guides/typescript.md) (charter-ts: start a project, the steps, the config, the library), [Any repo](guides/any-repo.md) (what a repo with no API takes: `charter-ts adopt`) |
| [Reference](reference.md) | [Go packages](reference/packages.md); every charter-ts command: [Using charter-ts](charter/using.md); every script of this repo: [Tasks](charter/tasks.md) |
| [How to help](contributing.md) | [Rules](rules.md), [Benchmarks](benchmarks.md), [Upstream issues](upstream.md), [Writing docs](charter/writing.md) |
| [How this repo is kept](charter/README.md) | What is the same in every repo charter keeps: [the rules they share](charter/rules.md) |

For an agent: [llms.txt](https://joeblew999.github.io/charter/llms.txt) lists every page as Markdown.
