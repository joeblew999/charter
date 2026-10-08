# charter

[![check](https://github.com/joeblew999/charter/actions/workflows/check.yml/badge.svg)](https://github.com/joeblew999/charter/actions/workflows/check.yml)
[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

**Contract-first APIs on Cloudflare Workers, with [Huma](https://huma.rocks) in Go or [oRPC](https://orpc.dev) in TypeScript.** Write the contract once and get the server, the OpenAPI and AsyncAPI specs, typed SDKs in any language ([Fern](https://buildwithfern.com)), a CLI, an MCP endpoint and real-time streams: generated from that contract, and tested.

```sh
mise x go node github:joeblew999/charter -- charter new -name billing-api -module github.com/you/billing-api   # -lang ts: oRPC
cd billing-api && mise install && mise run setup && mise run check   # a working API, checked locally
mise run deploy                                                       # on Cloudflare
```

Needs [mise](https://mise.jdx.dev) and git, on macOS, Linux or Windows: mise fetches the charter tool (a release binary), Go and Node. Then: [Getting started](docs/getting-started.md).

A repo with no API takes the docs site, the repo settings, the rules, the issue forms and the upstream tracking with `charter adopt`: [Any repo](docs/guides/any-repo.md).

## What you get

- **The contract is the source.** Change it, run `mise run spec`, and everything downstream follows. A stale spec fails the check.
- **Huma on Workers at TypeScript's cost,** through [workers-go](https://github.com/syumai/workers-go) and [TinyGo](https://tinygo.org): measured side by side ([performance](docs/benchmarks.md)).
- **SDKs and a CLI** for other developers, from the specs. A tagged release publishes them.
- **Real-time that loses nothing:** SSE and WebSocket feeds that survive redeploys and dropped connections, proven by a soak test.
- **MCP:** every operation that answers once is a tool for agents.
- **One way to run everything:** `mise run <task>`, the same locally, on GitHub and in every project.

## What is in this repository

| Folder | What it is |
|---|---|
| `go/` | The Go library a project imports |
| `examples/` | The notes API as a complete project, in Go (what `charter new` copies) and in TypeScript |
| `conformance/` | Every Fern feature end to end (OAuth, idempotency, uploads, webhooks, pagination, typed WebSockets), in Go and in TypeScript: the proof that the generated SDKs work |
| `cmd/charter/` | The tool behind every task |
| `docs/` | Everything else: [start here](docs/README.md), rendered at https://joeblew999.github.io/charter/ |

To help: [How to help](docs/contributing.md). License: [MIT](LICENSE).
