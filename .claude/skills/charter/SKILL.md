---
name: charter
description: Use when a Go app runs on Cloudflare Workers with github.com/joeblew999/charter (go tool charter, wrangler.jsonc, charter.toml) — adding or changing a binding (D1, a Durable Object, a rate limit, a variable), a migration, a route or handler, an API's contract, specs and SDKs, a site's components and browser files, running it locally, deploying it, or when charter check reports a problem.
---

# A Go app on Workers with charter

charter drives the tools that own each job. Change the tool's own file, then let `go tool charter check` say what is
wrong.

| To change | Edit | Then |
|---|---|---|
| A binding, a variable, an environment | `wrangler.jsonc` (`go tool charter add d1:DB` creates a database) | the running `charter dev` reloads |
| The database's schema | a new idempotent `migrations/NNNN_name.sql` | `go tool charter deploy` applies it |
| A query | `api/queries.sql` | `go tool charter check --fix` writes `api/queries.go` |
| An API's route | `api/contract.go` and its handler | `go tool charter check --fix` writes `fern/` |
| A page | a `.gsx` file | the running `charter dev` reloads it |
| Browser code or CSS | `web/` (Vite); `go tool charter add <npm package>` | the running `charter dev` reloads it |
| A message a reader sees | the source catalog, `locales/en.toml` | `go tool charter run translate`, or translate by hand; `go tool charter run languages` says what is left |
| A language | `go tool charter add locale:de` | translate its catalog |
| A UI component | `go tool charter add ui:dialog` | use it in a `.gsx` file |
| A page search engines may index | `Pages` of `gsxapp.App` in `main.go` | `go tool charter test` audits it |

## Rules

- **Bindings are workers-go's**, imported and called directly (`workers-go/cloudflare/kv`, `r2`, `queues`, `d1`,
  `cron`, `cache`, `fetch`, `sockets`), in a file that builds only for a Worker (`//go:build js && wasm`). Email is
  `charter/bindings`. Register what is no request before the program serves: `cron.ScheduleTaskNonBlock`,
  `queues.ConsumeNonBlock`, `bindings.ConsumeEmail`.
- **For code that should also run under `go test`**, charter's stand-ins: `d1.Open`, `bindings.Var`,
  `bindings.Allow`, `live.DurableObject`, `transport.CFOf(r)`.
- **Memory between requests is a cache, never state**: charter keeps Go runtimes between requests, and any request may
  start a new one.
- **D1 has no interactive transactions**: single statements with `RETURNING`, or `d1.Batch`.
- Never hand-write what a command generates: `fern/`, `api/queries.go`, `*.x.go`, `build/`, `dist/`.
- `charter.toml` holds only what wrangler has no place for: `health`, `url`, `[check]` (`spec`, `db`), `[pack]`, `[test]`, `[staged]`, `[urls]`,
  `[intl]`, `[seo]`.
- A site is `gsxapp.App{…}.Handler(mux)` and `<gsxapp.Head title=… description=…/>`: never wire Vite, languages or
  robots.txt by hand.
- No literal reader-facing text in a `.gsx` file: a message in the source catalog, written `{ locales.Key }`.

## Commands

`go tool charter` lists them: Vite+'s verbs. Keep `go tool charter dev` running and run everything else beside it.
Before a push: `go tool charter check && go tool charter test`. The same tests ask a deployed Worker:
`go tool charter test --deployed` (at `url`), `--env <environment>` (its address in `charter.toml`'s `[urls]`) or
`--base <url>`. A binding in `wrangler.jsonc` needs no id: wrangler makes the resource on the first deploy. To ship: `go tool charter deploy [environment]`, with
`CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment, or fnox, or `wrangler login`.
