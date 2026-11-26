---
name: charter
description: Use when a Go app runs on Cloudflare Workers with github.com/joeblew999/charter (go tool charter, wrangler.jsonc, pages/, api/contract.go, db/, locales/) — adding or changing a page, a route, an API operation, a live feed, a binding (D1, a Durable Object, KV, a rate limit, a variable), a migration or query, a language, a test of the Worker, running it locally, deploying it, or when charter check reports a problem.
---

# A Go app on Workers with charter

One way: the app runs on Cloudflare only (`wrangler dev` locally, Cloudflare remotely), its pages are a structpages
tree rendered with gsx, and charter finds every part by where it is. Change the file, then let
`go tool charter check` say what is wrong.

This skill is the rules. How each part works is the docs, as Markdown: https://charter.gedw99.workers.dev/llms.txt
lists every page (`/docs/pages.md`, `/docs/api.md`, `/docs/live.md`, `/docs/database.md`, `/docs/languages.md`,
`/docs/tests.md`, `/docs/deploy.md`, `/docs/docs.md`), and `/llms-full.txt` is all of them. Read the page before
changing that part.

| To change | Edit | Then |
|---|---|---|
| A page or its route | `pages/`: a type in the tree (`pages.Pages`) with a `Page` component, and `Props` that names what it needs as parameters (the store, from `app.App`'s `RequestArgs`) | the running `charter dev` reloads it |
| A region htmx replaces | a component of its page (`component (p Home) Form(…)`), with `id={Home.Form \|> id}` and `hx-target={Home.Form \|> target}` | |
| One of many that updates by itself (a row) | `id={Home.Item \|> key(id)}`, its control `hx-get={Home{} \|> url}` with `hx-target={Home.Item \|> keytarget(id)}`; `Props` takes a `structpages.RenderTarget`, asks `target.Is(p.Item)`, reads the row from `structpages.KeyOf(target)` and loads that alone | |
| The link of the page being shown | `structpages.Within(ctx, Page{})` | |
| Markup for part of a page with its own address (a dialog's form) | a fragment: a type with `Props(…) error` returning `structpages.RenderComponent(…)` and no `Page`. charter sends a browser that opens it to the page it is part of | |
| What a page calls | a type with `ServeHTTP(w, r, …) error` that returns `structpages.RenderComponent(…)`, or `app.NotFound`, `app.Status`, `app.Redirect{To}`; it never writes an error or a redirect itself | |
| A link, a form's target, a stream | `{ Page{} \|> url }`, `{ PostNote{} \|> url }`, with parameters `\|> url(map[string]any{"id": id})`: structpages' own `URLFor`, in the reader's language by itself. Never a path as a string | |
| A type the API serves that is a table's row | embed the row (`type Note struct{ db.NotesRow }`); its examples are the schema's comments (`-- example: 42; doc: …`) | |
| An API operation | `api/contract.go` and its handler | `go tool charter check --fix` writes `fern/`; `go tool charter pack -publish` the Go SDK |
| What is followed live | one `live.Feed` (`api.Notes` in the example): `Publish` after the write, `Fragments` for a page, `Events` and `Lines` for the API | |
| The database's schema | `db/schema.sql`: the tables as they are now. A renamed column says `-- was: old_name` on its line | the running `charter dev` brings the local database to it; `go tool charter check --fix` writes the migration (never write one by hand); `go tool charter deploy` applies it |
| A query | `db/queries.sql`; select whole rows (`SELECT *`, `RETURNING *`) so a new column needs no change here | `go tool charter check --fix` writes `db/queries.gen.go` |
| A binding, a variable | `wrangler.jsonc`, without ids: the first deploy makes the resource | the running `charter dev` reloads |
| Browser code or CSS | `web/` (Vite); `go tool charter add <npm package>` | |
| A message a reader sees | `locales/en.toml`, written `{ locales.Key }` in gsx | `go tool charter run intl:translate`; `go tool charter run intl:status` says what is left |
| A language | `go tool charter add locale:de` | translate its catalog |
| A page of the docs | `docs/NN-name.md`: Markdown, its first heading the title; link to another page by its file. A code sample comes from a real file: open its fence with ```` ```go file=pages/pages.go#Home.Props ```` (a declaration by name; no `#` for a whole file) and `check --fix` writes the code. The pages that show them are the docs screen (`go tool charter add screen:docs`, mounted in the tree at `/docs`; `app.App`'s `Docs`) | the running `charter dev` shows it at `/docs/name`; it is in the sitemap, `/docs/name.md`, `/llms.txt` and `/llms-full.txt` |
| A UI component | `go tool charter add ui:dialog` (gsxui add; `-diff`, `-overwrite` pass through); `go tool charter run ui:list` | |
| What every app has: the language list, the offer of the reader's language, the time zone dialog, the light and dark switch, the docs' pages | `go tool charter add screen:<name>`: charter's screens, copied into `pages/screens` and built from the app's own gsxui components. Never write one of these by hand | an unchanged copy follows charter through `check --fix`; a changed one is the app's |
| The look: colours, radius, fonts, the component style | `go tool charter run ui:theme` shows it; `go tool charter run ui:theme '<share code from https://ui.gsxhq.dev/theme>'` or a preset file switches it (gsxui apply). Light and dark are the theme's own; `<screens.ThemeToggle/>` switches | |
| A test of a page's logic | `pages/pages_test.go`: plain Go, the page handed a store of the test's own | `go test ./pages` |
| A test of a page as a reader uses it | `test/page_*_test.go`: `apptest.Browser(t)` (Chrome): `Open`, `Type`, `Click`, `WaitText`, `Tab`; addresses and selectors from the tree (`at`, `el` in `test/tree_test.go`) | `go tool charter test`, and `--deployed` |
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
- Never hand-write what a command generates: `db/migrations/`, `*.gen.go`, `*.x.go`, `fern/`, `sdk/`, `build/`, `dist/`.
- Not in git: `.dev.vars` (made from `.dev.vars.example`), and what is built each time (`*.x.go`,
  `*.built.go`, `.charter/`, `build/`, `dist/`). In git, and held to its source by `check`: `*.gen.go`,
  `db/migrations/`, `fern/`, `sdk/go/`.
- The app is `(&app.App{Name, Dist, Pages, API}).Run()` and `<app.Head title=… description=…/>`: never wire Vite,
  languages, the theme, robots.txt or the sitemap by hand. The page tree's options (the error handler, htmx 4's
  target selector, the language's prefix) are the app's: a test that mounts the tree itself passes
  `(&app.App{}).Options(false)`.
- No literal reader-facing text in a `.gsx` file: a message in the source catalog.
- `charter.toml` holds only `url` and `[pack]`.

## The gsx, gsxui and structpages skills

They are plugins of charter's marketplace beside this one (the app's `.claude/settings.json` enables them), each kept
by its tool's owner. Use them for how to write gsx, which component to choose and how a page works: they are right
about their tool. charter runs those tools for you, so where one of them tells you to run or set something up, this
is what it is in a charter app:

| Their skill says | In a charter app |
|---|---|
| `gsx generate`, `gsx dev`, `gsx fmt` | Nothing to run: `go tool charter dev` is `gsx dev` (with wrangler as its server), and `check --fix` generates and formats. Running one yourself is harmless; always as `go tool gsx …`, never a bare `gsx`. |
| `gsx info`, `gsx fix`, `gsx lsp` | `go tool gsx info` and the others, as they are. |
| `.x.go` committed | Not in git here: built each time. |
| `gsxui init`, `gsxui add <name>`, `gsxui list` | `go tool charter add ui:<name>`, `go tool charter run ui:list`. The app is set up already. |
| `gsxui apply --preset …` | `go tool charter run ui:theme <preset>`: the same, without the question it asks. |
| Register the `url`, `id`, `target`, `key` filters | Done in `gsx.toml`, exactly as structpages has them. |
| `structpages.Mount(…, WithErrorHandler, WithTargetSelector, WithArgs)` | `app.App` mounts the tree, once for each language. A handler returns `app.NotFound`, `app.Status` or `app.Redirect{To}`; what it needs is `app.App`'s `Args` and `RequestArgs`. |
| Run `structpages-lint` | `go tool charter check` runs it, for the host and for the Worker's build. |
| A dark mode toggle, a language switcher, docs pages | A screen: `go tool charter add screen:<name>`. |

## Working as an agent

- `go tool charter env` says whether `charter dev` is running here, at which address, and where its log is
  (`.charter/dev/dev.log`: the builds and the Worker). When it is running, do not start another: edit, and read
  the page at that address. A save reaches the page in about two seconds, and a mistake is in the log.
- No command asks a question. `go tool charter check` is the truth about the code: run it after a change, and
  `--fix` for what it can write. `go tool charter check --json` is the same as one document: `ok`, and each part
  with its output; the part that failed is the last.
- When no `charter dev` is running, `go tool charter test` starts a Worker of its own.

## Commands

`go tool charter` lists them: Vite+'s verbs. Keep `go tool charter dev` running and run everything else beside it.
Before a push: `go tool charter check && go tool charter test`. To ship: `go tool charter deploy`, then
`go tool charter test --deployed`. Credentials come from fnox, which charter brings itself: once, `fnox set CLOUDFLARE_API_TOKEN` and
`fnox set CLOUDFLARE_ACCOUNT_ID`; in CI they are the environment's. Containers for `go tool charter pack` are Kiri,
which charter installs and starts.
