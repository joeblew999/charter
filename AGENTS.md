# Rules for agents

How the repo works: the docs, which are `site/docs/*.md`, served by `site/` at https://charter.gedw99.workers.dev/docs/. Plans and open work are GitHub issues here, never files.

- **One way.** Cloudflare only (wrangler dev locally, Cloudflare remotely), gsx and structpages only, one example.
  Before adding an option, a setting or a second way to do something, don't: make the one way better.
- **Conventions, not settings.** charter finds an app's parts by where they are (`pages/`, `api/`, `db/`,
  `locales/`, `test/`). `charter.toml` holds only what a folder cannot say.
- **Nothing runs off Cloudflare.** No native server, no stand-in for a binding. Library packages still compile for
  the machine they are written on (an `other.go` whose calls answer `cloud.ErrNotCloudflare`), so editors, `go vet`
  and the program that writes an API's specs need no setup. Plain logic is tested with `go test`; anything that
  touches a binding is tested through the example, on a Worker.
- **Drive the upstream tool, never copy it.** wrangler runs and deploys the Worker, Vite builds the browser's files,
  gsx generates and runs the dev loop, structpages routes the pages, workers-go carries Go. Read the upstream guide
  before designing anything.
- **workers-go is used the workers-go way:** everything it has is imported and called directly. charter adds only
  what it lacks (`cloud`), and says so where it is defined.
- **A binding's name is written once, in `wrangler.jsonc`**, a variable's there too, a secret's in
  `.dev.vars.example` and `fnox.toml`. In Go each is a value of its kind's type (`cloud/names.go`), written by
  charter into the app's `env/env.built.go` (`internal/envcode`): `env.DB.Open()`, `env.KV.Open()`,
  `env.WRITE_LIMIT.Allow(key)`, `env.APP_NAME.Value()`. Nothing in an app names one as text, and nothing in
  charter takes one as text: `charter check` fails an app that does (its part `bindings`).
- **mise is all a developer installs.** `mise.toml` pins Go, Node and fnox (`mise.lock` says what each download
  must be), `go.mod` and `package-lock.json` pin the rest, and Renovate keeps all of them current
  (`.github/renovate.json5`). CI installs the same with `.github/actions/tools`. `charter check` fails where other
  versions run, and raises an app's pins to at least charter's. go.mod's `go` line is the oldest Go an app may use,
  raised by hand. charter brings only Kiri (an app, not a mise tool). No mise tasks: commands are charter's. No
  TypeScript of ours: `vite.config.ts` is gsx's file.
- **We pick winners.** One tool per job, brought and driven by charter, so the hard parts are easy. Don't add a
  second choice or a way around one.
- **Six library folders at the top** (`app`, `api`, `cloud`, `live`, `intl`, `seo`), with `cmd`, `internal`,
  `example`, `site` and `skills` (the skill charter ships, as gsx and structpages ship theirs). Don't add another.
- **`example/` uses all of charter** and is what `charter create` copies. A feature of charter that the example
  does not use is not done; a bug the example cannot show is a missing part of the example.
- **`site/` is charter's own site**, a charter app: a front page, and the docs (`site/docs/*.md`; never write docs
  anywhere else). A change to how charter works changes the page of the docs that says so, in the same commit. The
  pages that show docs are the docs screen.
- **What an app has is never written in its docs by hand.** Its API, its database and what its Worker is bound to
  are pages charter writes from the spec, the schema and `wrangler.jsonc` (`internal/docsgen/reference.go`),
  shown as the pages of `docs/*.md` are. A person writes what an app is for. charter's own docs explain charter
  and point at the example's written pages for what an app has: they never say this site has what only the
  example has. A link in any page is followed by `charter test`, which fails on one that leads nowhere. Code in
  charter's docs is a line of the example, the site or the screens, never written for the page, and a command is
  one charter has: `go test ./...` fails otherwise (`internal/docsgen`, `cmd/charter`). Each page that is about
  something a developer writes opens with its commands, what is checked before the app runs, and what is not.
- **The look is gsxui's, all of it.** Its styles, its themes, its light and dark, by gsxui's own commands
  (`charter run ui:theme` is `gsxui apply`). charter has no colour, no theme setting, no stylesheet and no rule
  that styles an element from outside it (`[&_h2]:…`) of its own. A page lays itself out with Tailwind's classes
  on its own elements, as gsxui's site does; what a docs page's Markdown becomes is styled by gsxui's own
  stylesheet for its docs and shown with gsxui's `Table` (`internal/docsgen/look.go`):
  what a reader chooses with is gsxui's own theme editor, its markup, catalogue and scripts written into the app
  from gsxui (`internal/themes`, the theme screen), with the app as its preview by gsxui's own protocol. A style
  is markup, so an app has its components in all eight: its own in `ui`, the others in `ui/styles/<style>` by
  gsxui's own `apply`, and pages take them from `ui/styles`, which renders the reader's (`app.Style`). Never
  import `ui` itself in a page. What charter repeats of gsxui (the layout of a share code, for `--style`) a test
  holds to gsxui's source.
- **One document around every page**: the layout screen (`app/screens/layout.gsx.tmpl`), made of gsxui's
  components (`Sidebar`, `Sheet`), never of markup copied from gsxui's website: what `gsxui add` brings is what
  follows gsxui. A page names its mode and nothing else: never a width, a header, a way back or a
  `<!DOCTYPE html>` of its own (`charter check` fails one). The header and the footer are the app's, said once
  (`screens.Around`), and every page and every screen has them: a screen that shows something no other does
  (a back button, a header of its own) is a bug. What a page is made of is said once too, in the layout screen:
  its heading (`heading`, `about`), `Panel`, `Rows` and `Row`, `None`, `Once`, `Tabs`, and the page an error is.
  Before writing a `ui.Card` with its header, a `<li>` with classes or an `<h1>` in a page or a screen, use the
  part; when a part is missing, add it there and use it everywhere. What is in a header twice (in it, and under its
  menu on a narrow window) has no id. A test opens every page at a phone's, a tablet's and a desk's width and
  fails one that is wider than its window (`apptest`'s `Fits`).
- **What every app needs is a screen** (`app/screens`: the docs' pages, the theme switch, the language list and
  offer, the time zone dialog): a gsx template of gsxui components that `charter add screen:<name>` copies into an
  app, as gsxui copies a component, so it is built from the app's own components and theme. Change a screen in
  `app/screens`, then `go tool charter check --fix` in `example/` and `site/`, which brings their copies up to date.
  Nothing in the library imports gsxui.
- **structpages is used the structpages way.** A page's `Props` and a handler's `ServeHTTP` declare what they need
  (`app.App`'s `Args` and `RequestArgs`) and return errors (`app.NotFound`, `app.Status`, `app.Redirect`); a region
  that changes by itself is a component of its page, with its id and target from the tree. `url` is
  structpages' own `URLFor`: the tree is mounted once for each language with its prefix. Never a path, an id or a
  selector as a string, in pages or in tests.
- **A developer never has to remember to run something.** What an upstream tool generates or checks runs inside
  `dev`, `check`, `test` or `deploy`. What is left is a namespaced task of `charter run` (`intl:`, `seo:`, `db:`,
  `ui:`).
- **Tests are of the pages and of the API**: pages in a real browser (`apptest.Browser`), the API through its own
  SDK. A test is for what only running the app shows: never for what the compiler, `check` or the audit already
  say (a link, an id, a column, a binding, a scope), and never in an app for what charter's own screens do,
  which `site/test` tests once. A test that opens a browser says `t.Parallel()`, finds its own rows by their id
  from the tree, and waits for a thing, not for a time. Skills are Claude Code plugins: this repo is one and a marketplace (`.claude-plugin`, `skills/charter`) that also
  lists gsx, gsxui and structpages from their owners' repos. Never copy a skill's files.
- **Who may do what is said once, for pages and API alike.** One `api.Auth` and one `api.Caller`; a page's
  `Needs` and an operation's scopes are the same values, checked by the same code. A scope is a value
  (`app.Scope`), declared once and named everywhere else: nothing takes one as a word. People sign in with passkeys
  (the sign-in screen); a session and a personal token are the same kind of thing. A client in any language is
  proven by calling the Worker (`example/test/sdk`), never only by compiling.
- **The seed and the tests are one thing.** What fills a developer's database is typed values and a function that
  writes them through the app's own functions; tests call the same function and name the same values. A seed never
  ships and is compiled before anything is dropped. A deployed database is filled by the same seed only when the
  Worker is named (`charter run db:seed --deployed <worker>`), and never gets a seeded token.
- **A test leaves the Worker as it found it.** The same tests ask a deployed Worker: whoever a test signs up it
  deletes (`apptest`'s `SignUp` does), and a row it writes it takes away. The example's `test/main_test.go` fails a
  run that leaves more than the seed.
- **Every answer says who may keep it.** Cloudflare's cache for the Worker is on (`wrangler.jsonc`'s `cache`) and is
  asked before the Worker runs. A page that is the same for every reader says so (`Shared() bool` on its type) and
  is kept; `app/shared.go` decides for each answer, once, and everything else says `private`, in Go and in
  `worker/go.mjs`. Never a `Cache-Control` written in a page, and never another cache: a kept page is in Cloudflare's
  cache for the Worker and, behind it, in the Worker in front's KV, under the build's name. A shared page shows
  nothing it read for the request and gives no cookie.
- **A deployed app is two Workers, from one `wrangler.jsonc`.** The Go Worker (`<name>-go`) is the app; a few KB of
  JavaScript in front (`<name>`, `worker/front.mjs`) has the address and answers a kept page from KV, since a Worker
  takes as long to load where it is not running as it is large (`internal/command/split.go` has the measurement).
  `charter dev` runs the one Worker. Nothing is added to the Worker in front that would make it large, and it
  decides nothing about what may be kept: Go does.
- **Nothing long holds a Go runtime.** A runtime takes 13 MB or more of a Worker's 128 MB while it serves, so a
  request that lasts (a page's stream) is the hub's, in JavaScript (`live.Feed`'s `Signal` and `Since`), and
  `worker/go.mjs` keeps what the runtimes hold together under a budget that follows the build's size (a request
  with no room waits). Cloudflare's analytics say `exceededMemory`
  when this is broken; local dev has no such limit and never shows it.
- **A deployed Worker is watched, and no deploy waits on it.** `charter run health` asks Cloudflare how requests
  ended; `deploy` and `test --deployed` say how it did before them; a workflow asks daily; `charter run load
  --deployed` is for before a release, `charter run cold` (a first hit, from many places) for after a change to how
  the Worker starts. A check that costs seconds does not go into `check`, `test` or `deploy`.
  Workers Logs are on, and `charter run logs --since` reads them. A failure is one
  JSON line with a reference, and never has what a request sent.
- **A resource is its id.** `charter deploy` writes the database's and the KV namespace's ids into
  `wrangler.jsonc` and stops on a renamed bucket or queue (a new, empty one). `charter run delete` takes the whole
  app away, asked twice; nothing on Cloudflare is made or removed by hand.
- **A secret is never fetched by hand.** An app's secrets are declared in its own `fnox.toml` (names and providers,
  never values; an environment is a profile). `charter run secrets:put` has fnox keep one and puts it on the Worker;
  `charter test --deployed` hands the tests the ones fnox has.
- **A page uses what it shows, and nothing else.** No page calls every binding to fill cards: each thing
  Cloudflare gives does a job in the app, on a page of its own (`example/built`). A call to another site happens
  when someone asks for it. Times and numbers are kept as such and shown through Intl.
- **A table is written down once**, in `db/schema.sql`. Migrations, the queries' Go and the API's row types come
  from it; none is hand-written.
- **No statement is written in Go**, in an app or in charter. Each is a query of a `queries.sql`, checked by sqlc
  against its schema and written as a function (`internal/dbcode`); charter's own, for the sign-in's tables, are
  `internal/accounts`. `charter check` fails an app that hands `cloud` SQL as text or reaches D1 around it, and an
  app whose schema no longer fits charter's statements; a test fails charter for the same.
- **A reader sees their own writes, and no app says how.** A request's statements on D1 share a session; the answer
  to a request that read or wrote gives a bookmark that the next request brings back, so a reader never sees the
  database earlier than they have seen it (`cloud/session.go`, `app/bookmark.go`). Who someone
  is, and what a feed told a reader of, are read at the primary (`cloud.Newest`). Nothing reads D1 around that.
- **A feed is declared once** (`live.Feed`) and serves pages, API and SDK alike.
- **The commands are Vite+'s verbs** (https://viteplus.dev/guide/), plus `deploy`. Anything else is a task of
  `charter run`.
- **Upstream's tools, upstream's way.** gsx, gsxui and structpages-lint are the app's own Go tools, driven by the
  verb where a developer already is (`dev`, `check`, `add`, `run ui:`). Their skills stay true in a charter app:
  `skills/charter/SKILL.md` has the one table that says what each of their commands is here. Keep it complete.
- **`charter dev` stays running.** A command is not done until it has been run beside a live `charter dev`
  (`example/test/devloop`).
- **Checks:** `go test ./...` at the top, then in `example/` and in `site/`: `go tool charter check && go tool charter test`, and
  `go tool charter test --deployed` after a deploy. Work is on `main`. CI is one Linux job per push.
- **Needs the owner's OK:** anything on another owner's repository.
- JavaScript only in `app/worker/` (the glue that runs Go, the hub's Durable Object, the Worker in front), `internal/intlkit/js`,
  `internal/themes/js` (what connects gsxui's own theme editor to an app, and nothing that chooses or resolves a
  theme: that is gsxui's) and the apps' `web/`. Keep it logic-free.
