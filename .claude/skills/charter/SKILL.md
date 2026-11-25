---
name: charter
description: Use when a Go app runs on Cloudflare Workers with github.com/joeblew999/charter (go tool charter, wrangler.jsonc, charter.toml) — adding or changing a binding (D1, a Durable Object, a rate limit, a variable), a migration, a route or handler, an API's contract, specs and SDKs, a site's components and browser files, running it locally, deploying it, or when charter check reports a problem.
---

# A Go app on Workers with charter

charter drives the tools that own each job. Change the tool's own file, then let `go tool charter check` say what is
wrong.

| To change | Edit | Then |
|---|---|---|
| A binding, a variable, an environment | `wrangler.jsonc` | `go tool charter preview` |
| The database's schema | a new idempotent `migrations/NNNN_name.sql` | `go tool charter deploy` applies it |
| A query | `api/queries.sql` | `go tool charter spec` writes `api/queries.go` |
| An API's route | `api/contract.go` and its handler | `go tool charter spec` writes `fern/` |
| A page | a `.gsx` file | `go tool charter dev` reloads it |
| Browser code or CSS | `web/` (Vite) | `go tool charter dev` reloads it |
| A message a reader sees | the source catalog, `locales/en.toml` | `go tool charter intl translate`, or translate by hand; `go tool charter intl` says what is left |
| A language | `go tool charter intl add-locale de` | translate its catalog |
| A page search engines may index | `Pages` of `site.Site` in `main.go` | `go tool charter check` audits it |

## Rules

- **Bindings through charter, never workers-go directly**: `bindings.Var`, `bindings.Allow`, `bindings.StaticAssets`,
  `bindings.HTTPClient`, `d1.Open`, `hub.DurableObject`, `transport.CFOf(r)` (all under
  `github.com/joeblew999/charter/cf/`). Each has a stand-in off Workers, so there is no file per platform and
  `go test` runs the real SQL.
- **Memory between requests is a cache, never state**: charter keeps Go runtimes between requests, and any request may
  start a new one.
- **D1 has no interactive transactions**: single statements with `RETURNING`, or `d1.Batch`.
- Never hand-write what a command generates: `fern/`, `api/queries.go`, `*.x.go`, `build/`, `dist/`.
- `charter.toml` holds only what wrangler has no place for: `health`, `url`, `spec`, `db`, `[sdks]`, `[test]`,
  `[intl]`, `[seo]`.
- A site is `site.Site{…}.Handler(mux)` and `<site.Head title=… description=…/>`: never wire Vite, languages or
  robots.txt by hand.
- No literal reader-facing text in a `.gsx` file: a message in the source catalog, written `{ locales.Key }`.

## Commands

`go tool charter` lists them. Before a push: `go tool charter check`. To ship: `go tool charter deploy [environment]`,
with `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment (or `wrangler login`).
