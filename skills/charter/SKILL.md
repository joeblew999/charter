---
name: charter
description: Use when a Go app runs on Cloudflare Workers with github.com/joeblew999/charter (go tool charter, wrangler.jsonc, pages/, api/contract.go, db/, locales/) — adding or changing a page, a route, an API operation, a live feed, a binding (D1, a Durable Object, KV, a rate limit, a variable), a migration or query, a language, a test of the Worker, running it locally, deploying it, or when charter check reports a problem.
---

# A Go app on Workers with charter

One way: the app runs on Cloudflare only (`wrangler dev` locally, Cloudflare remotely), its pages are a structpages
tree rendered with gsx, and charter finds every part by where it is. Change the file, then let
`go tool charter check` say what is wrong.

| To change | Edit | Then |
|---|---|---|
| A page or its route | `pages/`: a type in the tree (`pages.Pages`) with a `Page` component | the running `charter dev` reloads it |
| A link or a form's target | `{ Page{} \|> url }`, `{ Action{} \|> action }`: never a path as a string | |
| A type the API serves that is a table's row | embed the row (`type Note struct{ db.NotesRow }`); its examples are the schema's comments (`-- example: 42; doc: …`) | |
| An API operation | `api/contract.go` and its handler | `go tool charter check --fix` writes `fern/`; `go tool charter pack -publish` the Go SDK |
| What is followed live | one `live.Feed` (`api.Notes` in the example): `Publish` after the write, `Fragments` for a page, `Events` and `Lines` for the API | |
| The database's schema | `db/schema.sql`: the tables as they are now. A renamed column says `-- was: old_name` on its line | the running `charter dev` brings the local database to it; `go tool charter check --fix` writes the migration (never write one by hand); `go tool charter deploy` applies it |
| A query | `db/queries.sql`; select whole rows (`SELECT *`, `RETURNING *`) so a new column needs no change here | `go tool charter check --fix` writes `db/queries.gen.go` |
| A binding, a variable | `wrangler.jsonc`, without ids: the first deploy makes the resource | the running `charter dev` reloads |
| Browser code or CSS | `web/` (Vite); `go tool charter add <npm package>` | |
| A message a reader sees | `locales/en.toml`, written `{ locales.Key }` in gsx | `go tool charter run intl:translate`; `go tool charter run intl:status` says what is left |
| A language | `go tool charter add locale:de` | translate its catalog |
| A UI component | `go tool charter add ui:dialog` | |
| A test of a page | `test/page_*_test.go`: `apptest.Browser(t)` (Chrome): `Open`, `Type`, `Click`, `WaitText`, `Tab` | `go tool charter test`, and `--deployed` |
| A test of the API | `test/api_test.go`, through `sdk/go` | the same |
| The database by hand | `go tool charter run db:status`, `db:reset`, `db:seed`, `db:execute "…"` (`--deployed` for the deployed one) | |

## Rules

- **Nothing runs off Cloudflare.** No stand-ins: a test of anything that touches a binding is in `test/` and asks the
  Worker. Plain logic is an ordinary `go test`.
- **Bindings are workers-go's**, imported and called directly (`workers-go/cloudflare/kv`, `r2`, `queues`, `d1`,
  `cron`, `cache`, `fetch`, `sockets`), in a file that builds only for a Worker (`//go:build js && wasm`). charter's
  `cloud` has what workers-go lacks: `cloud.Open`/`Query`/`Batch` (D1 as typed rows), `cloud.Allow`, `cloud.Var`,
  `cloud.SendEmail`, `cloud.ConsumeEmail`. Register what is no request before the app runs:
  `cron.ScheduleTaskNonBlock`, `queues.ConsumeNonBlock`, `cloud.ConsumeEmail`.
- **Memory between requests is a cache, never state**: any request may start a new Go runtime.
- **D1 has no interactive transactions**: single statements with `RETURNING`, or `cloud.Batch`.
- Never hand-write what a command generates: `db/migrations/`, `*.gen.go`, `*.x.go`, `fern/`, `sdk/`, `build/`, `dist/`. Never run
  gsx, gsxui's migrations, sqlc or structpages-lint by hand either: `dev`, `check --fix` and `test` run them.
- The skills for gsx, gsxui and structpages are plugins of charter's marketplace beside this one (the app's
  `.claude/settings.json` enables them; `go tool charter install` installs them): use them for those tools' own rules.
- Not in git: `.dev.vars` (made from `.dev.vars.example`), and what is built each time (`*.x.go`,
  `*.built.go`, `.charter/`, `build/`, `dist/`). In git, and held to its source by `check`: `*.gen.go`,
  `db/migrations/`, `fern/`, `sdk/go/`.
- The app is `(&app.App{Name, Dist, Pages, API}).Run()` and `<app.Head title=… description=…/>`: never wire Vite,
  languages, robots.txt or the sitemap by hand.
- No literal reader-facing text in a `.gsx` file: a message in the source catalog.
- `charter.toml` holds only `url` and `[pack]`.

## Commands

`go tool charter` lists them: Vite+'s verbs. Keep `go tool charter dev` running and run everything else beside it.
Before a push: `go tool charter check && go tool charter test`. To ship: `go tool charter deploy`, then
`go tool charter test --deployed`. Credentials come from fnox, which charter brings itself: once, `fnox set CLOUDFLARE_API_TOKEN` and
`fnox set CLOUDFLARE_ACCOUNT_ID`; in CI they are the environment's. Containers for `go tool charter pack` are Kiri,
which charter installs and starts.
