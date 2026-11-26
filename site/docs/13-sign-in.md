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
func (p Home) Props(r *http.Request, store api.Store, caller charter.Caller, target structpages.RenderTarget) (HomeProps, api.Note, Open, Removable, error) {
	if target.Is(p.Item) {
		id, err := strconv.ParseInt(structpages.KeyOf(target), 10, 64)
		if err != nil {
			return HomeProps{}, api.Note{}, false, false, app.NotFound("no such note")
		}
		note, err := store.Get(r.Context(), id)
		if err != nil {
			return HomeProps{}, api.Note{}, false, false, app.NotFound("no such note")
		}
		return HomeProps{}, note, r.URL.Query().Get("open") != "", Removable(api.MayRemove(caller, note)), nil
	}
	notes, err := store.Before(r.Context(), math.MaxInt64, shown)
	if err != nil {
		return HomeProps{}, api.Note{}, false, false, err
	}
	props := HomeProps{Notes: notes, MayWrite: caller.May(PostNote{}.Needs()...), Caller: caller}
	// One read of KV, for every reader: what it is for.
	if props.Notice, err = built.NoticeNow(r.Context()); err != nil {
		return HomeProps{}, api.Note{}, false, false, err
	}
	if len(notes) > 0 {
		props.After = notes[0].ID
	}
	return props, api.Note{}, false, false, nil
}
```

In a component, `caller.May("write")` decides what to show: the form, or the way to sign in. Hiding a button protects nothing; the check on the route does.

## Roles

What people may be is said once, as names and what each may do. A person has one role; someone who signs up gets the first you name.

```go file=../example/api/handlers.go#Roles
// Roles are what the app's people may be, and what each may do. Someone who signs up is a member. A moderator
// may also take anyone's note away; an admin may also say who is what.
var Roles = map[string][]string{
	"member":    {"read", "write"},
	"moderator": {"read", "write", "moderate"},
	"admin":     {"read", "write", "moderate", "admin"},
}
```

A role changes for everyone who has it when you change it here, and a person's changes at their next request: in the session they have open, and for the tokens they made before.

A rule that is about more than a scope is a function, asked by the page and by the API alike:

```go file=../example/api/handlers.go#MayRemove
// MayRemove is the rule for taking a note away, said once: whoever wrote it may, and so may whoever may moderate.
// The page asks it to show the button, the page's route and the API's deleteNote ask it before they act.
func MayRemove(who charter.Caller, note Note) bool {
	return who.May("moderate") || (who.SignedIn() && who.Subject != "" && who.Subject == note.AuthorID)
}
```

In the example a note says who wrote it. Its writer may take it away, and so may a moderator: the page shows the button to exactly those, and the page's route and the API's `deleteNote` ask the same function before they act.

Who is what is an admin's to say: on the people page (from their account), and through the API (`listPeople`, `setRole`), so the generated clients and the command-line client can do it too.

### The first admin, and inviting people

Nobody is an admin until someone is invited to be. An invitation is a link: whoever signs up through it, once, within the hour, is what it says.

```sh
go tool charter run people:invite admin              # under charter dev
go tool charter run people:invite admin --deployed   # on the deployed Worker
go tool charter run people                           # who signed up, and what each is
```

Open the link, give a name, and your device makes the passkey: you are signed in as an admin. Someone who has an account already, and is signed in, gets a button there instead: "Accept the invitation". From then on you invite others on the people page ("Invite someone"), to any role, or change the role of someone who signed up already.

An invitation is kept as a started sign-in is, by the SHA-256 of its code, so the database has nothing that invites anyone. A program does the same with `auth.Invite(r, role)`.

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

At the foot of the account page someone can delete their account: asked twice, the second time with what goes. Their passkeys, sessions and tokens go with it, and so does what your app keeps of theirs, which you say once:

```go
charter.People{…, Leaving: func(ctx context.Context, who charter.Person) error { … }}
```

The example takes away the notes they wrote, what the activity page recorded of those, and the files they attached. Only the person can do this, signed in on the page: a program with one of their tokens cannot.

A passkey belongs to a host name. Under `charter dev` that is `localhost`; an IP address has none.

## Testing it

| Test | How it has a caller |
|---|---|
| A page's logic (`pages/pages_test.go`) | The test hands the page a caller, as it hands it a store |
| A page in a browser | `page.SignUp(path, "Ada")`: Chrome's own virtual authenticator makes the passkey, in a fraction of a second |
| The API | A bearer token: one of the Worker's secrets |

The example holds both sides to one rule in `test/page_signin_test.go`: nobody is refused by the page's form and by the API's `createNote`; someone signed in is let through by both, and so is the token they made. A member may not remove another's note, on the page or through the API; made a moderator, they may, through both, at once.

## The clients

Every client Fern generates is run against the Worker itself (`go tool charter pack -prove`), by the program in `test/sdk/<language>/`: a token nobody issued is refused, a token that may only read may not write, a token that may write does, and what it wrote is read back. Go, Python, TypeScript and the command-line client each prove it in their own language.

## What is left to you

- **A lost device.** A passkey is on a device. Until the app sends email to anyone, the answer is a second passkey.
- **More than one role each, groups, teams.** A person has one role. Anything richer is your app's own tables and its own rule, written as `MayRemove` is.
