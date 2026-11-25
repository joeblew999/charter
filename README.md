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

```sh
go run github.com/joeblew999/charter/cmd/charter@latest create my-site -from site
cd my-site
go tool charter dev
```

`-from api` is the smallest API, `-from notes` a full one. `go tool charter env` says what your machine has.

## Commands

The commands are [Vite+](https://viteplus.dev/guide/)'s, for a Go app on Workers: the same verbs, each driving the
tool that owns the job. Run them as `go tool charter <command>` in the app's folder.

| Command | What it does |
|---|---|
| `create` | A new app from a template |
| `install` | Everything the app pins: npm's packages, Go's modules |
| `dev` | The loop you work in, on the real runtime. Leave it running |
| `check` | Everything that can be said without running the app: format, lint, the contract, the languages. `--fix` writes what it can |
| `fmt`, `lint` | The two halves of `check`, alone |
| `test` | `go test`, then the Worker on wrangler dev: its API, the app's own tests, the search-engine audit. `test e2e` runs a suite; `--base <url>` tests a deployed Worker |
| `build` | The messages, the components' Go, the browser's files and the Wasm. wrangler runs it itself |
| `preview` | The production build on wrangler dev |
| `deploy` | The D1 migrations, then wrangler deploy: `deploy staging` |
| `pack` | What others consume: an SDK from the contract |
| `run <task>` | Everything that is not a verb: `translate`, `languages`, `lighthouse`, `badges`, `logs`, `bench`, and the app's own tasks. Alone, it lists them |
| `exec <tool>` | One of the app's tools as it pins it: `exec wrangler …`, `vite`, `gsx`, `gsxui` |
| `add`, `remove` | `add locale:de`, `add ui:dialog`, `add d1:DB`, a Go module, an npm package |
| `update`, `outdated` | Every pin, Go's and npm's |
| `upgrade`, `migrate` | charter itself to its latest; an older app moved onto this one |
| `staged` | Format and check what is staged for a commit; `staged install` makes it the pre-commit hook |
| `cache clean` | Everything a build wrote, removed |
| `env` | What this machine has of what an app needs |

Before a push: `go tool charter check && go tool charter test`.

## Leave `dev` running

`charter dev` is gsx's loop with wrangler as its server and Vite in front, so what you look at is the Workers
runtime with `wrangler.jsonc`'s bindings, as in production. Everything else you do in a second terminal reaches the
open page by itself, in about three seconds:

| You | The page |
|---|---|
| Save Go or a `.gsx` file | rebuilds, restarts the Worker, reloads |
| Break the build | keeps the last version that worked, and says what is wrong |
| `charter add locale:fr`, `charter run translate`, or edit a catalog | the language appears, the text changes |
| `charter add ui:badge` and use it | appears |
| `charter add leaflet` and import it in `web/` | Vite serves it |
| Change a variable in `wrangler.jsonc` | wrangler reloads |

## An app

```
my-app/
  main.go          transport.Run(handler): a Worker on Cloudflare, a plain server with go run .
  wrangler.jsonc   the Worker: name, bindings, variables, environments
  charter.toml     what wrangler has no place for: the contract, SDKs, tests, languages, search engines
  package.json     wrangler (and Vite, for a site)
  go.mod           charter the library, and charter the command as a tool
  api/             an API's contract and handlers
  pages/           a site's gsx components
  locales/         a site's text, a file per language
  ui/              gsxui's components, as gsxui add copies them
  web/             a site's browser files, which Vite builds
  public/          files served as they are (the icon)
  migrations/      D1's *.sql
```

`vite.config.ts` is the one file with a TypeScript name: gsx writes it and gsxui edits it by that name. There is no
TypeScript of your own to write.

## A site in one call

```go
//go:embed dist/.vite/manifest.json
var dist embed.FS

func main() {
	a := &gsxapp.App{Name: "My App", Dist: dist, Pages: []string{"/", "/about"}}
	transport.Run(a.Handler(mux))
}
```

```gsx
<html { gsxapp.HTML(ctx)... }>
	<head>
		<gsxapp.Head title={ locales.HomeTitle |> text } description={ locales.HomeDescription |> text }/>
	</head>
```

That is Vite's files in dev and in production, a URL per language (`/about`, `/de/about`) with every alternate in
`<head>`, `robots.txt`, a sitemap in every language, `/healthz`, and `noindex` on previews. `charter test` then
audits the site the way a search engine reads it.

Languages and search engines are two tables of `charter.toml`:

```toml
url = "https://app.example.com"

[intl]                # charter add locale:de writes it
locales = ["en", "de"]
catalogs = "locales"

[seo]                 # the live site is url, above
```

## The library

Every package is at the top: `github.com/joeblew999/charter/<name>`.

| For | Packages |
|---|---|
| Cloudflare from Go | `transport` (run a handler as a Worker), `bindings` (variables, static assets, rate limits, fetch), `d1`, `hub` (Durable Object fan-out), `ratelimit` |
| APIs from a contract | `serve` (the standard API), `specs`, `asyncapi`, `humaworkers` (Huma on Workers), `mcp`, `auth`, `problem`, `follow` (SSE and WebSocket streams) |
| Sites | `gsxapp` (the one call above), `web`, `fragments`, `intl` and `gsxintl` (languages), `seo` and `gsxseo` (search engines) |

Every binding has a stand-in off Workers: `d1.Open` is SQLite in memory with your migrations applied, a rate limit
counts in memory, the hub is in the process, Intl is a local workerd. So one `main.go` serves both, and `go test` runs
the real SQL.

The rest of the repo: [`cmd/charter`](cmd/charter) is the command, [`internal/`](internal) its code,
[`worker/`](worker) the JavaScript that runs the Go, [`examples/`](examples) three apps.

## Examples

| Example | What it shows |
|---|---|
| [`examples/api`](examples/api) | The smallest API: one route from a contract, its specs, MCP |
| [`examples/site`](examples/site) | An API and its pages in one Worker: gsx, gsxui, htmx 4, Vite, three languages (one right-to-left), search engines, D1, live over SSE |
| [`examples/notes`](examples/notes) | A full API: auth, D1, SSE and WebSocket streams, rate limits, SDKs in Go and Python and a command-line client |

In this repo: `npm install` once at the top (the examples share one `node_modules`), then `go tool charter dev` or
`go tool charter check && go tool charter test` in an example.

## Size

A Worker built with the standard Go compiler is about 4 MB gzipped, for the smallest API and for a site alike. Workers Paid takes 10 MB;
Workers Free takes 3 MB, so these do not fit the free plan.
