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
- **Go and Node are all a developer installs.** `go.mod` and `package-lock.json` pin everything. charter brings
  the two other programs it picks, itself, on every system (`internal/tools`): fnox for secrets (a pinned release,
  checked against its SHA-256) and Kiri for containers. Never call either through mise or another manager, and
  never ask a developer to install one. No TypeScript of ours: `vite.config.ts` is gsx's file.
- **We pick winners.** One tool per job, brought and driven by charter, so the hard parts are easy. Don't add a
  second choice or a way around one.
- **Six library folders at the top** (`app`, `api`, `cloud`, `live`, `intl`, `seo`), with `cmd`, `internal`,
  `example`, `site` and `skills` (the skill charter ships, as gsx and structpages ship theirs). Don't add another.
- **`example/` uses all of charter** and is what `charter create` copies. A feature of charter that the example
  does not use is not done; a bug the example cannot show is a missing part of the example.
- **`site/` is charter's own site**, a charter app: a front page, and the docs (`site/docs/*.md`; never write docs
  anywhere else). A change to how charter works changes the page of the docs that says so, in the same commit. The
  pages that show docs are the docs screen.
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
  SDK. Skills are Claude Code plugins: this repo is one and a marketplace (`.claude-plugin`, `skills/charter`) that also
  lists gsx, gsxui and structpages from their owners' repos. Never copy a skill's files.
- **Who may do what is said once, for pages and API alike.** One `api.Auth` and one `api.Caller`; a page's
  `Needs` and an operation's scopes are the same words, checked by the same code. People sign in with passkeys
  (the sign-in screen); a session and a personal token are the same kind of thing. A client in any language is
  proven by calling the Worker (`example/test/sdk`), never only by compiling.
- **The seed and the tests are one thing.** What fills a developer's database is typed values and a function that
  writes them through the app's own functions; tests call the same function and name the same values. A seed never
  ships and is compiled before anything is dropped. A deployed database is filled by the same seed only when the
  Worker is named (`charter run db:seed --deployed <worker>`), and never gets a seeded token.
- **A test leaves the Worker as it found it.** The same tests ask a deployed Worker: whoever a test signs up it
  deletes (`apptest`'s `SignUp` does), and a row it writes it takes away. The example's `test/main_test.go` fails a
  run that leaves more than the seed.
- **Nothing long holds a Go runtime.** A runtime takes 13 MB or more of a Worker's 128 MB while it serves, so a
  request that lasts (a page's stream) is the hub's, in JavaScript (`live.Feed`'s `Signal` and `Since`), and
  `worker/go.mjs` keeps what the runtimes hold together under a budget. Cloudflare's analytics say `exceededMemory`
  when this is broken; local dev has no such limit and never shows it.
- **A deployed Worker is watched, and no deploy waits on it.** `charter run health` asks Cloudflare how requests
  ended; `deploy` and `test --deployed` say how it did before them; a workflow asks daily; `charter run load
  --deployed` is for before a release. A check that costs seconds does not go into `check`, `test` or `deploy`.
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
- JavaScript only in `app/worker/` (the glue that runs Go, the hub's Durable Object), `internal/intlkit/js` and the
  apps' `web/`. Keep it logic-free.
