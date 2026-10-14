---
title: Home
nav_order: 1
permalink: /
---

# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

**Add charter to the repo you have,** in one command: `mise x github:joeblew999/charter@X.Y.Z -- charter adopt`, with X.Y.Z the [latest release](https://github.com/joeblew999/charter/releases/latest). It keeps what is yours: what your `CLAUDE.md` or `AGENTS.md` says moves into `docs/rules.md` ([Any repo](guides/any-repo.md)). **Start a new API** with `charter new`, below, or in TypeScript with `charter-ts new` ([TypeScript](guides/typescript.md)).

Contract-first APIs on Cloudflare Workers, with [Huma](https://huma.rocks) in Go or [oRPC](https://orpc.dev) in TypeScript. Start with [Getting started](getting-started.md).

A repo with no API takes the parts that are not about one (the docs site, the repo on GitHub, the rules, the issue forms, the upstream issues): [Any repo](guides/any-repo.md).

## What you get

- **One contract** (`api/contract.go`), from which the validation, the OpenAPI and AsyncAPI specs, the MCP tools and the SDKs follow.
- **A Go server on Workers** at the CPU cost of a TypeScript one ([Benchmarks](benchmarks.md)), that also runs natively.
- **SDKs** in any language [Fern](https://buildwithfern.com) supports, and **a CLI** for every OS if you want one ([Add the CLI](guides/sdks.md#add-the-cli)).
- **SSE and WebSocket streams** that lose nothing across deploys.
- **Server-rendered pages** in the same Worker, with gsx and htmx 4 or Datastar, live over those streams ([Server-rendered pages](guides/pages.md)).
- **Every step is a task:** `mise run <task>`, the same locally and on GitHub.

## What is where

Two folders: everything TypeScript is in `ts/`, everything Go in `go/`.

**The TypeScript side is npm workspaces:** `cd ts && npm ci && npm run check` is every check of charter-ts and the projects made with it. mise only installs node and fnox (`ts/mise.toml`). A project's steps are its npm scripts, each `charter-ts <step>`, and its `charter.config.ts` says what they need ([TypeScript](guides/typescript.md)).

**The Go side is moved out of the way:** the tool, the Go library, the Go projects and the mise task files are in `go/` as they were. The tool builds there; its tasks and the pages that name the old paths are not updated yet.

| Name | Where | What it is |
|---|---|---|
| charter-ts | `ts/packages/charter-ts/` | The package a TypeScript project installs from its GitHub Release: the `charter-ts` command (`new`, `dev`, `lint`, `spec`, `test:workerd`, `deploy`, `live-test`, ...) and the library: `specs`, `asyncapi`, `auth`, `ratelimit`, `follow`, `config`, `scripts` |
| The TypeScript projects | `ts/examples/start/` (one route), `ts/examples/notes/` (the notes API: auth, streams, rate limits), `ts/examples/showcase/` (every Fern feature, the SDK tested inside a Worker) | Each: the contract (`src/contract.ts`), `charter.config.ts`, `test/` (typed, checked against the contract's schemas), `fern/`. `charter-ts new` copies start or notes |
| The Go library | `go/lib/` | What a Go project imports ([Go packages](reference/packages.md)), with its Worker glue in `go/lib/worker/` |
| The tool | `go/cmd/charter/` | The command behind every mise task ([The charter command](reference/charter.md)) |
| The mise task files | `go/tasks/repo/`, `go/tasks/project/` | The tasks any repo takes, and those of a Go project |
| The Go projects | `go/notes/`, `go/start/`, `go/start-htmx/`, `go/start-datastar/`, `go/showcase/` | The notes API, the start projects (with server-rendered pages: [Server-rendered pages](guides/pages.md)) and the conformance project, in Go |

## What is generated

Never edit these: change the source and run the task.

| Path | Written by |
|---|---|
| `fern/openapi.json`, `fern/asyncapi.json` | `npm run spec` (TypeScript), `mise run spec` (Go), from the contract |
| `pages/*.x.go` (committed) | `mise run ui:gen`, from the gsx components (`go/start-htmx/`, `go/start-datastar/`) |
| `sdk/go/` (committed), `sdk/out/` | `mise run sdk:publish`, `mise run sdk:gen`; in TypeScript, `npm run sdk -- <group>` |
| `build/`, `dist/`, `ts/packages/charter-ts/dist/` | `mise run build`, `mise run sdk:dist`, `npm ci` or `npm run build -w charter-ts` in `ts/` |
| `.github/` (but `workflows/check.yml`, `deploy.yml`, `release.yml`, `sdk-check.yml`, this repo's own), `renovate.json`, `AGENTS.md`, `CLAUDE.md` in a repo with no project at its root | `mise run adopt`, from `go/cmd/charter/kit/` |
| `docs/charter/`, `docs/_config.yml`, `docs/llms.txt`, `docs/_sass/` | `mise run adopt`, from `go/cmd/charter/kit/` |
| `docs/charter/tasks.md`, `docs/charter/using.md` | `mise run adopt`, from mise and from the tool |
| A page that `_generated.toml` in `docs/` lists (this repo has none) | `mise run adopt`, from its command ([Generated pages](guides/deploy.md#generated-pages)) |

## Every page

| Section | Pages |
|---|---|
| Start | [Getting started](getting-started.md) |
| [Guides](guides.md) | [Your API](guides/replace-the-example.md) (the contract, MCP, TypeScript), [Streaming](guides/streaming.md), [SDKs](guides/sdks.md) (Fern features, languages, releases), [Deploy and CI](guides/deploy.md), [Auth](guides/auth.md) (tokens, Cloudflare Access, OpenID Connect), [Release](guides/release.md) (from your machine, the CLI for every OS), [Performance](guides/performance.md), [Repos that use each other](guides/repos.md) (who pins what, breaking changes, Renovate), [Server-rendered pages](guides/pages.md) (gsx, htmx 4, Datastar), [TypeScript](guides/typescript.md) (charter-ts: start a project, the steps, the config, the library), [Any repo](guides/any-repo.md) (what a repo with no API takes: `charter adopt`, `tasks/repo/`) |
| [Reference](reference.md) | [Tasks](reference/tasks.md), [The charter command](reference/charter.md), [Go packages](reference/packages.md) |
| [How to help](contributing.md) | [Rules](rules.md), [Benchmarks](benchmarks.md), [Upstream issues](upstream.md), [Writing docs](charter/writing.md) |
| [How this repo is kept](charter/README.md) | What is the same in every repo charter keeps: [the rules they share](charter/rules.md) |

For an agent: [llms.txt](https://joeblew999.github.io/charter/llms.txt) lists every page as Markdown.
