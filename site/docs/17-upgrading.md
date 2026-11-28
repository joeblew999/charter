# Upgrading

```sh
go tool charter upgrade
```

That moves the app to charter's latest, the library and the command together, and runs the new `check --fix`: what charter writes for an app (its screens, its generated files, its tool versions) follows by itself.

What you wrote does not, when a release changes how something is written. This page lists those changes, newest first: what it was, what it is now, and what tells you. In every case below something does: the compiler or `charter check` names the file and the line. Nothing here fails silently on a Worker.

charter is before 1.0: a release may change how an app is written, and says so here. This page starts at 0.63; it does not cover what changed before that.

## 0.69

Nothing to change in an app's code. The docs screen is made of gsxui's `Sidebar`, `Breadcrumb`, `Sheet` and `Item`, which `check --fix` adds to `ui/` as it brings the screen up to date.

- **A message is gone.** An app whose docs screen was brought up to date by 0.68 has `screens.docs_menu` in its catalogs, which nothing uses now: `check` says so, and the line is to be taken out of each `locales/*.toml`.
- **A docs screen you changed** is yours and is left as it is: `go tool charter add screen:docs -diff` shows what charter's is now.
- **The groups and the order of the docs are their front page's** (since 0.68): a line of `docs/index.md` that is only bold names a group, and the list under it its pages ([groups, and the order](11-docs.md#groups-and-the-order)). With no such lines the pages are in their files' order, as before.

## 0.65

Nothing to change in an app's code. Three things behave differently.

- **A page reached by a link is a new page.** Its body takes the place of the one before, where 0.61 to 0.64 morphed it in and left a menu or a dialog that was open still open over the next page. If you relied on something in the body staying across a link (a value typed into a field outside a form, an element you changed with a script), it no longer does: it is as if the page had loaded.
- **`charter test` follows every link** a page has to the app, and fails on one that leads nowhere. A link in your docs to a page you removed is now an error.
- **Three pages are added to your docs**, written from the app: `/docs/reference-api`, `/docs/reference-database` and `/docs/reference-bindings` ([what charter writes](11-docs.md#what-you-write-and-what-charter-writes)). A file of yours in `docs/` that would be at one of those addresses stops the build, which says which: give it another name.

Worth doing while you are here, though nothing requires it: a test that opens a browser can say `t.Parallel()`, which took the example's suite to a third of its time. A test that takes "the first row" for its own, or counts what is on a page, then fails; [tests](09-tests.md) says what to do instead.

## 0.64

Two things an app names are no longer text. Both changes are mechanical: the compiler stops at each line, and the table says what to write.

### A binding, a variable, a secret's name

Each is a value in the package `env`, which charter writes from `wrangler.jsonc`, `.dev.vars.example` and `fnox.toml` ([what the Worker is bound to](03-layout.md#what-the-worker-is-bound-to)). Import it as `"<your module>/env"`.

| It was | It is |
|---|---|
| `cloud.Open("DB")` | `env.DB.Open()` |
| `kv.NewNamespace("KV")` | `env.KV.Open()` |
| `r2.NewBucket("BUCKET")` | `env.BUCKET.Open()` |
| `queues.NewProducer("QUEUE")` | `env.QUEUE.Open()` |
| `cloud.Allow("WRITE_LIMIT", key)` | `env.WRITE_LIMIT.Allow(key)` |
| `cloud.SendEmail("MAIL", from, to, raw)` | `env.MAIL.Send(from, to, raw)` |
| `cloud.Var("APP_NAME")`, `cloudflare.Getenv("APP_NAME")` | `env.APP_NAME.Value()` |
| `live.Feed[Note]{Binding: "HUB", …}` | `live.Feed[Note]{Binding: env.HUB, …}` |
| `charter.Limit{Binding: "WRITE_LIMIT", Limit: 100, Period: 60}` | `charter.Limit{Binding: env.WRITE_LIMIT}`: the calls and the seconds are `wrangler.jsonc`'s |
| a constant of your own that repeats a limit (`LimitCalls = 100`) | `env.WRITE_LIMIT.Calls()`, `env.WRITE_LIMIT.Seconds()` |
| `charter.Token{Secret: "ADMIN_TOKEN", …}` | `charter.Token{Secret: env.ADMIN_TOKEN, …}` |
| `charter.D1Accounts("DB")` | `charter.D1Accounts(env.DB)` |
| `app.Seed("DB", …)` | `app.Seed(env.DB, …)` |
| `apptest.Secret(t, "WRITE_TOKEN")`, in a test | `apptest.Secret(t, env.WRITE_TOKEN)` |

Who tells you: the compiler, for everything of charter's (`undefined: cloud.Open`, or "cannot use "DB" as cloud.Database"). `charter check`, in its part `bindings`, for workers-go's own (`kv.NewNamespace`, `cloudflare.Getenv`), which still compile.

Two things this may turn up, as it did in charter's own example:

- **A name your code reads that is declared nowhere.** `env.MAIL_UNLIMITED` does not exist until `MAIL_UNLIMITED` is in `wrangler.jsonc`'s `vars` (empty is fine), or among the secrets.
- **A binding's name that is no name in Go** (`my-db`, `db`). It must start with a capital letter, then letters, digits and `_`: rename it in `wrangler.jsonc`.

### A scope

A scope is a value, declared once and named everywhere else ([saying what is needed](14-sign-in.md#saying-what-is-needed)).

| It was | It is |
|---|---|
| nothing: a scope was the word wherever it was used | `var Write = charter.NewScope("write")`, once, beside your roles |
| `map[string][]string{"member": {"read", "write"}}` | `map[string][]charter.Scope{"member": {Read, Write}}` |
| the scope `"admin"` | `charter.Admin`, which is charter's own |
| `func (PostNote) Needs() []string { return []string{"write"} }` | `func (PostNote) Needs() []charter.Scope { return []charter.Scope{api.Write} }` |
| `charter.Needs("write")` | `charter.Needs(Write)` |
| `caller.May("moderate")` | `caller.May(api.Moderate)` |
| `charter.Token{…, Scopes: []string{"read"}}` | `charter.Token{…, Scopes: []charter.Scope{Read}}` |
| `charter.Access{People: []string{"read"}, …}` | `charter.Access{People: []charter.Scope{Read}, …}` |
| `charter.Caller{…, Scopes: []string{"read", "write"}}`, in a test | `charter.Caller{…, Scopes: []charter.Scope{api.Read, api.Write}}`, or a role's: `Scopes: api.Roles["member"]` |
| `charter.People{…, Manage: "admin"}` | leave `Manage` out: it is `charter.Admin` |
| `Allow(r *http.Request, needs ...string)`, on an Auth of your own for a test | `Allow(r *http.Request, needs ...charter.Scope)` |

The list above is every kind of change charter's own example needed, taken from its history. Who tells you: the compiler, at every one ("cannot use "write" as Scope"). `charter check`, in its part `scopes`, where a scope is made with `NewScope` anywhere but its declaration.

One thing this may turn up: an app whose `People` has a `Role` that is none of its `Roles` no longer starts, and `charter check` fails as it writes the specs, with the role and the roles there are.

### Screens

A copy of the sign-in or the admin screen that you have not changed is brought up to date by `check --fix`. One you changed is yours: it will not compile until its `Needs` methods return `[]api.Scope` (`api.Admin` for the people and the admin pages), and where it shows a caller's scopes, `scope.String()`.

## 0.63

**No statement on the database is written in Go.**

| It was | It is |
|---|---|
| `cloud.Query[Row](db, "SELECT …", id)`, and `cloud.One`, `cloud.Exec`, `cloud.ExecRows` by hand | a query in `db/queries.sql` (`-- name: Something :one`), and the function `check --fix` writes for it: `db.Something(conn, id)` |
| `cloud.Batch(db, cloud.Stmt("DELETE …", id), …)` | `cloud.Batch(conn, db.ForgetNoteStmt(id), …)`: each query that returns no rows has a `…Stmt` function |
| `cloud.SQL("DB")`, workers-go's `database/sql` driver | a query, as above. `cloud.SQL` is gone: what went through it read the primary always and gave its reader no bookmark |

Who tells you: `charter check`, in its part `statements`, with the file and the line. The compiler, for `cloud.SQL`.

One thing this may turn up: a statement that never matched your schema. sqlc checks each query's columns and types against `db/schema.sql`, which nothing did for a statement written in Go.

`check` now also holds your `db/schema.sql` to the statements charter's sign-in runs on its tables (`users`, `passkeys`, `tokens`, `ceremonies`): you may add columns to them, and a column of theirs that you dropped, renamed or retyped is said, with the statement that needs it.

## When `upgrade` stops

`charter upgrade` ends with the new `check --fix`. When your code no longer compiles against the new release, that is where it stops, with the compiler's lines. Work through them with the tables above, then:

```sh
go tool charter check --fix
go tool charter test
```
