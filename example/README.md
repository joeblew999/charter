# The example

The one app that uses all of charter, and what `go tool charter create` copies. Deployed at
https://charter-example.gedw99.workers.dev. charter's docs: https://charter.gedw99.workers.dev/docs/.

`npm install` at the repo's top, then here:

```sh
go tool charter dev                              # leave it running → http://localhost:5173
go tool charter check && go tool charter test    # before a push
go tool charter deploy                           # then: go tool charter test --deployed
```

| Folder | What is in it |
|---|---|
| `pages/` | The page tree (structpages): the home page with its live list, the pages of how it's built (`Built`), and the routes they call |
| `api/` | The notes API's contract and handlers; `api.Notes` is the one feed behind the page's list and the API's stream; `api.Auth`, `api.Roles` and `api.MayRemove` are who may do what, for the pages and the API alike |
| `db/` | `schema.sql`, the tables as they are now, and `queries.sql`; charter writes `migrations/` and `queries.gen.go` from them |
| `seed.go`, `api/fixtures.go` | What the local database starts with, and what the tests know is there: three people, one of each role, with a token each (`ch_seed_ada`, `ch_seed_grace`, `ch_seed_lin`), and their notes |
| `built/` | What the app does with what Cloudflare gives a Worker, one job each, through workers-go's own packages: a file attached to a note (R2), what happened (Queues, a Cron Trigger, D1, followed live through a Durable Object), a notice for everyone (KV), mail, a rate limit and the cache, a request to another site |
| `locales/` | English, German, Arabic |
| `docs/` | The app's own docs, as Markdown |
| `pages/screens/` | charter's screens (`charter add screen:<name>`): the docs' pages, the language list and offer, the time zone dialog, the theme switch |
| `test/` | `page_*_test.go`: the pages in a real browser (a note typed in one tab shows in another; Arabic is right to left; a card per Cloudflare binding). `api_test.go`: the API through `sdk/go`. `test/devloop` works on the app under `charter dev` |
| `sdk/go/` | The Go client, generated from the contract |

Generated, never edited. Built each time and not in git: `*.x.go`, `*.built.go`, `.charter/`. Committed and checked:
`db/migrations/`, `db/queries.gen.go`, `fern/` and `sdk/go/` (written by `check --fix` and `pack -publish`).
`ui/` and `web/gsxui/` are gsxui's components, vendored by `charter add ui:<name>`.
