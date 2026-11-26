# Tests

Tests are of the pages and of the API, and they ask a real Worker.

```sh
go tool charter test             # the Worker on wrangler dev
go tool charter test --deployed  # the deployed one, with the same tests
```

## A page, in a real browser

```go
func TestANoteShowsOnEveryOpenPage(t *testing.T) {
	writer := apptest.Browser(t)
	reader := writer.Tab()
	writer.Open("/")
	reader.Open("/")
	writer.Type(`#post input[name=body]`, "hello")
	writer.Click(`#post button[type=submit]`)
	reader.WaitText("#notes", "hello")
}
```

`apptest.Browser` is Chrome on the Worker under test, so htmx, the feed and the scripts Vite built are part of what is tested.

## The API, through its own SDK

```go
note, err := sdk(t, option.WithToken(token)).Notes.Create(ctx, &notes.CreateNotesRequest{Body: "hi"})
```

The client is the one in `sdk/go`, generated from the contract: the same one another repository would import.

## Where a test runs

| Folder | Runs against |
|---|---|
| `test/` | both |
| `test/dev/` | a local Worker only |
| `test/deployed/` | a deployed one only |

A test that only one can answer asks `apptest.Local()`.

## What else `charter test` does

The standard test of an API with a contract, and the search-engine audit.
