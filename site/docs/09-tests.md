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
	t.Parallel()
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

A browser is Chrome, and it is what the time goes on: of the example's suite, the tests that open one take nine tenths of it, about a second each, and the forty that do not take a few hundredths. So:

- **A test that opens a browser says `t.Parallel()`** as its first line. They then run beside each other, and the suite takes a third of the time (the example's: 29 s to 11 s on a local Worker, 85 s to 22 s on the deployed one).
- **A test finds its own rows.** Others write while it runs, so "the first row of the list" is not its note. It asks the tree for the row by the note's id (`row(note.ID)` in `test/tree_test.go`), and never counts what is on a page.
- **A test waits for the thing, not for a while.** `WaitText` and its like ask until it is there. A `time.Sleep` that is long enough on your machine is too short on a deployed Worker that is starting.

## What to test, and what not to

A test is for what running the app shows and nothing else does.

| Do not test | Because |
|---|---|
| That a link, an id or a target reaches its page | the tree and structpages-lint say so when the app is compiled and checked, and `charter test` follows every link |
| That a statement names real columns, with the right types | sqlc checks it against the schema |
| That a binding, a variable or a scope exists | it is a Go value: one that is not there does not compile |
| That the API's spec matches the code | `charter check` compares them |
| What charter's own screens do (the docs' pages, their search, their menu) | charter tests them once, on its own site |

| Do test | Because |
|---|---|
| Who may do what, signed in as each role, on the page and through the API | whether a moderator should have a scope is your decision, and only a request shows it |
| What a binding does: a file kept and given back, a queue's message consumed, an email sent | the name is checked; what Cloudflare does with it is not |
| What a reader sees happen: a list that follows a feed in a second tab, a form refused in place, a menu that opens | it is behaviour in a browser |
| The API through its generated SDK | a client is proven by calling the Worker, never only by compiling |

## The API, through its own SDK

```go
note, err := as.Notes.Create(t.Context(), &notes.CreateNotesRequest{Body: body})
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
