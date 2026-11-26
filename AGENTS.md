# Rules for agents

How the repo works: the docs, which are `example/docs/*.md` and are served by the example itself. Plans and open work are GitHub issues here, never files.

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
  `example` and `skills` (the skill charter ships, as gsx and structpages ship theirs). Don't add another.
- **`example/` uses all of charter**, is charter's docs site (its `docs/` folder; never write docs anywhere else),
  and is what `charter create` copies. A change to how charter works changes the page of the docs that says so, in
  the same commit. A feature of charter that the example does
  not use is not done; a bug the example cannot show is a missing part of the example.
- **A developer never has to remember to run something.** What an upstream tool generates or checks runs inside
  `dev`, `check`, `test` or `deploy`. What is left is a namespaced task of `charter run` (`intl:`, `seo:`, `db:`,
  `ui:`).
- **Tests are of the pages and of the API**: pages in a real browser (`apptest.Browser`), the API through its own
  SDK. Skills are Claude Code plugins: this repo is one and a marketplace (`.claude-plugin`, `skills/charter`) that also
  lists gsx, gsxui and structpages from their owners' repos. Never copy a skill's files.
- **A table is written down once**, in `db/schema.sql`. Migrations, the queries' Go and the API's row types come
  from it; none is hand-written.
- **A feed is declared once** (`live.Feed`) and serves pages, API and SDK alike.
- **The commands are Vite+'s verbs** (https://viteplus.dev/guide/), plus `deploy`. Anything else is a task of
  `charter run`.
- **`charter dev` stays running.** A command is not done until it has been run beside a live `charter dev`
  (`example/test/devloop`).
- **Checks:** `go test ./...` at the top, then in `example/`: `go tool charter check && go tool charter test`, and
  `go tool charter test --deployed` after a deploy. Work is on `main`. CI is one Linux job per push.
- **Needs the owner's OK:** anything on another owner's repository.
- JavaScript only in `app/worker/` (the glue that runs Go, the hub's Durable Object), `internal/intlkit/js` and the
  example's `web/`. Keep it logic-free.
