# Rules for agents

How the repo works: [README.md](README.md). Plans and open work are GitHub issues here, never files.

- **Go only.** No TypeScript library, no TypeScript examples. `vite.config.ts` is gsx's file, kept under the name
  gsx and gsxui look for; a TypeScript SDK is something an app may generate for its users, never code here.
- **Drive the upstream tool, never copy it.** wrangler runs and deploys the Worker (`wrangler.jsonc`), Vite builds
  the browser's files, gsx generates and runs the dev loop, workers-go carries Go. Read the upstream guide and build
  its demo before designing anything.
- **Go and Node are all a machine needs.** No mise, no tool downloads of our own. `go.mod`'s toolchain line and
  `package-lock.json` pin everything; a Go tool is a `tool` line in `go.mod`, an npm tool a `devDependency`.
- **`charter.toml` holds only what wrangler has no place for** (the contract, SDKs, tests, `[intl]`, `[seo]`). A
  Worker's facts are wrangler's. No other settings file: languages and search engines are tables of it.
- **Every binding has a stand-in off Workers**, so there is no file per platform in an app and `go test` runs the
  real SQL.
- **The layout is `cf/`, `api/`, `site/`** for the library, `internal/` for the command, `examples/` for apps. A new
  top-level folder needs a reason a developer would recognise.
- **Checks:** `go test ./...` at the top (with `GOWORK=off`), and `go tool charter check` in each example. Changes
  land by branch and pull request once both pass.
- **Needs the owner's OK:** a release, a deploy, anything on another owner's repository.
- JavaScript only in `cf/worker/` (the glue that runs Go and the hub's Durable Object), `internal/intlkit/js`
  (Intl's bridge) and an example's `web/`. Keep it logic-free.
- **A site is `site.Site`**: one handler and one `<site.Head>`. An example that wires Vite, languages or search
  engines by hand is a bug in package `site`.
