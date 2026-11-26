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
| `test` | `go test`, then the Worker on wrangler dev: its API, the app's own tests, the search-engine audit. The same tests ask a deployed Worker with `--env <environment>` (its address in `charter.toml`'s `[urls]`, which `deploy` writes) or `--base <url>`; a test command gets `BASE` and `TARGET` (`local` or `deployed`). `test e2e` runs a suite |
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

`charter.toml` holds what wrangler has no place for, a block per command:

```toml
health = "/healthz"
url = "https://app.example.com"

[check]               # an API with a contract
spec = "fern/openapi.json"

[test]                # asked of the Worker, on wrangler dev or deployed: BASE is its URL, TARGET local or deployed
run = ["go test ./test"]

[urls]                # where each environment is deployed: charter deploy writes it, charter test --env reads it
preview = "https://app-preview.example.workers.dev"

[pack]                # the SDKs Fern makes from the specs
name = "App"

[staged]              # before a commit (default: charter check)
run = ["charter check"]

[intl]                # charter add locale:de writes it
locales = ["en", "de"]
catalogs = "locales"

[seo]                 # the live site is url, above
```

An app written for an older charter: `go tool charter upgrade` moves charter and runs `charter migrate`, which
rewrites the import paths, the names that moved with them and `charter.toml`.

## The library

Every package is at the top: `github.com/joeblew999/charter/<name>`. Fourteen of them, for three jobs.

**Cloudflare from Go, beside workers-go**

| Package | For |
|---|---|
| `transport` | Run a handler as a Worker (`transport.Run`), where a request came from, and net/http helpers for pages |
| `bindings` | What workers-go lacks, and stand-ins for `go test`: variables, static assets, rate limits, fetch, email |
| `d1` | D1 as typed rows and batches; under `go test` it is SQLite with the app's migrations |
| `live` | A feed that is followed live: a Durable Object as the wake-up, D1 as the log, server-sent events out |
| `ratelimit` | Limits per caller |

**An API from a contract**

| Package | For |
|---|---|
| `serve` | The standard API: specs, MCP tools, problem details, streams |
| `humaworkers` | Huma on Workers |
| `specs`, `asyncapi` | The OpenAPI and AsyncAPI documents |
| `auth` | OpenID Connect tokens |

**A site**

| Package | For |
|---|---|
| `gsxapp` | The one call above: Vite, static files, languages, search engines |
| `intl`, `gsxintl` | Languages |
| `seo` | Search engines |

## All of workers-go

Everything [workers-go](https://github.com/syumai/workers-go) has works as its own docs say: import its packages and
call them. charter's `transport.Run` serves through workers-go's own request path, a plain `workers.Serve` program
works too, and the Worker entry handles requests, Cron Triggers, queues and email.

| Cloudflare | Use | From |
|---|---|---|
| KV | `kv.NewNamespace("KV")` | workers-go |
| R2 | `r2.NewBucket("BUCKET")` | workers-go |
| Queues | `queues.NewProducer("QUEUE")`, `queues.ConsumeNonBlock(f)` | workers-go |
| D1 | `d1.OpenConnector("DB")` with `database/sql` | workers-go |
| Cron Triggers | `cron.ScheduleTaskNonBlock(f)` | workers-go |
| Cache API | `cache.New()` | workers-go |
| Durable Objects | `cloudflare.NewDurableObjectNamespace("NAME")`, its stub's `Fetch` | workers-go |
| fetch, service bindings | `fetch.NewClient()` | workers-go |
| TCP sockets | `sockets.Connect(ctx, addr, nil)` | workers-go |
| Variables, waitUntil, request.cf | `cloudflare.Getenv`, `cloudflare.WaitUntil`, `fetch.NewIncomingProperties` | workers-go |
| Email, out and in | `bindings.SendEmail("MAIL", from, to, raw)`, `bindings.ConsumeEmail(f)` | charter, until workers-go has it |
| Rate limits, a fan-out hub | `bindings.Allow`, `live.DurableObject` | charter |
| Any other binding | `cloudflare.GetBinding("NAME")`, a `js.Value` | workers-go |

[`examples/site`](examples/site)'s `/cloud` page uses every one of them, a card each, in a page you can click
through (package `cloud`, a file per binding), with a test per card that runs on wrangler dev (`test/cloud_test.go`).

A file that imports workers-go's packages builds only for a Worker (`//go:build js && wasm`): they are `syscall/js`.
That is fine now that `charter dev` and `charter test` run the Worker itself. For code that should also run under
`go test`, charter has stand-ins:

Every binding below has a stand-in off Workers: `d1.Open` is SQLite in memory with your migrations applied, a rate limit
counts in memory, the hub is in the process, Intl is a local workerd. So one `main.go` serves both, and `go test` runs
the real SQL.

The rest of the repo: [`cmd/charter`](cmd/charter) is the command, [`internal/`](internal) its code,
[`worker/`](worker) the JavaScript that runs the Go, [`examples/`](examples) three apps.

## Examples

| Example | What it shows |
|---|---|
| [`examples/api`](examples/api) | The smallest API: one route from a contract, its specs, MCP |
| [`examples/site`](examples/site) | An API and its pages in one Worker: gsx, gsxui, htmx 4, Vite, three languages (one right-to-left), search engines, D1, live over SSE, and `/cloud`, a page with a card for everything workers-go gives a Worker, and email |
| [`examples/notes`](examples/notes) | A full API: auth, D1, SSE and WebSocket streams, rate limits, SDKs in Go and Python and a command-line client |

In this repo: `npm install` once at the top (the examples share one `node_modules`), then `go tool charter dev` or
`go tool charter check && go tool charter test` in an example.

## Size

A Worker built with the standard Go compiler is about 4 MB gzipped, for the smallest API and for a site alike. Workers Paid takes 10 MB;
Workers Free takes 3 MB, so these do not fit the free plan.
