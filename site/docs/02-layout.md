# An app's layout

charter finds everything by where it is. There is nothing to configure for any of this.

```
pages/         the page tree and its components
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

```go
func main() {
	(&app.App{
		Name:  "My App",
		Dist:  dist,
		Pages: pages.Mount,
		API:   api.Handler(api.Env{Var: cloud.Var}),
		Docs:  &app.Docs{Name: "My App", Pages: docs.Pages},
		// What a page's Props may ask for, made from the request.
		RequestArgs: []any{api.OpenStore},
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
