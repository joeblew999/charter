# Tests

Tests are of the pages and of the API. Two kinds: a page as plain Go, in a fraction of a second, and the whole app on a real Worker.

## A page, as plain Go

A page declares what it needs, so a test hands it its own. No Worker, no database, no browser.

```go file=../example/pages/pages_test.go#TestHomeProps
// The home page's data is the newest notes first, and where its stream starts: after the newest.
func TestHomeProps(t *testing.T) {
	store := &notes{}
	store.Create(context.Background(), "first", "u1", "Ada")
	store.Create(context.Background(), "second", "u1", "Ada")
	props, _, _, _, err := Home{}.Props(httptest.NewRequest("GET", "/", nil), store, writer, page{})
	if err != nil || len(props.Notes) != 2 || props.Notes[0].Body != "second" || props.After != 2 {
		t.Fatalf("%+v %v", props, err)
	}
}
```

These live beside the pages (`pages/pages_test.go`) and run with `go test`, which `charter test` does first.

## The whole app, on a real Worker

```sh
go tool charter test             # the Worker on wrangler dev
go tool charter test --deployed  # the deployed one: the same tests, and the audit
```

## A page, in a real browser

```go file=../example/test/page_home_test.go#TestANoteShowsOnEveryOpenPage
// A note typed into one open page shows at once in another: the form, the feed, htmx's SSE swap.
func TestANoteShowsOnEveryOpenPage(t *testing.T) {
	writer := apptest.Browser(t)
	reader := writer.Tab()
	// Writing is for whoever is signed in; reading is anyone's.
	writer.SignUp(at(screens.SignInPage{}), "Writer")
	writer.Open("/")
	reader.Open("/")
	body := "typed at " + time.Now().Format(time.RFC3339Nano)
	writer.Type(el(pages.Home.Form)+` input[name=body]`, body)
	writer.Click(el(pages.Home.Form) + ` button[type=submit]`)
	reader.WaitText(el(pages.Home.List), body)
	writer.WaitText(el(pages.Home.List), body)
	if left := writer.Attr(el(pages.Home.Form)+` input[name=body]`, "value"); left != "" {
		t.Errorf("the form still holds %q after posting", left)
	}
}
```

`apptest.Browser` is Chrome on the Worker under test, so htmx, the feed and the scripts Vite built are part of what is tested. `el` asks the app's own tree for a component's selector (`test/tree_test.go`): a renamed component breaks the test when it is compiled, not when a selector quietly matches nothing.

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
