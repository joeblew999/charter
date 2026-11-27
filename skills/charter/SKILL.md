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
| What is followed live | one `live.Feed` (`api.Notes` in the example): `Publish` after the write; `Changed` after a change that is no new item (one taken away, one altered); for a page, an element with `hx-sse:connect={ feed.Signal() }` around the part that shows the feed, and on that part `hx-trigger={ live.OnChange }`, `hx-get={ live.Again(<the page's address>) }`, its own component as `hx-target`, `hx-swap="innerMorph"` and `hx-sync="this:replace"`: the stream says only that something changed and the part asks for itself again, which the page's `Props` answers for that target (the stream is the hub's, never a Go handler: a Go runtime held per open page takes the Worker over its 128 MB; never append what a stream sends, and never a route of its own for the rows); `Events` and `Lines` for the API, each given the answer's `SetHeader`: they write what the log has and hand the rest of the stream to the hub | |
| Reading and writing the database | nothing to write: a request's statements share a D1 session, the answer to a write gives a bookmark (the cookie `d1`, the header `X-D1-Bookmark`) that the next request brings back, and `charter deploy` turns D1's read replication on. A read that must be the newest (after someone else's write): `cloud.Newest()` before it. Never the binding's own `prepare` around `cloud`, and `cloud.SQL` reads the primary | `curl -H 'x-go-runtime: ?'` says who answered in `X-D1-Served`; `go tool charter run cold` says it from many places |
| The database's schema | `db/schema.sql`: the tables as they are now. A renamed column says `-- was: old_name` on its line | the running `charter dev` brings the local database to it; `go tool charter check --fix` writes the migration (never write one by hand); `go tool charter deploy` applies it |
| What a new checkout's database starts with, and what tests know is there | Values of the app's own types and a function that fills a store with them through the app's own functions (the example's `api.Seeded`, `api.Seed`); `seed.go` at the app's top (`//go:build seed`, `app.Seed` in its init) names it. A test of a page calls the same function on its own store. Seeded people have tokens (`ch_seed_…`) to act as them | `go tool charter run db:seed` makes the local database again and fills it, also beside a running `charter dev`; `dev` and `test` seed a database they have just made. `go tool charter run db:seed --deployed` says what the deployed database has; with the Worker's name after it, every row is deleted and the seed's put there, without the seeded tokens |
| A query | `db/queries.sql`; select whole rows (`SELECT *`, `RETURNING *`) so a new column needs no change here | `go tool charter check --fix` writes `db/queries.gen.go` |
| Something Cloudflare gives a Worker, put to use | Its own job in the app and its own page, used when that page is opened or that thing is done, never to show another page (the example's package `built`). Keep a time as a time and a count as a count; the page shows them through Intl's filters and a message's plural | |
| A binding, a variable | `wrangler.jsonc`, without ids: the first deploy makes the resource | the running `charter dev` reloads |
| Browser code or CSS | `web/` (Vite); `go tool charter add <npm package>` | |
| A message a reader sees | `locales/en.toml`, written `{ locales.Key }` in gsx | `go tool charter run intl:translate`; `go tool charter run intl:status` says what is left |
| A language | `go tool charter add locale:de` | translate its catalog |
| A page of the docs | `docs/NN-name.md`: Markdown, its first heading the title; link to another page by its file. A code sample comes from a real file: open its fence with ```` ```go file=pages/pages.go#Home.Props ```` (a declaration by name; no `#` for a whole file) and `check --fix` writes the code. The pages that show them are the docs screen (`go tool charter add screen:docs`, mounted in the tree at `/docs`; `app.App`'s `Docs`) | the running `charter dev` shows it at `/docs/name`; it is in the sitemap, `/docs/name.md`, `/llms.txt` and `/llms-full.txt` |
| A UI component | `go tool charter add ui:dialog` (gsxui add; `-diff`, `-overwrite` pass through); `go tool charter run ui:list` | |
| Who may do what | One `api.Auth` for the pages and the API (`api.NewAuth`, given to `app.App` and used by the contract). An operation: `Security: charter.Needs("write")`. A page or a group of the tree: `func (T) Needs() []string` in the same words. Who is asking: a `caller api.Caller` parameter of `Props`; in a component `caller.May("write")`. Never check a cookie or a header yourself | the example's `test/page_signin_test.go` holds the page and the API to one rule |
| Roles | `api.People`'s `Roles` (a name and its scopes) and `Role` (what someone who signs up is), in one place. A rule about more than a scope (whose note it is) is one function the page and the API's handler both call (the example's `api.MayRemove`). Who is what: the people page, or the API's `setRole` | |
| People signing in | `go tool charter add screen:sign-in`: passkeys, sessions, the account page (personal tokens, deleting one's own account), the people page for an admin (roles, invitations with a share button, removing a person), the tables in `db/schema.sql`. The name asked at sign-up is a display name, shown publicly: never an email address. What the app keeps of a person goes with them: `api.People{…, Leaving: func(ctx, who) error}`. Never passwords | a browser test signs up with `page.SignUp(path, name)` |
| What every app has: the language list, the offer of the reader's language, the time zone dialog, the light and dark switch, the docs' pages | `go tool charter add screen:<name>`: charter's screens, copied into `pages/screens` and built from the app's own gsxui components. Never write one of these by hand | an unchanged copy follows charter through `check --fix`; a changed one is the app's |
| The look: colours, radius, fonts, the component style | `go tool charter run ui:theme` shows it; `go tool charter run ui:theme '<share code from https://ui.gsxhq.dev/theme>'` or a preset file switches it (gsxui apply). Light and dark are the theme's own; `<screens.ThemeToggle/>` switches | |
| What an admin sees of the system | `go tool charter add screen:admin`: overview (versions, people by role, invitations, the app's own `app.App{Facts: …}`), system (the Worker's real bindings with a check, variables and secrets by name only), database (tables with row counts, migrations). `admin screens.Admin` in the tree, `<screens.AdminLink/>` in the header. The same answers for a program: `auth.Overview(r)`, `auth.System(r)`, `auth.Databases(r)` | the example's `test/page_admin_test.go` |
| A test of a page's logic | `pages/pages_test.go`: plain Go, the page handed a store of the test's own | `go test ./pages` |
| A test of a page as a reader uses it | `test/page_*_test.go`: `apptest.Browser(t)` (Chrome): `Open`, `Type`, `Click`, `WaitText`, `Tab`, `Has`, `JS` (to stand in for what a test cannot do as a reader, such as the share sheet); addresses and selectors from the tree (`at`, `el` in `test/tree_test.go`). A test leaves the Worker as it found it: `SignUp` deletes its account when the test ends (`page.Leave(path)` after signing up by hand), and a row a test writes it takes away in `t.Cleanup` | `go tool charter test`, and `--deployed` |
| A test of the API | `test/api_test.go`, through `sdk/go` | the same |
| What a test can count on | `go tool charter test` starts the Worker on a database of its own, filled by the seed and thrown away. A test that needs the seed's tokens asks `apptest.Local()` first: a deployed database never has them. A test that must be someone on the deployed Worker asks `apptest.Secret(t, "ADMIN_TOKEN")`, which `--deployed` fills from the app's `fnox.toml` | the example's `test/main_test.go` fails a local run that leaves more than the seed |
| A client in another language | `charter.toml`'s `[pack] more`; its proof against the Worker is `test/sdk/<language>/prove.*` | `go tool charter pack -prove` (containers; minutes) |
| The first admin, and who is what | `go tool charter run people:invite admin` (`--deployed` for the deployed Worker) prints a link that makes whoever signs up through it an admin, once, within the hour; after that roles are set and people invited on the people page. `go tool charter run people` lists them. Never SQL on the users table | the example's `TestAnInvitationMakesAnAdmin` |
| How the deployed Worker is doing | `go tool charter run health` (Cloudflare's count of how requests ended: over memory, over CPU, an exception are found only there); `go tool charter run logs --since 1h --errors` for what was kept, `--find <reference>` for one request. A failed request is logged by the app as one JSON line with a reference the reader is given; never log a body, a header, a token or a name | `go tool charter test --deployed` and `deploy` say how it did before them; `go tool charter run load --deployed` before a release (many at once, with streams; waits a minute for Cloudflare's count); `go tool charter run cold` after a change to what the Worker loads or does as it starts (a page asked from 30 places twice, through Globalping: what a reader waits where no instance is running; `-everywhere` for one probe in every country, which maps the data centres they reach into `.charter/colos.json`); `.github/workflows/health.yml` asks daily. The build says the bundle's size, deploy the start-up time, test what a Go runtime holds: each remarked on when it grows or nears its limit |
| The Worker's database, KV namespace, bucket, queue | Named in `wrangler.jsonc`; the first `go tool charter deploy` makes them and writes the database's and namespace's ids there (commit that). A bucket or queue whose name changed stops the deploy (`--new-resources` to mean it). `go tool charter run delete` says what there is; with the Worker's name it deletes all of it (`--keep-data`: only the Worker and its secrets) | `go tool charter run resources` |
| Email the Worker sends | The `send_email` binding. To anyone, the domain it sends from is onboarded once (Workers Paid): `go tool charter run mail` lists the account's sending domains, `go tool charter run mail:enable mail.example.com` onboards one (a subdomain keeps its records apart from the domain's own mail) | |
| A secret of the deployed Worker | Named in `.dev.vars.example` with a throwaway value, and declared in the app's `fnox.toml` (committed: the name and the provider that keeps it, never the value). `go tool charter run secrets` lists them; `go tool charter run secrets:put NAME --random` (or a value piped in) has fnox keep it and puts it on the Worker. An environment is a fnox profile: `secrets:put staging NAME`. Never `fnox` or `wrangler secret` by hand, never a value in a file | `go tool charter test --deployed` hands the tests the ones fnox has |
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
`go tool charter test --deployed`. Tools come from mise: `mise.toml` pins Go, Node and fnox, `go tool charter check` fails where other versions run (a shell without mise's hook: `eval "$(mise hook-env)"`), and `check --fix` raises an app's pins to charter's. Credentials come from fnox: once, `fnox set CLOUDFLARE_API_TOKEN` and
`fnox set CLOUDFLARE_ACCOUNT_ID`; in CI they are the environment's. Containers for `go tool charter pack` are Kiri,
which charter installs and starts.
