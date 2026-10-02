# charter

[![check](https://github.com/joeblew999/charter/actions/workflows/check.yml/badge.svg)](https://github.com/joeblew999/charter/actions/workflows/check.yml)
[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

**Contract-first APIs on Cloudflare Workers, with [Huma](https://huma.rocks) in Go or [oRPC](https://orpc.dev) in TypeScript.** Write the contract once and get the server, the OpenAPI and AsyncAPI specs, typed SDKs in any language ([Fern](https://buildwithfern.com)), a CLI, an MCP endpoint and real-time streams: generated from that contract, and tested.

If you use Huma, this is how it runs on Workers ([workers-go](https://github.com/syumai/workers-go) and [TinyGo](https://tinygo.org)) at the cost of a TypeScript Worker. If you use oRPC, this is your contract turned into SDKs, a CLI and an AsyncAPI spec that oRPC does not generate itself.

```sh
go run github.com/joeblew999/charter/cmd/charter@latest new -name billing-api -module github.com/you/billing-api
cd billing-api && mise install && mise run setup && mise run check   # a working API, checked locally
mise run deploy                                                       # on Cloudflare
```

Needs [mise](https://mise.jdx.dev), Go and git. Then: [Getting started](docs/getting-started.md).

## What you get

- **The contract is the source.** Huma operations in Go (structs and tags), or an oRPC contract in TypeScript (Zod). Change it, run `mise run spec`, and everything downstream follows. A stale spec fails the check.
- **Huma on Cloudflare Workers at TypeScript's cost.** Under 1 ms of CPU for a simple read, 1 to 2 ms with a database read, about 2 ms for a write: the same as the TypeScript Worker, measured side by side ([benchmarks](docs/benchmarks.md)). TinyGo and workers-go as they come cost 55 to 265 ms; the build and the Worker glue here are what close the gap.
- **SDKs for other developers, in their language.** [Fern](https://buildwithfern.com) generates Go and TypeScript clients and a CLI from the specs; add a language with one block of configuration. A tagged release publishes them.
- **Real-time that does not lose messages.** SSE and WebSocket feeds that survive redeploys, dropped connections and idle hours, with a client rule and a soak test that proves it.
- **An MCP endpoint from the same contract:** every operation that answers once is a tool, for agents.
- **One way to run everything.** `mise run <task>` does the same thing on your machine, in GitHub Actions and in a project made from this one.

## What is in this repository

| Folder | What it is |
|---|---|
| `go/` | The Go library a project imports: Huma on Workers (`humaworkers`), the request bridge (`transport`), D1 (`d1`), the hub (`hub`), the gap-free feed (`follow`), MCP from a Huma API (`humamcp`), AsyncAPI for Huma (`asyncapi`) |
| `examples/` | Four complete projects: the notes API and a showcase of every Fern feature, each in Go and in TypeScript. `charter new` copies the Go notes one |
| `cmd/charter/` | The tool behind every task: build, bench, generate, release, scaffold |
| `docs/` | Everything else: [start here](docs/README.md), rendered at https://joeblew999.github.io/charter/ |

Generated files are marked as such and never edited by hand: [what is generated](docs/README.md#what-is-generated).

## Status

Used by other projects, released with version tags. TinyGo is used as it ships, with two small runtime patches applied at build time, both reported upstream ([upstream issues](docs/upstream.md)). To help: [How to help](docs/contributing.md).
