# How it is built

Every part of the app is in the folder charter looks for it in.

| Folder | What is in it |
|---|---|
| `pages/` | The page tree: the home page with its live list, the bindings page, and the routes they call |
| `api/` | The notes API's contract and handlers, and the one feed behind the page's list and the API's stream |
| `db/` | `schema.sql`, the tables as they are now, and `queries.sql` |
| `bindings/` | One file per thing Cloudflare gives a Worker |
| `locales/` | English, German, Arabic |
| `docs/` | These pages |
| `test/` | Tests of the pages in a real browser, and of the API through its own SDK |

## A page declares what it needs

```go
func (Home) Props(r *http.Request, store api.Store) (HomeProps, error) {
	notes, err := store.Before(r.Context(), math.MaxInt64, shown)
	…
}
```

The store is made from the request and handed over. So `pages/pages_test.go` tests the pages as plain Go, with a store of its own, in half a second.

## Running it

```sh
go tool charter dev
go tool charter check && go tool charter test
go tool charter deploy
```

Back to [the start](index.md).
