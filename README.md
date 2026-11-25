# charter

Go apps on Cloudflare Workers: APIs from a contract, server-rendered sites, or both in one Worker.

charter is a Go library and one command. It does not replace the tools that own each job, it drives them:

| Job | Tool | Its file |
|---|---|---|
| The Worker: run it, ship it, its bindings | [wrangler](https://developers.cloudflare.com/workers/wrangler/) | `wrangler.jsonc` |
| Go on Workers | [workers-go](https://github.com/syumai/workers-go) | `go.mod` |
| Components and the dev loop | [gsx](https://gsxhq.dev) | `gsx.toml` |
| The browser's files | [Vite](https://vite.dev), as `gsx init` sets it up | `vite.config.ts` |

What charter adds: Go runtimes kept between requests (a Worker that answers in a few milliseconds), bindings that also
work off Workers so `go test` and `go run .` need no Cloudflare, an API's specs, SDKs and MCP tools from its contract,
languages (`charter intl`) and what search engines read (`charter seo`).

## You need

Go and Node. Nothing else: `go.mod`'s toolchain line and `package-lock.json` pin every tool, on macOS, Linux and
Windows. (Docker only to generate SDKs.)

## Start

Copy an example:

```sh
go run github.com/joeblew999/charter/cmd/charter@latest new my-app            # the smallest API
go run github.com/joeblew999/charter/cmd/charter@latest new my-site -from site
cd my-app
npm install
go tool charter dev
```

## Commands

`go tool charter <command>`, in the app's folder:

| Command | What it does |
|---|---|
| `dev` | The loop you work in: a save rebuilds and reloads (gsx dev; wrangler dev for an app with no gsx) |
| `build` | The messages, the components' Go, the browser's files and the Wasm. wrangler runs it itself |
| `preview` | The Worker as Cloudflare runs it, on wrangler dev |
| `check` | Everything before a push: format, vet, the specs, `go test`, the languages, the Worker on wrangler dev |
| `test <suite>` | One of `charter.toml`'s `[test.suites]` against the Worker; `-base https://…` for a deployed one |
| `with <command>` | A command run with the Worker on wrangler dev, `BASE` its URL |
| `deploy [env]` | The D1 migrations, then wrangler deploy |
| `live-test` | The standard test and `[test] live` against the deployed Worker |
| `spec` | An API's OpenAPI and AsyncAPI from its contract, Fern's settings, the queries' Go |
| `sdk` | An SDK from the specs with Fern |
| `intl …` | The app's languages: `charter intl help` |
| `seo …` | What search engines read: `charter seo help` |
| `new <dir>` | A new app from an example |

## An app

```
my-app/
  main.go          transport.Run(handler): a Worker on Cloudflare, a plain server with go run .
  wrangler.jsonc   the Worker: name, bindings, variables, environments
  charter.toml     only what wrangler has no place for: the contract, the SDKs, tests (optional)
  package.json     wrangler (and Vite, for a site)
  api/             an API's contract and handlers
  pages/           a site's gsx components
  web/             a site's browser files, which Vite builds
  migrations/      D1's *.sql
```

## The library

Three folders, by what you are building:

| Folder | Packages |
|---|---|
| [`cf/`](cf) Cloudflare from Go | `transport` (run a handler as a Worker), `bindings` (variables, static assets, rate limits, fetch), `d1`, `hub` (Durable Object fan-out), `ratelimit` |
| [`api/`](api) APIs from a contract | `serve` (the standard API), `specs`, `asyncapi`, `humaworkers` (Huma on Workers), `mcp`, `auth`, `problem`, `follow` (SSE and WebSocket streams) |
| [`site/`](site) Sites | `web`, `fragments`, `intl` and `gsxintl` (languages), `seo` and `gsxseo` (search engines) |

Every binding has a stand-in off Workers: `d1.Open` is SQLite in memory with your migrations applied, a rate limit
counts in memory, the hub is in the process. So one `main.go` serves both, and `go test` runs the real SQL.

## Examples

| Example | What it shows |
|---|---|
| [`examples/api`](examples/api) | The smallest API: one route from a contract, its specs, MCP |
| [`examples/site`](examples/site) | An API and its pages in one Worker: gsx, htmx 4, Vite, D1, live over SSE |
| [`examples/notes`](examples/notes) | A full API: auth, D1, SSE and WebSocket streams, rate limits, SDKs in Go, TypeScript, Python and a CLI |

In this repo: `npm install` once at the top (the examples share one `node_modules`), then `go tool charter check` in
an example.

## Size

A Worker built with the standard Go compiler is about 4 MB gzipped for the smallest API. Workers Paid takes 10 MB;
Workers Free takes 3 MB, so these do not fit the free plan.
