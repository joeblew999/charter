# Tests

Tests are of the pages and of the API. Two kinds: a page as plain Go, in a fraction of a second, and the whole app on a real Worker.

## A page, as plain Go

A page declares what it needs, so a test hands it its own. No Worker, no database, no browser.

```go file=../example/pages/pages_test.go#TestHomeProps
// The home page's data is the newest notes first.
func TestHomeProps(t *testing.T) {
	store := &notes{}
	store.Create(context.Background(), "first", "u1", "Ada")
	store.Create(context.Background(), "second", "u1", "Ada")
	props, _, _, _, _, _, err := Home{}.Props(httptest.NewRequest("GET", "/", nil), store, writer, page{})
	if err != nil || len(props.Notes) != 2 || props.Notes[0].Body != "second" {
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

The local Worker starts on a database of its own, made new, filled by the app's [seed](07-database.md#seeding) and thrown away: a test can count on the seed's people and notes, and changes nothing you have. The deployed Worker's database is real, which is why a test leaves it as it found it ([below](#a-test-leaves-the-worker-as-it-found-it)), and why a test that needs the seed's tokens runs only locally (`apptest.Local()`): a deployed database never has them.

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

## A test leaves the Worker as it found it

The same tests ask the deployed Worker, whose database is real. So a test takes away what it makes:

- **People.** `page.SignUp` deletes the account again when the test is done, on the account page's own form, and with it what the app keeps of that person. A test that signs up by hand calls `page.Leave(path)`.
- **Rows.** A test that writes through the API takes the row away in `t.Cleanup` (the example's `write` helper).

`page.JS(expression)` evaluates in the page, for what a test cannot do as a reader does: the example stands in for the device's share sheet with it, and checks what the Share button hands over.

The example holds itself to this in `test/main_test.go`: a local Worker starts with the seed, and when the tests are done it must have the seed's people and notes and no more, or the run fails and names what was left.

## Secrets a test needs

A test that must be someone asks for a secret of the Worker by name: `apptest.Secret(t, "ADMIN_TOKEN")`. Locally that is the throwaway value in `.dev.vars`. For the deployed Worker, `go tool charter test --deployed` hands the tests each secret fnox has for it (see [deploying](10-deploy.md)); a test whose secret is missing is skipped, and charter says which.

## What else `charter test` does

The standard test of an API with a contract, and the search-engine audit.
