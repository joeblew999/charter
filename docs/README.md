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

| Page | What it covers |
|---|---|
| [Getting started](getting-started.md) | From `charter new` to a deployed API with a generated SDK |
| **[Guides](guides.md)** | One job each, in a project |
| [Make the example yours](guides/replace-the-example.md) | Which files hold the notes example, and what to do with each |
| [Change the contract](guides/contract.md) | Add an operation: inputs, validation, errors, SDK names, storage |
| [Streaming and real-time](guides/streaming.md) | An SSE stream and a WebSocket of your own resource |
| [Fern features](guides/fern-features.md) | OAuth, idempotency, uploads, webhooks, a two-way WebSocket, audiences, overlays |
| [MCP](guides/mcp.md) | Your operations as tools for agents |
| [SDKs and releasing them](guides/sdks.md) | Generate, check, add a language, publish, release |
| [Deploy](guides/deploy.md) | To Cloudflare, from your machine and from GitHub |
| [Test and CI](guides/testing.md) | What runs locally, on Cloudflare and on GitHub |
| [Measure and improve performance](guides/performance.md) | CPU per request, and the 70-second experiment |
| [The TypeScript (oRPC) version](guides/typescript.md) | The same design on oRPC |
| **[Concepts](concepts.md)** | Why it works this way |
| [Contract first](concepts/contract-first.md) | One definition, everything else derived |
| [Huma on Cloudflare Workers](concepts/workers-go.md) | What is different from a Go server, and what it costs |
| [How real-time works](realtime.md) | The five rules behind the gap-free feed |
| **[Reference](reference.md)** | Tables |
| [Tasks](reference/tasks.md) | Every `mise run` task |
| [The charter command](reference/charter.md) | Every command and flag |
| [Go packages](reference/packages.md) | The library's packages, with signatures |
| [Configuration and pins](reference/config.md) | Variables, bindings, files, versions |
| [Releases](reference/releases.md) | What a release is, and how a project updates |
| **[How to help](contributing.md)** | Set up, report, make it faster, pick up an issue |
| [Rules](rules.md) | The working rules. Binding |
| [Upstream issues](upstream.md) | Every workaround and the issue it waits for |
| [Benchmarks](benchmarks.md) | What a request costs. The only page with the numbers |
| [Findings](findings.md) | The log of verified results |
| [What is next](plans/next.md) | What is not built yet |
| [Performance plan](plans/performance.md) | What is left to make cheaper |
| [The restructure](plans/structure.md) | One project shape: what was done |
| [Writing docs](writing.md) | The rules a page is held to |

For an agent: [llms.txt](https://joeblew999.github.io/charter/llms.txt) lists every page as Markdown.
