# An app's layout

charter finds everything by where it is. There is nothing to configure for any of this.

```
pages/         the page tree and its components
pages/screens/ charter's screens: the docs' pages, the languages, the theme switch
api/           contract.go: the API
db/            schema.sql, queries.sql; the migrations charter writes from them
env/           what the Worker is bound to, as Go: written by charter from wrangler.jsonc, never by hand
seed.go        what a new database starts with, and what the tests know is there
fnox.toml      the deployed Worker's secrets: their names and who keeps them, never a value
docs/          the app's docs, as Markdown
locales/       <language>.toml: the languages are the files here
web/  public/  what the browser loads
ui/            gsxui's components
test/          tests of the pages and of the API
sdk/go/        the app's own Go client
wrangler.jsonc the Worker and its bindings, without ids
charter.toml   only url, and what Fern should generate
main.go
```

## What the Worker is bound to

A binding is declared once, in `wrangler.jsonc`. charter writes each as a Go value in the app's `env` package, under the name it has there, with the type of its kind:

| Declared in `wrangler.jsonc` as | In Go | What you do with it |
|---|---|---|
| `d1_databases`: `DB` | `env.DB` | `env.DB.Open()`: the database, for [the queries](07-database.md#queries) |
| `kv_namespaces`: `KV` | `env.KV` | `env.KV.Open()`: workers-go's `*kv.Namespace` |
| `r2_buckets`: `BUCKET` | `env.BUCKET` | `env.BUCKET.Open()`: workers-go's `*r2.Bucket` |
| `queues.producers`: `QUEUE` | `env.QUEUE` | `env.QUEUE.Open()`: workers-go's `*queues.Producer` |
| `durable_objects.bindings`: `HUB` | `env.HUB` | `live.Feed[Note]{Binding: env.HUB, …}` |
| `ratelimits`: `WRITE_LIMIT` | `env.WRITE_LIMIT` | `env.WRITE_LIMIT.Allow(key)`; `api.Limit{Binding: env.WRITE_LIMIT}`. It carries the calls and the seconds `wrangler.jsonc` says, so they are said once |
| `send_email`: `MAIL` | `env.MAIL` | `env.MAIL.Send(from, to, raw)` |
| `vars`: `APP_NAME` | `env.APP_NAME` | `env.APP_NAME.Value()` |
| a secret named in `.dev.vars.example` or `fnox.toml` | `env.ADMIN_TOKEN` | `env.ADMIN_TOKEN.Value()`; `api.Token{Secret: env.ADMIN_TOKEN}` |

Why it is this way: a name written as text (`kv.NewNamespace("KV")`) compiles whatever it says, and fails on a Worker when nothing is bound there or something of another kind is. A value cannot be wrong like that:

- **What is not declared does not exist.** Take a bucket out of `wrangler.jsonc` and every line that used `env.BUCKET` stops compiling.
- **A kind is a type.** `api.D1Accounts(env.KV)` does not compile: it takes a database.
- **No value can be written by hand.** `api.D1Accounts("DB")` does not compile either.

The file is `env/env.built.go`. `dev` writes it again when `wrangler.jsonc`, `.dev.vars.example` or `fnox.toml` is saved, and `build`, `check` and `test` write it too; it is not committed. `charter check` fails an app that names a binding the old way, with the file and the line.

A binding's name must be one Go can have, starting with a capital letter (`DB`, `WRITE_LIMIT`), since it is its name in Go too. What workers-go gives that has no name of the Worker's (the Cache API, `fetch`, Cron Triggers) is used as workers-go's docs say.

What this does not tell you: whether the deployed Worker has the thing. `charter deploy` makes what `wrangler.jsonc` names; `charter run resources` and `charter run secrets` say what is there.

## main.go

```go file=../example/main.go#main
func main() {
	// What the Worker does that is no request: a queue's consumer, a Cron Trigger's task, email (package built).
	built.Background()
	(&app.App{
		Name:  "charter-example",
		Dist:  dist,
		Pages: pages.Mount,
		// The app's own docs (docs/*.md), shown by charter's docs pages, which the tree mounts at /docs.
		Docs: &app.Docs{
			Name:    "charter-example",
			Summary: "charter's example: notes that are live on every open page, an API from a contract, and a card for everything Cloudflare gives a Worker.",
			Pages:   docs.Pages,
			Repo:    "https://github.com/joeblew999/charter",
			Dir:     "example",
			Agents:  "A coding agent working on this app uses the Claude Code plugins of the marketplace joeblew999/charter: charter, gsx, gsxui and structpages. The app's .claude/settings.json enables them.",
		},
		// Who may do what, for the pages as for the API (api.Auth): a page or a group says what it needs with a Needs
		// method, and whoever is not signed in is sent to the sign-in screen.
		Auth:   api.Auth,
		SignIn: screens.SignInPage{},
		// The app's own lines on the admin screen's overview: asked when that page is.
		Facts: []app.Fact{
			{Label: locales.FactsNotes, Value: func(ctx context.Context) (any, error) {
				report, err := api.Report(ctx)
				return report.Notes, err
			}},
			{Label: locales.FactsWriters, Value: func(ctx context.Context) (any, error) {
				report, err := api.Report(ctx)
				return report.Authors, err
			}},
		},
		Args: []any{api.Auth},
		// What a page asks for by type, made from the request: the notes' store (D1), and who is asking.
		RequestArgs: []any{api.OpenStore, api.Auth.Caller},
		API:         api.Handler(api.Env{Var: charter.Var}),
	}).Run()
}
```

That one value is the pages, the API, the docs, Vite's files, a URL per language, `robots.txt`, the sitemap and `/llms.txt`.

## What is generated

Two kinds, told apart by name.

| Kind | Files | In git |
|---|---|---|
| Built from your sources each time | `*.x.go`, `*.built.go`, `.charter/`, `build/`, `dist/` | no |
| Read from the repository by something | `*.gen.go`, `db/migrations/`, `fern/`, `sdk/go/` | yes, and `check` holds each to its source |

`dev`, `build` and `check` write the first kind, so a fresh clone needs only `go tool charter check`. `check --fix` writes the second.

## What is never in git

`.dev.vars`, the local secrets wrangler dev reads. charter makes it from `.dev.vars.example`.
