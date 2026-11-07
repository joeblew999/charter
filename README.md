# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

**The problem.** An API on Cloudflare Workers needs much more than its handlers: input checked, a spec, SDKs, tools for AI agents, sign-in, rate limits, streams that survive a deploy, tests on the real runtime, CI and docs. Each is usually set up by hand, differently in every repo, and drifts as the API changes. In Go there is more: a Worker is WebAssembly built with TinyGo, whose gaps `go test` can't see.

**What charter does.** You write one thing, the contract, in TypeScript ([oRPC](https://orpc.dev)) or Go ([Huma](https://huma.rocks)), and the rest follows from it and is checked against it: the OpenAPI and AsyncAPI specs, SDKs in any language [Fern](https://buildwithfern.com) supports and a CLI, an MCP tool for every operation that answers once, and the same sign-in and refusals in every API, tested on every run. A Go API is built with TinyGo and tested under workerd, at TypeScript's cost for a warm request ([Benchmarks](docs/self/benchmarks.md)). It is one command, `charter`, which mise installs: it makes a new API, keeps a repo you have the charter way, and runs each check here as GitHub runs it. Why, and how: [docs/](docs/self/README.md), rendered at <https://joeblew999.github.io/charter/>.

```sh
mise use -g github:joeblew999/charter   # charter, the one command
charter new my-api                      # a new API (-go: in Go)
charter init                            # the repo you have, kept the charter way
```
