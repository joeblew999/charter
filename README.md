# charter

Go apps on Cloudflare Workers, one way: pages, an API from a contract, a live feed behind both, languages and search
engines, in one Worker.

charter is a Go library and one command. It drives the tools that own each job and adds what they lack:

| Job | Tool | Its file |
|---|---|---|
| The Worker: run it, ship it, its bindings | [wrangler](https://developers.cloudflare.com/workers/wrangler/) | `wrangler.jsonc` |
| Go on Workers | [workers-go](https://github.com/syumai/workers-go) | `go.mod` |
| Components and the dev loop | [gsx](https://gsxhq.dev), [gsxui](https://ui.gsxhq.dev) | `gsx.toml` |
| Pages and their routes | [structpages](https://github.com/jackielii/structpages) | `pages/` |
| The browser's files | [Vite](https://vite.dev), as `gsx init` sets it up | `vite.config.ts` |

**A charter app runs on Cloudflare, nowhere else.** Locally that is `wrangler dev` (the same runtime), remotely it is
Cloudflare. There is no other server, and nothing that stands in for a binding.

## You need

Go and Node. `go.mod`'s toolchain line and `package-lock.json` pin every tool, on macOS, Linux and Windows. (Docker
only to generate SDKs.)

## Start

```sh
go run github.com/joeblew999/charter/cmd/charter@latest create my-app
cd my-app
go tool charter dev
```

That copies [`example/`](example), the one app that uses all of charter, under your name.

## An app, by convention

charter finds everything by where it is. There is nothing to configure for any of this:

```
pages/         the page tree (structpages) and its gsx: routes, links, the sitemap
api/           contract.go: the API, from which come its specs, MCP tools and SDKs
db/            migrations/*.sql and queries.sql, written as typed Go (db/queries.gen.go)
locales/       <language>.toml: the languages are the files here
web/  public/  what the browser loads (Vite)
ui/            gsxui's components
test/          tests of the pages (a real browser) and the API (its SDK), on wrangler dev and deployed
  dev/         only a local Worker
  deployed/    only a deployed one
sdk/go/        the app's own Go client, generated from the contract
wrangler.jsonc the Worker and its bindings, without ids
charter.toml   only url, and what Fern should generate
main.go
```

```go
func main() {
	(&app.App{Name: "My App", Dist: dist, Pages: pages.Mount, API: api.Handler(env)}).Run()
}
```

## What you never have to remember

- **Skills for your coding agent.** gsx, gsxui, structpages and charter each ship their skill inside their Go module.
  `charter install` copies them into `.claude/skills/` from the versions `go.mod` pins, and `dev` and `check` do it
  again whenever `go.sum` has moved. Updating a module updates its skill.
- **Generation.** `dev` and `build` write what is built from your sources (gsx's `*.x.go`, the messages, the browser's
  files): none of it is committed. `check --fix` writes what is committed and held to its source: `db/queries.gen.go`,
  `fern/`, `sdk/go/`. Everything generated is named for it (`*.gen.go`, `*.x.go`) or in a folder of its own, and
  `.gitattributes` marks it.
- **The database.** `db/schema.sql` is the one place a table is written down. `dev` keeps the local database in step
  with it as you edit, keeping its rows; `check --fix` writes the migration for what changed (one for everything not yet committed) and
  the typed Go for `db/queries.sql`; `deploy` applies it. A change that would lose data is refused until you ask for it (`check --fix --drop`); a renamed
  column says `-- was: old_name`. A column's comment (`-- example: 42`) becomes its tag, so
  the row is also what the API's contract serves: a note is written down once.
- **The upstream tools.** `gsx generate`, `gsx fmt`, `gsx dev`, `gsxui migrate rtl`, `structpages-lint`, `sqlc`,
  `vite build`, `wrangler d1 migrations apply` all run inside `dev`, `check`, `test` and `deploy`, at the moment
  they are needed.

## Commands

The commands are [Vite+](https://viteplus.dev/guide/)'s verbs. Run them as `go tool charter <command>` in the app's
folder.

| Command | What it does |
|---|---|
| `create` | A new app, from the example |
| `dev` | The loop you work in, on the real runtime. Leave it running and run everything else beside it |
| `check` | What can be said without running the app: format, lint, the contract, the languages. `--fix` writes what it can |
| `test` | The Worker on wrangler dev: the standard API test, `test/` (pages in a real browser, the API through its own SDK), the search-engine audit. `--deployed` asks the deployed one the same |
| `deploy` | `wrangler deploy` and the D1 migrations. The first deploy makes the database, KV namespace, bucket and queue `wrangler.jsonc` names |
| `pack` | The SDKs, from the contract |
| `run <task>` | Everything that is no verb, by what it is about: `intl:translate`, `intl:status`, `seo:audit`, `seo:lighthouse`, `db:status`, `db:reset`, `db:seed`, `db:execute`, `db:export`, `ui:list`, `ui:theme`, `logs`, `bench`, and the app's own. Alone, it lists them |
| `add`, `remove` | `add locale:de`, `add ui:dialog`, a Go module, an npm package |
| `build`, `preview`, `fmt`, `lint`, `install`, `exec`, `update`, `outdated`, `upgrade`, `staged`, `cache`, `env` | As in Vite+ |

Before a push: `go tool charter check && go tool charter test`.

## The library

Six packages, `github.com/joeblew999/charter/<name>`:

| Package | For |
|---|---|
| `app` | The app as a Worker: `app.App` (pages, API, Vite, languages, search engines in one value), `<app.Head>`, and `app/apptest` for its tests: `Browser` (Chrome), `Get`, `Events`, `Secret`, `Local` |
| `api` | An API from a contract: Huma on Workers, OpenAPI and AsyncAPI, MCP tools, auth, rate limits |
| `live` | A feed declared once (`live.Feed`): a D1 table as the log, a Durable Object to wake followers, and from it the page's live fragments, the API's stream and its WebSocket channel |
| `cloud` | What workers-go lacks or gives differently: D1 as typed rows and batches, rate limits, email. For everything else, import workers-go's own packages |
| `intl` | Languages, from the runtime's Intl: messages, plurals, numbers, dates, direction, a URL per language |
| `seo` | What search engines read: robots.txt, the sitemap, the head, and the audit |

## The example

[`example/`](example) is deployed at https://charter-example.gedw99.workers.dev. It has pages in three languages (one
right to left), a notes API with tokens, limits, a stream and SDKs in Go, Python and as a CLI, one feed behind the
page's live list and the API's stream, and `/cloud`: a card for everything Cloudflare gives a Worker (KV, R2, Queues,
D1, Cron Triggers, the Cache API, Durable Objects, sockets, fetch, email), each used through workers-go's own package.
Its tests use its own generated Go SDK as the client, and run unchanged against the local and the deployed Worker.
