# Upgrading

```sh
go tool charter upgrade
```

That moves the app to charter's latest, the library and the command together, and runs the new `check --fix`: what charter writes for an app (its screens, its generated files, its tool versions) follows by itself.

What you wrote does not, when a release changes how something is written. This page lists those changes, newest first: what it was, what it is now, and what tells you. In every case below something does: the compiler or `charter check` names the file and the line. Nothing here fails silently on a Worker.

charter is before 1.0: a release may change how an app is written, and says so here. This page starts at 0.63; it does not cover what changed before that.

## 0.78

Nothing of yours has to change. One line in `wrangler.jsonc` turns on what is new, and `charter create` writes it for a new app.

- **Cloudflare keeps the pages that are the same for every reader**, and answers with them before the Worker runs: add `"cache": { "enabled": true }` to `wrangler.jsonc`. The docs and the theme editor are kept from then on. A page of yours is when its type has `Shared() bool` ([a shared page](04-pages.md#a-page-that-is-the-same-for-every-reader)).
- **Every answer says who may keep it.** What says nothing is `private` now, the API too. An answer of yours that sets its own `Cache-Control` keeps it.
- **A shared page gives no cookie**: the language of the last page viewed is no longer remembered from the docs. A language the reader chose still is.
- **The browser reports its time zone only from a page that shows a time**, where every page did: a reader of pages that show none brings no cookie, and is given the kept copy.
- **An app with one language no longer varies by the browser's languages** (`Vary: Accept-Language`): there is none to offer.

## 0.77

Your pages change where they wrote out what is now said once.

- **A page's heading is the layout's**: `<screens.Layout … heading={…} about={…}>`. A page has no `<h1>` of its own, and no card whose title is the page's.
- **What a page is made of is in the layout screen**: `screens.Panel` (a titled card), `screens.Rows` and `screens.Row` (a list and its lines), `screens.None`, `screens.Once`, `screens.Tabs`. charter's screens are made of them; yours can be, in place of a `ui.Card` with its header and content written out and a `<li>` with its classes.
- **An error is a page of the app**, in its layout, for a reader who asked for a page; a program still gets its words.
- **The account asks before it is deleted in gsxui's `Dialog`**, where it had the browser's own disclosure; the people's page has the admin's tabs.

## 0.76

Two lines of yours change: what is around your pages is said once.

- **Every page has the same header and footer.** The app says them once, and no page or screen passes its own:

  | Was | Is |
  |---|---|
  | `<screens.Layout mode={…} title={…} description={…} brand={…} links={…} tools={…} footer={…}>` on each page | `screens.Around.Links, screens.Around.Tools, screens.Around.Footer = Links(), Tools(), Footer()` in a `func init()`, once, and `<screens.Layout mode={…} title={…} description={…}>` on each page |

  The compiler names each page that still passes them.
- **`LayoutPage`** is a page of the app at one width, the same for all: your lists and forms, and charter's sign-in, account and admin pages. `LayoutMarketing` is a front page.
- **No screen has a way back of its own** (the account's, the admin's): the app's name in the header is the way to the front page everywhere. The docs' link to the repository is at the foot of a page, beside "Edit this page".

## 0.75

Nothing to change in an app's code, unless it took 0.74.

- **The layout is of gsxui's components**: its `Sidebar` for a page of docs (a drawer on a phone) and its `Sheet` for the header's links on a phone and for "On this page". 0.74 had copied the shell of gsxui's own website instead, which on a phone had the pages in a small popover and no "On this page"; that is gone. `check --fix` writes the screens again.
- **A page of the docs is put in place by a link again**, with no page load.

## 0.74

A page of yours that writes its own `<!DOCTYPE html>` moves into the layout.

- **One document around every page: the `layout` screen**, with three modes (a centred page, a page of docs, a workspace). `check --fix` adds it and moves charter's screens onto it. Your own pages move by hand, and `check` names each that has not:

  | Was | Is |
  |---|---|
  | `<!DOCTYPE html>`, `<html>`, `<head>`, `<body>`, a `<header>` and a `<main>` with widths of your own | `<screens.Layout mode={screens.LayoutMarketing} title={…} description={…} brand={<screens.LayoutBrand/>} links={…} tools={…} footer={…}>` around the page |

  `links` are the ways to your pages: in the header where there is room, in a drawer where there is not, so nothing in them has an id. `tools` are what the header always has.
- **`apptest`'s `Page` has `Wide(width)` and `Fits()`**: a test that no page is wider than its window, at a phone's width and a desk's.

## 0.73

Nothing to change in an app's code.

- **A docs page looks as gsxui's own docs do**, by gsxui's own stylesheet for them (`web/gsxui-site/site.css`, which `check --fix` writes and adds to `web/main.js`) and gsxui's `Table`, which `check --fix` adds. charter's own styling of a page is gone. Commit what `check --fix` writes.
- **`app.Doc` has `Sections`** where it had `HTML`: a page's parts under each heading, a table as its head and rows. Only a docs screen you changed yourself reads it; `check` says which.

## 0.72

One line to change in each page that uses gsxui's components.

- **A reader chooses a style too.** The app has its components in every one of gsxui's eight styles: `check --fix` writes the other seven into `ui/styles/<style>/` with gsxui's own `apply`, and `ui/styles` itself, a package named `ui` that renders the reader's. Commit all of it. In each `.gsx` of yours that imports the components, take them from there:

  | Was | Is |
  |---|---|
  | `"example.com/app/ui"` | `ui "example.com/app/ui/styles"` |

  `check` names each file that still has the first. charter's screens are changed by `check --fix`. Nothing else in a page changes: `<ui.Button>` stays `<ui.Button>`.
- **`charter add ui:<name>`** writes a new component in every style. `gsxui add` alone fails once a page uses the component.
- **A page's answer varies by cookie** (`Vary: Cookie`): the style is in one.

## 0.71

Nothing to change in an app's code, unless it took 0.70's theme picker.

- **gsxui's theme editor is a screen** (`theme`), and the docs screen brings it. `check --fix` adds it, writes gsxui's editor into the app (`pages/screens/theme-editor.gen.html`, `pages/screens/theme_editor.gen.go`, `web/gsxui-site/`), adds two npm packages gsxui's scripts import (`css-tree`, `jsonc-parser`) and a line to `web/main.js`. Mount it, and commit all of it:

  | In | Add |
  |---|---|
  | the page tree | ``theme screens.Theme `route:"/theme Theme"` `` |
  | a header | `<screens.ThemeLink/>` beside `<screens.ThemeToggle/>` |

- **0.70's `theme-picker` screen is gone**, with `web/gsxui/themes.css`: it was one of the editor's eight parts. In an app that has them, delete `pages/screens/theme-picker.gsx` and `web/gsxui/themes.css`, take `import "./gsxui/themes.css";` out of `web/main.js` and `screens.theme_own` out of the catalogs, and write `<screens.ThemeLink/>` where `<screens.ThemePicker/>` was. The compiler and `check` name each.
- **`charter run ui:theme --style <name>`** switches to another of gsxui's eight styles and keeps the app's colours, radius and fonts.

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
