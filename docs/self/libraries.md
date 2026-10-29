---
title: Libraries
nav_order: 13
---

# Libraries: what an API imports

A Worker imports charter's library; the command, `charter`, runs a project's steps, from its `charter.toml`. Both come from the same release.

## TypeScript: charter-lib

`charter-lib` (`ts/`) is in a project's `dependencies`. It is built with no Node types, so nothing in it can need Node: it runs on Workers.

| Import | What it is |
|---|---|
| `"charter-lib/serve"` | `serve(router, { document, trusted, limits })`: the API the standard way, every part below wired, every refusal as problem details ([The standard API](standard.md)); `guard`, the middleware that lets in and limits each call; `refused(location, message, value)`, a handler's own 422; `specsOf(contract, document)`, the `specs` `api/spec.ts` exports for `charter spec` |
| `"charter-lib/asyncapi"` | `asyncapi({...})` on a procedure: a WebSocket channel in the AsyncAPI spec |
| `"charter-lib/auth"` | Bearer tokens, Cloudflare Access, an OpenID Connect issuer and tokens the API issued itself, with scopes; each refusal's challenge, and what a client reads to find out how to get in ([Auth](auth.md)) |
| `"charter-lib/ratelimit"` | Limits per caller on Workers Rate Limiting bindings |
| `"charter-lib/follow"` | `follow()`: the gap-free feed behind the streams ([Streaming](streaming.md)) |
| `"charter-lib/mcp"` | Every procedure that answers once as an MCP tool at `/api/mcp`, on the official MCP SDK |

A Worker's bindings are `env` from `cloudflare:workers`, typed by Cloudflare's runtime types (`@cloudflare/workers-types`, in a project's `devDependencies`) and charter's, from `charter.toml` (`.charter/env.d.ts`, which `charter build` and `check` write).

## Go: github.com/joeblew999/charter

The packages at the repo's top, its root module. The rest is in their doc comments: `go doc github.com/joeblew999/charter/<name>`.

| Package | What it is for | What you call |
|---|---|---|
| `serve` | The API the standard way: its routes under `/api`, allowed (`auth`) and limited (`ratelimit`), the specs, the MCP tools, `/.well-known/`, a channel's `?access_token=` ([The standard API](standard.md)) | `serve.API{Config, Routes, LiveTitle, Setting, Trusted}.Handler()`; `.OpenAPI`, `.AsyncAPI`, which `charter spec` writes |
| `problem` | A refusal as RFC 9457 problem details, as Huma writes an operation's, for what answers outside one (404, 405, `/.well-known/`) | `problem.Write(w, status, detail)` |
| `humaworkers` | A Huma API on Workers: routes itself, registers an operation when a request first matches it | `humaworkers.New(humaworkers.Config(title, version), routes)` |
| `transport` | The request bridge to the Worker, and the WebSocket adapter natively; what Cloudflare knows of where a request came from (`request.cf`: country, city, time zone, …), each field read on its own so one Cloudflare changes can't cost the rest | `transport.Run(handler)` in `main`; `transport.CFOf(r)`; `transport.CFJSON(r)`, the object as Cloudflare sent it |
| `d1` | Statements on a D1 database; rows decode by their `json` tags (Wasm only). What `charter spec` writes a project's queries over ([The database](api.md#the-database)) | `d1.Query[T](db, query, args...)`, `d1.One[T]` (no row: `d1.ErrNoRows`), `d1.Exec` |
| `hub` | The live wake-up of a feed: the Durable Object, or memory | `hub.DurableObject[T]("HUB", "<feed>")`, `hub.Memory[T]` |
| `follow` | The gap-free feed over a log and a hub ([Streaming](streaming.md)) | `follow.Follow(ctx, source, follow.Options{After: follow.Newest(after, lastEventID)}, emit)` |
| `fragments` | A feed to the browser as SSE, each item an event you render: htmx 4's or Datastar's ([Server-rendered pages](pages.md)). Ids are positions, so a browser resumes with `Last-Event-ID`; a stream ends after `For` (5 minutes) | `fragments.Stream[T]{Feed: feed, Options: options, Event: event}.ServeHTTP(w, r)`, `fragments.Data(prefix, html)` |
| `mcp` | The API's operations as MCP tools (`serve` mounts it) | `mcp.NotATool(op)` |
| `asyncapi` | AsyncAPI 3.0.0 from Huma operations: WebSocket channels | `asyncapi.Operation(op, asyncapi.Channel{...})`, `asyncapi.SendOperation` |
| `auth` | Who may call, with scopes declared in the contract and enforced by one middleware (401, 403): bearer tokens, Cloudflare Access (people and service tokens), an OpenID Connect issuer, tokens the API issued itself (`auth.Issued`, for an OAuth endpoint of its own); every refusal's challenge and `auth.WellKnown`, what a client reads to find out how to get in (RFC 9728, RFC 8414) ([Auth](auth.md)); `auth/authtest` is a test issuer | `auth.Scheme`, `auth.AccessScheme`, `auth.OIDCScheme`, `Security: auth.Needs("write")`, `auth.CallerOf(ctx)`; `serve` adds the middleware |
| `ratelimit` | How often a caller may call, beside `auth`: a limit per operation or per scope, declared in the contract (`x-rate-limit` and a 429 with Retry-After in the specs), counted per caller by a Workers Rate Limiting binding, in memory on the host ([Auth](ratelimits.md)) | `ratelimit.Declare(config.OpenAPI, perScope...)`, `Extensions: ratelimit.Limited(limit, nil)`; `serve` adds the middleware, after auth's |
| `specs` | What `charter spec` runs for a Go project's `api.API`, which stops on what the specs don't show (a route that does not name its operation, an operation that cannot be its tool, an example missing or invalid) | `specs.Must(specs.Problems(spec))`, `specs.Main(spec.OpenAPI, spec.AsyncAPI)` |

A Worker's steps on Cloudflare are one command, `charter <step>`, a binary of charter's GitHub release that a repo
takes from mise: `mise use github:joeblew999/charter` pins it in the repo's `mise.toml`. `charter <step> -h` lists a
step's flags, and `charter version` says which charter it is.

A Go app is set up for Workers the way go-htmx4's intl and seo set one up, and as
[mise-cloudflare](https://github.com/joeblew999/mise-cloudflare) sets a repo up: a pin, one command, a config file, its
own mise tasks, an agent skill, and a check. `mise use github:joeblew999/charter`, then `charter init` from the app's
root, writes:
- `charter.toml` (named after the module, dated from the workerd on its PATH);
- `build/` into `.gitignore`;
- the mise tasks, `.mise/tasks/charter.toml` (`charter:build`, `serve`, `with`, `deploy`, `tail`, `token`, `check`,
  `ci`, `builds-configure`, …), and Workers Builds' launcher beside them, `.mise/tasks/charter-builds`;
- the TinyGo and wasm-opt its Worker builds with, `.config/mise/conf.d/charter.toml`, which mise reads beside the app's
  own `mise.toml`: the versions this charter's TinyGo patch is for, so the app pins neither itself (`charter
  check` refuses a `tinygo` or binaryen line in its `mise.toml`, and the build refuses another TinyGo);
- the agent skill, `.claude/skills/charter`;
- the CI workflow, `charter.yml` in the app's GitHub workflows folder, which runs `charter ci run`.

`charter check` names every problem with `charter.toml` and with those files, and when the app's two pins of charter
differ: the command in `mise.toml` and the library in `go.mod`, which the build takes the Worker glue from. To upgrade,
move both (`mise use github:joeblew999/charter@X.Y.Z`, `go get github.com/joeblew999/charter@vX.Y.Z`), then `charter
tasks`, `skill` and `workflow` write their files again. The app writes one `main` package, `transport.Run(handler)`,
which is the Worker on Cloudflare and a plain HTTP server off it, and its bindings in `charter.toml`. go-htmx4's
`e2e/fresh` makes such an app from nothing and runs it on workerd (and, asked to, on Cloudflare).

Everything about the Worker on Cloudflare is one file at the app's root, `charter.toml` (package `charter/project`
documents every key): its name, compatibility date and modules, Static Assets, D1 databases, Durable Objects and
rate limits, and what production, staging, previews and local workerd each get. The steps read it, and an app's tests
can (`project.Load`), so a limit a test counts to is checked against the one that ships.

```toml
name = "my-app"
compatibility_date = "2026-09-11"
main = "worker/index.mjs"
assets = "dist/site"
health = "/healthz"

[[d1]]
binding = "DB"
migrations = "migrations"

[[rate_limits]]
binding = "WRITES"
namespace_id = 1001
limit = 60
period = 10

[production]
domain = "app.example.com"
vars = { APP_ENV = "production" }
```

Locally, no wrangler and no Node: `charter local` writes `.charter/` from it (workerd's config, an entry that
gives the app a D1 database on a Durable Object's SQLite with its migrations, in-memory rate limits and the visitor's
time zone, and Static Assets in front), ignored by git. `charter serve` runs it on its port in the developer's
time zone; `charter with <command>` runs a command against it on a free port with fresh state, `BASE` its URL,
and stops it (`-strict`: a panic or uncaught exception in workerd's log fails it too). Deploys read it too: `charter
deploy production`, `staging` or `preview <name>`.

CI and Workers Builds share the work so that a commit is compiled once, on CI. `charter ci run` is the
whole CI, from `charter.toml`'s `[ci]` (`tools`, `build`, `test`, `release_hook`): it publishes on a push, releases a
`v*` tag or a `release/v*` branch, and prints where the time went. On GitHub, `charter build` shares its
builds through the repository's `charter-cache` pre-release, keyed by every input: a release, or a change that leaves
the Worker's inputs alone, takes the earlier build instead of minutes of TinyGo and wasm-opt. `charter ci publish`, from a
static linux/amd64 build of the command, puts the commit's build and that very binary on the repository's
`preview-builds` pre-release, and the branch's preview URL on its pull request. Workers Builds then runs that binary:
`.mise/tasks/charter-builds` fetches that binary, `charter builds build` fetches the build, verified (or
`-compile` builds it there with `[build] command` when CI published none), and `charter builds deploy` deploys
what the branch
deploys, which is production for `production`, staging for `main`, otherwise the branch's own preview. A release is
`charter ci release <version>` on CI: once the commit is checked to be on `main`, it is tagged and the
`production` branch moves to it. `charter token` checks the Cloudflare token before a deploy. `charter.toml`'s
`repo` and `subdomain` say where these happen.

The build, `charter build`: the Worker's Wasm with TinyGo (the runtime patches in [Upstream issues](upstream.md), an 8 MB starting heap, 128 KB stacks, `-opt=z`), the glue below copied into `build/`, and a cache that gives back a Wasm built before from the same sources (a build of minutes, 0.6 s when nothing changed). `charter check`, `dev` and `twins` run it too, with tinygo and wasm-opt from charter's pins (through mise). A TypeScript Worker it bundles with esbuild instead, into `build/worker.mjs`.

```sh
charter build
```

Switches, each turning one part off so what it saves can be measured: `-reuse=false` (a Go runtime per request), `-patches=false` (TinyGo as it ships), `-heap=0` (TinyGo's default heap). `-compiler=go` builds with the standard compiler in seconds, for a dev loop; it is never deployed.

What a request costs, `charter bench`: the CPU a local workerd spends per request, from the operating system's count, in repeated runs (`-json` for a program). No Cloudflare account is needed, so anyone gets the same comparison:

```sh
charter bench -base http://localhost:8787 'GET /api/hello' 'POST /api/notes {"body":"x"}'
```

Putting it on Cloudflare without Node, `charter deploy`: the build in `build/`, static assets (`-assets`), bindings (`-var`, `-d1`, `-do`, `-ratelimit`), `migrations/*.sql` on every D1 database (each once), a Durable Object migration and custom domains (`-domain`), over the REST API. `-preview <name>` deploys a Worker Preview instead, and `-delete-preview`, `-prune-previews` remove them. So a commit is compiled once, CI publishes its build (`-publish-build <commit>`, with `GITHUB_TOKEN`) and Workers Builds deploys that (`-fetch-build <commit>`); `-configure-builds` sets Workers Builds up. `charter deploy -h` lists every flag. Its live logs, `charter tail`: requests, console output and exceptions, until Ctrl-C deletes the tail.

```sh
fnox exec -- charter deploy -name my-api -compat <date> -allow-existing -d1 DB=my-api-db -d1-create -migrations migrations
fnox exec -- charter tail -name my-api
```

How Google sees a site with pages (robots.txt, the sitemap, an audit as Googlebot, Search Console, Lighthouse) is
[seo](https://github.com/joeblew999/go-htmx4/tree/main/seo), its own Go module, not part of charter.

The Worker glue, in `worker/`, written into `build/` by the build:

| File | What it gives |
|---|---|
| `go.mjs` | `goWorker()`: `fetch` for the Worker, and `warm({ runtimes, paths })` (default 2 runtimes, at most 4) |
| `websocket.mjs` | The WebSocket adapter |
| `hub.mjs` | `Hub`, the Durable Object class |
| `tinygo-clock.mjs` | Makes Go timers fire on Cloudflare |
