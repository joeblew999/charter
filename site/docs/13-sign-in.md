# Signing in, and who may do what

An app has one answer to "who is asking, and may they?", for its pages and for its API alike.

## One caller

```go file=../example/api/handlers.go#Auth
// Auth is that, for the whole app: the API's operations are checked by it (API), and so are the pages (main.go
// gives it to app.App). So a page that needs "write" and the operation that needs "write" ask the same thing of
// the same caller.
var Auth = charter.NewAuth(cloud.Var, Trusted...)
```

`api.Auth` works out who a request comes from, whatever it carries:

| The request has | The caller is |
|---|---|
| The session cookie of someone who signed in on a page | That person |
| `Authorization: Bearer` with a token that person made | That person, with the token's scopes |
| `Authorization: Bearer` with one of the Worker's secrets | A program, with that secret's scopes |
| Cloudflare Access's token, or an OpenID Connect issuer's | Whoever those vouch for |

The API's operations are checked by it, and so are the pages: `main.go` gives the same `Auth` to both.

## Saying what is needed

An operation says it in the contract. A page, or a group of pages, says it with a method, in the same words:

```go
// api/contract.go
Security: charter.Needs("write"),

// pages/pages.go
func (PostNote) Needs() []string { return []string{"write"} }
```

`Needs` on a group of the tree covers every route under it. A group that returns `nil` needs someone signed in, whoever they are.

| The caller | An operation answers | A page answers |
|---|---|---|
| Is nobody | 401 | Sent to the sign-in page, then back to where they were going |
| May not | 403 | 403 |
| May | The result | The page |

A page that shows something of the caller declares them, as it declares the store:

```go file=../example/pages/pages.go#Home.Props
// Props loads what the request wants, as structpages has it. A row's button asks for that row alone (its target is
// Home's Item, with the note's id as its key): then one note is read, and structpages renders Item with it. Anything
// else is the page, with the newest notes. Each component takes its share of the results by their types.
func (p Home) Props(r *http.Request, store api.Store, caller charter.Caller, target structpages.RenderTarget) (HomeProps, api.Note, Open, error) {
	if target.Is(p.Item) {
		id, err := strconv.ParseInt(structpages.KeyOf(target), 10, 64)
		if err != nil {
			return HomeProps{}, api.Note{}, false, app.NotFound("no such note")
		}
		note, err := store.Get(r.Context(), id)
		if err != nil {
			return HomeProps{}, api.Note{}, false, app.NotFound("no such note")
		}
		return HomeProps{}, note, r.URL.Query().Get("open") != "", nil
	}
	notes, err := store.Before(r.Context(), math.MaxInt64, shown)
	if err != nil {
		return HomeProps{}, api.Note{}, false, err
	}
	props := HomeProps{Notes: notes, MayWrite: caller.May(PostNote{}.Needs()...)}
	if len(notes) > 0 {
		props.After = notes[0].ID
	}
	return props, api.Note{}, false, nil
}
```

In a component, `caller.May("write")` decides what to show: the form, or the way to sign in. Hiding a button protects nothing; the check on the route does.

## Signing in

People sign in with a passkey: the device keeps the key and asks its owner (a finger, a face, a PIN). There is no password anywhere.

```sh
go tool charter add screen:sign-in
```

That brings the sign-in page, the account page and their routes, built from your app's gsxui components, and writes four tables into `db/schema.sql`: users, their passkeys, their tokens, and sign-ins that were started.

```go
// pages/pages.go
signIn  screens.SignIn  `route:"/sign-in SignIn"`
account screens.Account `route:"/account Account"`

// main.go
Auth:        api.Auth,
SignIn:      screens.SignInPage{},
Args:        []any{api.Auth},
RequestArgs: []any{api.OpenStore, api.Auth.Caller},
```

`<screens.AccountLink/>` in a header says who is signed in, or leads to signing in.

Someone signed in has a session: a token the app issued, kept by their browser as a cookie that scripts cannot read. On their account page they make tokens for their programs, each with a name; a token is shown once, and the database keeps only its SHA-256. A program sends one as `Authorization: Bearer`, and is that person.

A passkey belongs to a host name. Under `charter dev` that is `localhost`; an IP address has none.

## Testing it

| Test | How it has a caller |
|---|---|
| A page's logic (`pages/pages_test.go`) | The test hands the page a caller, as it hands it a store |
| A page in a browser | `page.SignUp(path, "Ada")`: Chrome's own virtual authenticator makes the passkey, in a fraction of a second |
| The API | A bearer token: one of the Worker's secrets |

The example holds both sides to one rule in `test/page_signin_test.go`: nobody is refused by the page's form and by the API's `createNote`; someone signed in is let through by both, and so is the token they made.

## The clients

Every client Fern generates is run against the Worker itself (`go tool charter pack -prove`), by the program in `test/sdk/<language>/`: a token nobody issued is refused, a token that may only read may not write, a token that may write does, and what it wrote is read back. Go, Python, TypeScript and the command-line client each prove it in their own language.

## What is left to you

- **A lost device.** A passkey is on a device. Until the app sends email to anyone, the answer is a second passkey.
- **Roles.** A person has scopes, set when they sign up (`api.People`'s `Scopes`). Who gets which is your app's to decide.
