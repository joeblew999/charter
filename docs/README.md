---
title: Home
nav_order: 1
permalink: /
---

# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

Contract-first APIs on Cloudflare Workers, with [Huma](https://huma.rocks) in Go or [oRPC](https://orpc.dev) in TypeScript. Start with [Getting started](getting-started.md).

## What you get

- **One contract** (`api/contract.go`), from which the validation, the OpenAPI and AsyncAPI specs, the MCP tools and the SDKs follow.
- **A Go server on Workers** at the CPU cost of a TypeScript one ([Benchmarks](benchmarks.md)), that also runs natively.
- **SDKs and a CLI** in any language [Fern](https://buildwithfern.com) supports.
- **SSE and WebSocket streams** that lose nothing across deploys.
- **Every step is a task:** `mise run <task>`, the same locally and on GitHub.

## What is where

| Name | Where | What it is |
|---|---|---|
| A project | a folder with `mise.toml` beside `fern/` | One API. `charter new` makes one |
| The contract | `api/contract.go` (Go), `src/contract.ts` (TypeScript) | The source |
| The library | `go/` | What a Go project imports ([Go packages](reference/packages.md)) |
| The TypeScript library | `ts/` | `@charter/ts`: the AsyncAPI generator for oRPC, both specs from one contract, `follow()`, bearer tokens with scopes (`auth`). The TypeScript examples take it as a local package; elsewhere it is installed from a release |
| The Worker glue | `go/worker/` | The JavaScript the library needs; the build copies it into `build/` |
| The shared tasks | `tasks/shared/`, `tasks/go/`, `tasks/ts/` | The tasks every project includes, at the release it pins; its `mise.toml` holds only its own |
| The tool | `cmd/charter/` | The command behind every task ([The charter command](reference/charter.md)) |
| The examples | `examples/notes-go/` (what `charter new` copies), `examples/notes-ts/` | The notes API, in Go and in TypeScript |
| The start projects | `examples/start-go/`, `examples/start-ts/` (what `charter new -empty` copies) | One route, `GET /api/hello`, with the same tasks, specs, SDKs, tests and workflows |
| The conformance projects | `conformance/showcase-go/`, `conformance/showcase-ts/` | Every Fern feature end to end, in Go and in TypeScript: the proof that the generated SDKs work |

## What is generated

Never edit these: change the source and run the task.

| Path | Written by |
|---|---|
| `fern/openapi.json`, `fern/asyncapi.json` | `mise run spec`, from the contract |
| `sdk/go/` (committed), `sdk/out/` | `mise run sdk:publish`, `mise run sdk:gen` |
| `build/`, `dist/`, `ts/dist/` | `mise run build`, `mise run sdk:dist`, `mise run ts:build` |
| `.github/` | `mise run workflows` |
| `renovate.json` | `mise run repo` |
| `docs/_config.yml`, `docs/writing.md`, `docs/llms.txt`, `docs/_sass/` | `mise run docs:setup` |
| A page that `_generated.toml` in `docs/` lists (this repo has none) | `mise run docs:setup`, from its command ([Generated pages](guides/deploy.md#generated-pages)) |

## Every page

| Section | Pages |
|---|---|
| Start | [Getting started](getting-started.md) |
| [Guides](guides.md) | [Your API](guides/replace-the-example.md) (the contract, MCP, TypeScript), [Streaming](guides/streaming.md), [SDKs](guides/sdks.md) (Fern features, languages, releases), [Deploy and CI](guides/deploy.md), [Performance](guides/performance.md), [Repos that use each other](guides/repos.md) (who pins what, breaking changes, Renovate) |
| [Reference](reference.md) | [Tasks](reference/tasks.md), [The charter command](reference/charter.md), [Go packages](reference/packages.md) |
| [How to help](contributing.md) | [Rules](rules.md), [Benchmarks](benchmarks.md), [Upstream issues](upstream.md), [Writing docs](writing.md) |

For an agent: [llms.txt](https://joeblew999.github.io/charter/llms.txt) lists every page as Markdown.
