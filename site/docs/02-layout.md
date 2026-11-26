# An app's layout

charter finds everything by where it is. There is nothing to configure for any of this.

```
pages/         the page tree and its components
pages/screens/ charter's screens: the docs' pages, the languages, the theme switch
api/           contract.go: the API
db/            schema.sql, queries.sql, seed.sql
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

## main.go

```go file=../example/main.go#main
func main() {
	// What the Worker does that is no request: a Cron Trigger, a queue's consumer, email (package bindings).
	bindings.Background()
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
		Args:   []any{api.Auth},
		// What a page asks for by type, made from the request: the notes' store (D1), the bindings' state, and who
		// is asking.
		RequestArgs: []any{api.OpenStore, bindings.Read, api.Auth.Caller},
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
