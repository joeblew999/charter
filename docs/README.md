---
title: Home
nav_order: 1
permalink: /
---

# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

Contract-first APIs on [Cloudflare](https://developers.cloudflare.com/workers/) Workers.

- **You write the contract once:** [Huma](https://huma.rocks) operations in Go, or an [oRPC](https://orpc.dev) contract in TypeScript.
- **charter generates the rest from it:** the OpenAPI and AsyncAPI specs, typed SDKs and a CLI ([Fern](https://buildwithfern.com)), an MCP endpoint, and real-time streams over SSE and a WebSocket.
- **The Go server runs on Workers** through [workers-go](https://github.com/syumai/workers-go) and [TinyGo](https://tinygo.org), at the CPU cost of a TypeScript Worker ([Benchmarks](benchmarks.md)).
- **Everything is a task:** `mise run <task>` does the same on your machine, on GitHub and in every project.

```sh
go run github.com/joeblew999/charter/cmd/charter@latest new -name billing-api -module github.com/you/billing-api
cd billing-api && mise install && mise run setup && mise run check   # a working API, checked locally
```

Then follow [Getting started](getting-started.md).

## Who it is for

| You | Start at |
|---|---|
| Use Huma, and want it on Cloudflare Workers | [Huma on Cloudflare Workers](concepts/workers-go.md), then [Getting started](getting-started.md) |
| Use oRPC, and want SDKs, a CLI and an AsyncAPI spec from your contract | [The TypeScript (oRPC) version](guides/typescript.md) |
| Ran `charter new` and are building your API | [Guides](guides.md) |
| Want to help, or make it faster | [How to help](contributing.md) |

## What is what

| Name | Where | What it is |
|---|---|---|
| The library | `go/` | The Go module a project imports (`github.com/joeblew999/charter/go`): Huma on Workers, the request bridge, D1, the hub, the gap-free feed, MCP, AsyncAPI ([Go packages](reference/packages.md)) |
| The Worker glue | `go/worker/` | The JavaScript the library needs on Cloudflare. The build writes it into a project's `build/` |
| The tool | `cmd/charter/` | The command behind every task (`charter`): build, bench, generate, release, scaffold ([The charter command](reference/charter.md)) |
| A project | a folder with a `mise.toml` beside a `fern/` folder | One API: its contract, server, specs, SDK settings and tests. `charter new` makes one |
| The notes example | `examples/notes-go/` | A complete Go project: the notes API. `charter new` copies it |
| The notes example in TypeScript | `examples/notes-ts/` | The same API from an oRPC contract |
| The showcase | `examples/showcase-go/`, `examples/showcase-ts/` | Every Fern feature in one small API, in Go and in TypeScript ([Fern features](guides/fern-features.md)) |
| The contract | `api/contract.go` in a Go project, `src/contract.ts` in a TypeScript one | The source. Everything below comes from it |
| The specs | `fern/openapi.json`, `fern/asyncapi.json` in a project | Generated from the contract, committed |
| The hub | `go/worker/hub.mjs` | A Durable Object that wakes open streams. It stores nothing ([How real-time works](realtime.md)) |

## What is generated

Never edit these by hand. Change the source, then run the task.

| Path | What | Written by | From |
|---|---|---|---|
| `fern/openapi.json`, `fern/asyncapi.json` in a project | The specs | `mise run spec` | The contract |
| `sdk/go/` in a Go project | The Go client, committed so another repo can `go get` it. It starts with a "Generated code" banner | `mise run sdk:publish` | The specs and `fern/generators.yml` |
| `sdk/out/` in a project | Every generated SDK and the CLI. Not committed | `mise run sdk:gen` | The specs and `fern/generators.yml` |
| `build/` in a Go project | The Wasm and the Worker glue. Not committed | `mise run build` | The Go code, and `go/worker/` of the library version in `go.mod` |
| `.github/workflows/*.yml` | The GitHub workflows | `mise run workflows` | The templates in `cmd/charter/workflows/` |
| `docs/_config.yml`, `docs/writing.md`, `docs/llms.txt`, `docs/_sass/` | The docs site's config and the writing rules | `mise run docs:setup` | The templates in `cmd/charter/docs/` |
| `dist/` | Release files. Not committed | `mise run sdk:dist`, `mise run charter:release` | The generated SDKs, the tool |
| `package-lock.json`, `go.sum` | Lockfiles, committed | npm, Go | `package.json`, `go.mod` |

`.gitattributes` marks the committed ones, so GitHub folds them in a diff.

## Every page

| Section | Pages |
|---|---|
| Start | [Getting started](getting-started.md): from `charter new` to a deployed API with a generated SDK |
| [Guides](guides.md): one job each, in a project | [Make the example yours](guides/replace-the-example.md), [Change the contract](guides/contract.md), [Streaming and real-time](guides/streaming.md), [Fern features](guides/fern-features.md), [MCP](guides/mcp.md), [SDKs and releasing them](guides/sdks.md), [Deploy](guides/deploy.md), [Test and CI](guides/testing.md), [Measure and improve performance](guides/performance.md), [The TypeScript (oRPC) version](guides/typescript.md) |
| [Concepts](concepts.md): why it works this way | [Contract first](concepts/contract-first.md), [Huma on Cloudflare Workers](concepts/workers-go.md), [How real-time works](realtime.md) |
| [Reference](reference.md): tables | [Tasks](reference/tasks.md), [The charter command](reference/charter.md), [Go packages](reference/packages.md), [Configuration and pins](reference/config.md), [Releases](reference/releases.md) |
| [How to help](contributing.md): working on charter | [Rules](rules.md) (binding), [Upstream issues](upstream.md), [Benchmarks](benchmarks.md) (the only page with the numbers), [Findings](findings.md) (the log of verified results), [What is next](plans/next.md), [Performance plan](plans/performance.md), [The restructure](plans/structure.md) |
| The docs themselves | [Writing docs](writing.md): the rules a page is held to |

For an agent: [llms.txt](https://joeblew999.github.io/charter/llms.txt) lists every page as Markdown.
