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
| `db/` | The migrations and the queries, typed |
| `cloud/` | One file per thing Cloudflare gives a Worker, through workers-go's own packages |
| `locales/` | English, German, Arabic |
| `test/` | Tests of the Worker: the API through `sdk/go`, the feed, a test per Cloudflare card. `test/devloop` works on the app under `charter dev` |
| `sdk/go/` | The Go client, generated from the contract |
