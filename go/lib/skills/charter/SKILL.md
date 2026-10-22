---
name: charter
description: Use when a Go app runs on Cloudflare Workers with github.com/joeblew999/charter (charter.toml, go tool charter) — adding or changing a binding (D1, a Durable Object, a rate limit, a variable), a migration, a route or handler on Workers, running it locally on workerd, deploying it (production, staging, a preview), its CI and Workers Builds, or when go tool charter check reports a problem.
---

# A Go Worker with charter

Everything about the Worker on Cloudflare is `charter.toml` at the app's root (every key: `go doc
github.com/joeblew999/charter/go/lib/project`). `go tool charter` reads it for every step, and `go tool charter check`
says what is wrong or stale. Never hand-write workerd config, wrangler files or deploy scripts: they are generated.

## The app's code

- One `main` package: `func main() { transport.Run(handler) }` (`go/lib/transport`). It is the Worker on Cloudflare
  and a plain HTTP server on `$PORT` anywhere else.
- Reach Cloudflare only through charter's packages. Each stands in for itself off Workers (go test, a local server),
  as charter.toml says, so there is no file per platform:

| Call | On Workers | Off Workers |
|---|---|---|
| `bindings.Var("APP_ENV")` | a variable | the environment |
| `d1.Open("DB")`, `d1.Query[T]`, `d1.One[T]`, `d1.Exec` | D1 | SQLite in memory, migrated |
| `bindings.Allow("WRITES", bindings.ClientKey(r))` | a rate limit binding | charter.toml's limit, counted |
| `hub.DurableObject[T]("ROOM", name).Publish(ctx, item)` | Workers RPC to the object | in memory |
| `transport.CFOf(r)` | `request.cf` | zero (`transport.WithCF` in a test) |
| `bindings.StaticAssets("dist/site")` | 404 (assets answer first) | the folder |

- TinyGo builds what ships: register plain paths (`"/notes"`) and check the method with `web.Allow(w, r, "GET")`,
  never `"GET /notes"` or `{$}`; render pages with `web.Render` (buffered, a Content-Length); no `html/template`.
  Absolute URLs come from `web.Origin(r)`.
- D1 has no interactive transactions: one statement each, `RETURNING` for what it wrote.
- Memory between requests is a cache, never state: any request may start a new runtime.

## Bindings and migrations

Add them to charter.toml, never elsewhere:

```toml
[[d1]]
binding = "DB"
migrations = "migrations"   # *.sql in name order, each once; a new one is a new idempotent file

[[durable_objects]]
binding = "ROOM"
class = "Room"              # exported by main
migration = "v1"            # a new class takes a new migration tag

[[rate_limits]]
binding = "WRITES"
namespace_id = 1001         # unique in the account
limit = 60
period = 10                 # 10 or 60
```

An app's test can read charter.toml (`project.Load`) to check a value it copies.

## Running it

| Command | What |
|---|---|
| `go tool charter build` | the Worker into build/ with TinyGo (`-compiler=go`: seconds, never deployed) |
| `go tool charter serve` | on local workerd, charter.toml's port (`PORT` overrides), in your time zone |
| `go tool charter with <cmd>` | `<cmd>` against it on a free port, fresh state, `BASE` its URL (`-strict`: a panic fails it) |
| `go tool charter deploy production\|staging\|preview <name>` | to Cloudflare, as charter.toml says; prints its URL |
| `go tool charter tail` | the deployed Worker's live logs |
| `go tool charter token` | whether the Cloudflare token is active |

The mise tasks (`.mise/tasks/charter.toml`) are the same: `mise run charter:serve`, `charter:deploy production`, … A
deploy needs `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` (the environment, else fnox); confirm with the user
before deploying to production or creating anything in their account.

## CI and Workers Builds

A commit is compiled once, on CI:
- **CI** (`.github/workflows/charter.yml`) runs `go tool charter ci run` from charter.toml's `[ci]`: its `build`, then
  on a push the build published for its preview, then its `test`. A `v*` tag or a `release/v*` branch also releases:
  the commit is checked to be on main, tagged, `release_hook` runs, and the production branch moves to it.
- **Workers Builds** runs `.mise/tasks/charter-builds`, which fetches the charter CI published and deploys what the
  branch deploys: production for `production`, staging for `main`, otherwise the branch's preview.
- `go tool charter builds configure` sets Workers Builds up once.

## After upgrading charter

`go tool charter tasks`, `go tool charter workflow` and `go tool charter skill` write their files again, and
`go tool charter check` says when one is stale.
