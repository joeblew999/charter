---
title: Libraries
nav_order: 13
---

# Libraries: what an API imports

A Worker imports charter's library; the tool, `charter-ts`, only runs its commands. Both come from the same release.

## TypeScript: charter-lib

`charter-lib` (`ts/lib/`) is in a project's `dependencies`, and `charter-ts` in its `devDependencies`, at the same version. It is built with no Node types, so nothing in it can need Node: it runs on Workers.

| Import | What it is |
|---|---|
| `"charter-lib/serve"` | `serve(router, { document, trusted, limits })`: the API the standard way, every part below wired, every refusal as problem details ([The standard API](standard.md)); `guard`, the middleware that lets in and limits each call; `refused(location, message, value)`, a handler's own 422; `specsOf(contract, document)` for `charter.config.ts` |
| `"charter-lib/asyncapi"` | `asyncapi({...})` on a procedure: a WebSocket channel in the AsyncAPI spec |
| `"charter-lib/auth"` | Bearer tokens, Cloudflare Access, an OpenID Connect issuer and tokens the API issued itself, with scopes; each refusal's challenge, and what a client reads to find out how to get in ([Auth](auth.md)) |
| `"charter-lib/ratelimit"` | Limits per caller on Workers Rate Limiting bindings |
| `"charter-lib/follow"` | `follow()`: the gap-free feed behind the streams ([Streaming](streaming.md)) |
| `"charter-lib/mcp"` | Every procedure that answers once as an MCP tool at `/api/mcp`, on the official MCP SDK |

From the tool, for a project's own files:

| Import | What it is | In |
|---|---|---|
| `"charter-ts/config"` | `defineConfig` | `charter.config.ts` |
| `"charter-ts/cloudflare"` | `rateLimits(spec, worker, bindings.rateLimit)`: a Rate Limiting binding per limit the spec declares | `cloudflare.config.ts` |
| `"charter-ts/sdk"` | `sdk(group, { ifStale })`: an SDK the live tests need, generated when `fern/` changed | `charter.config.ts` |
| `"charter-ts/vite"` | The Vite config every project's is | `vite.config.ts` |
| `"charter-ts/ci"` | `CI`, the type of a repo's own jobs | `ci.config.ts` |

## Go: github.com/joeblew999/charter/go/lib

The packages of `go/lib/`. The rest is in their doc comments: `go doc github.com/joeblew999/charter/go/lib/<name>`.

| Package | What it is for | What you call |
|---|---|---|
| `serve` | The API the standard way: its routes under `/api`, allowed (`auth`) and limited (`ratelimit`), the specs, the MCP tools, `/.well-known/`, a channel's `?access_token=` ([The standard API](standard.md)) | `serve.API{Config, Routes, LiveTitle, Setting, Trusted}.Handler()`; `.OpenAPI`, `.AsyncAPI`, which `npm run spec` writes |
| `problem` | A refusal as RFC 9457 problem details, as Huma writes an operation's, for what answers outside one (404, 405, `/.well-known/`) | `problem.Write(w, status, detail)` |
| `humaworkers` | A Huma API on Workers: routes itself, registers an operation when a request first matches it | `humaworkers.New(humaworkers.Config(title, version), routes)` |
| `transport` | The request bridge to the Worker, and the WebSocket adapter natively; what Cloudflare knows of where a request came from (`request.cf`: country, city, time zone, …), each field read on its own so one Cloudflare changes can't cost the rest | `transport.Run(handler)` in `main`; `transport.CFOf(r)`; `transport.CFJSON(r)`, the object as Cloudflare sent it |
| `d1` | Statements on a D1 database; rows decode by their `json` tags (Wasm only). What `npm run spec` writes a project's queries over ([The database](api.md#the-database)) | `d1.Query[T](db, query, args...)`, `d1.One[T]` (no row: `d1.ErrNoRows`), `d1.Exec` |
| `hub` | The live wake-up of a feed: the Durable Object, or memory | `hub.DurableObject[T]("HUB", "<feed>")`, `hub.Memory[T]` |
| `follow` | The gap-free feed over a log and a hub ([Streaming](streaming.md)) | `follow.Follow(ctx, source, follow.Options{After: follow.Newest(after, lastEventID)}, emit)` |
| `fragments` | A feed to the browser as SSE, each item an event you render: htmx 4's or Datastar's ([Server-rendered pages](pages.md)). Ids are positions, so a browser resumes with `Last-Event-ID`; a stream ends after `For` (5 minutes) | `fragments.Stream[T]{Feed: feed, Options: options, Event: event}.ServeHTTP(w, r)`, `fragments.Data(prefix, html)` |
| `mcp` | The API's operations as MCP tools (`serve` mounts it) | `mcp.NotATool(op)` |
| `asyncapi` | AsyncAPI 3.0.0 from Huma operations: WebSocket channels | `asyncapi.Operation(op, asyncapi.Channel{...})`, `asyncapi.SendOperation` |
| `auth` | Who may call, with scopes declared in the contract and enforced by one middleware (401, 403): bearer tokens, Cloudflare Access (people and service tokens), an OpenID Connect issuer, tokens the API issued itself (`auth.Issued`, for an OAuth endpoint of its own); every refusal's challenge and `auth.WellKnown`, what a client reads to find out how to get in (RFC 9728, RFC 8414) ([Auth](auth.md)); `auth/authtest` is a test issuer | `auth.Scheme`, `auth.AccessScheme`, `auth.OIDCScheme`, `Security: auth.Needs("write")`, `auth.CallerOf(ctx)`; `serve` adds the middleware |
| `ratelimit` | How often a caller may call, beside `auth`: a limit per operation or per scope, declared in the contract (`x-rate-limit` and a 429 with Retry-After in the specs), counted per caller by a Workers Rate Limiting binding, in memory on the host ([Auth](ratelimits.md)) | `ratelimit.Declare(config.OpenAPI, perScope...)`, `Extensions: ratelimit.Limited(limit, nil)`; `serve` adds the middleware, after auth's |
| `specs` | What `npm run spec` runs for a Go project's `api.API`, which stops on what the specs don't show (a route that does not name its operation, an operation that cannot be its tool, an example missing or invalid) | `specs.Must(specs.Problems(spec))`, `specs.Main(spec.OpenAPI, spec.AsyncAPI)` |

A Go Worker's steps on Cloudflare are one command, `go tool charter <step>` (go.mod: `tool
github.com/joeblew999/charter/go/lib/cmd/charter`); `go tool charter <step> -h` lists a step's flags. `go tool
workerbuild`, `workerdeploy`, `workertail` and `workerbench` are the same steps by their old names, kept while repos
move over.

The build, `go tool charter build`: the Worker's Wasm with TinyGo (the runtime patches in [Upstream issues](upstream.md), an 8 MB starting heap, 128 KB stacks, `-opt=z`), the glue below copied into `build/`, and a cache that gives back a Wasm built before from the same sources (a build of minutes, 0.6 s when nothing changed). charter-ts's build runs it; a Go repo without Node runs it itself, with tinygo and wasm-opt on its PATH:

```sh
go tool charter build
```

Switches, each turning one part off so what it saves can be measured: `-reuse=false` (a Go runtime per request), `-patches=false` (TinyGo as it ships), `-heap=0` (TinyGo's default heap). `-compiler=go` builds with the standard compiler in seconds, for a dev loop; it is never deployed.

What a request costs, `go tool charter bench`: the CPU a local workerd spends per request, from the operating system's count, in repeated runs (`-json` for a program). No Cloudflare account is needed, so anyone gets the same comparison:

```sh
go tool charter bench -base http://localhost:8787 'GET /api/hello' 'POST /api/notes {"body":"x"}'
```

Putting it on Cloudflare without Node, `go tool charter deploy`: the build in `build/`, static assets (`-assets`), bindings (`-var`, `-d1`, `-do`, `-ratelimit`), `migrations/*.sql` on every D1 database (each once), a Durable Object migration and custom domains (`-domain`), over the REST API. `-preview <name>` deploys a Worker Preview instead, and `-delete-preview`, `-prune-previews` remove them. So a commit is compiled once, CI publishes its build (`-publish-build <commit>`, with `GITHUB_TOKEN`) and Workers Builds deploys that (`-fetch-build <commit>`); `-configure-builds` sets Workers Builds up. `go tool charter deploy -h` lists every flag. Its live logs, `go tool charter tail`: requests, console output and exceptions, until Ctrl-C deletes the tail.

```sh
fnox exec -- go tool charter deploy -name my-api -compat <date> -allow-existing -d1 DB=my-api-db -d1-create -migrations migrations
fnox exec -- go tool charter tail -name my-api
```

How Google sees a site with pages (robots.txt, the sitemap, an audit as Googlebot, Search Console, Lighthouse) is
[seo](https://github.com/joeblew999/go-htmx4/tree/main/seo), its own Go module, not part of charter.

The Worker glue, in `go/lib/worker/`, written into `build/` by the build:

| File | What it gives |
|---|---|
| `go.mjs` | `goWorker()`: `fetch` for the Worker, and `warm({ runtimes, paths })` (default 2 runtimes, at most 4) |
| `websocket.mjs` | The WebSocket adapter |
| `hub.mjs` | `Hub`, the Durable Object class |
| `tinygo-clock.mjs` | Makes Go timers fire on Cloudflare |
