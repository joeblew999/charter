# The example

The one app that uses all of charter, and what `go tool charter create` copies. Deployed at
https://charter-example.gedw99.workers.dev.

`npm install` at the repo's top, then here:

```sh
go tool charter dev                              # leave it running → http://localhost:5173
go tool charter check && go tool charter test    # before a push
go tool charter deploy                           # then: go tool charter test --deployed
```

| Folder | What is in it |
|---|---|
| `pages/` | The page tree (structpages): the home page with its live list, the Cloudflare page, and the routes they call |
| `api/` | The notes API's contract and handlers; `api.Notes` is the one feed behind the page's list and the API's stream |
| `db/` | `schema.sql`, the tables as they are now, and `queries.sql`; charter writes `migrations/` and `queries.gen.go` from them |
| `cloud/` | One file per thing Cloudflare gives a Worker, through workers-go's own packages |
| `locales/` | English, German, Arabic |
| `test/` | `page_*_test.go`: the pages in a real browser (a note typed in one tab shows in another; Arabic is right to left; a card per Cloudflare binding). `api_test.go`: the API through `sdk/go`. `test/devloop` works on the app under `charter dev` |
| `sdk/go/` | The Go client, generated from the contract |

Generated, never edited: `*.x.go` and `locales/messages.gen.go` (written by `dev` and `build`, not committed);
`db/migrations/`, `db/queries.gen.go`, `fern/` and `sdk/go/` (written by `check --fix` and `pack -publish`, committed and checked).
`ui/` and `web/gsxui/` are gsxui's components, vendored by `charter add ui:<name>`.
