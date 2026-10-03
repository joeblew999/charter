---
title: Go packages
nav_order: 3
parent: Reference
---

# Go packages: the library a Go project imports

The packages of `github.com/joeblew999/charter/go`. The rest is in the doc comments: `go doc github.com/joeblew999/charter/go/<name>`.

| Package | What it is for | What you call |
|---|---|---|
| `humaworkers` | A Huma API on Workers: routes itself, registers an operation when a request first matches it | `humaworkers.New(humaworkers.Config(title, version), routes)` |
| `transport` | The request bridge to the Worker, and the WebSocket adapter natively | `transport.Run(handler)` in `main` |
| `d1` | Statements on a D1 database; rows decode by their `json` tags (Wasm only) | `d1.Query[T](db, query, args...)` |
| `hub` | The live wake-up of a feed: the Durable Object, or memory | `hub.DurableObject[T]("HUB", "<feed>")`, `hub.Memory[T]` |
| `follow` | The gap-free feed over a log and a hub ([Streaming](../guides/streaming.md)) | `follow.Follow(ctx, source, follow.Options{After: &after}, emit)` |
| `fragments` | A feed to the browser as SSE, each item an event you render: htmx 4's or Datastar's ([Server-rendered pages](../guides/pages.md)). Ids are positions, so a browser resumes with `Last-Event-ID`; a stream ends after `For` (5 minutes) | `fragments.Stream[T]{Feed: feed, Options: options, Event: event}.ServeHTTP(w, r)`, `fragments.Data(prefix, html)` |
| `humamcp` | The API's operations as MCP tools | `humamcp.Handler(api)`, `humamcp.Expose(op, false)` |
| `asyncapi` | AsyncAPI 3.0.0 from Huma operations: WebSocket channels | `asyncapi.Operation(op, asyncapi.Channel{...})`, `asyncapi.SendOperation` |
| `auth` | Who may call, with scopes declared in the contract and enforced by one middleware (401, 403): bearer tokens, Cloudflare Access (people and service tokens), an OpenID Connect issuer ([Auth](../guides/auth.md)); `auth/authtest` is a test issuer | `auth.Scheme`, `auth.AccessScheme`, `auth.OIDCScheme`, `Security: auth.Needs("write")`, `api.UseMiddleware(auth.Middleware(api, env.Var, trusted...))`, `auth.CallerOf(ctx)` |
| `ratelimit` | How often a caller may call, beside `auth`: a limit per operation or per scope, declared in the contract (`x-rate-limit` and a 429 with Retry-After in the specs), counted per caller by a Workers Rate Limiting binding, in memory on the host ([Auth](../guides/auth.md#rate-limits)) | `ratelimit.Declare(config.OpenAPI, perScope...)`, `Extensions: ratelimit.On(limit, nil)`, `routes.UseMiddleware(ratelimit.Middleware(routes, nil))` after auth's |
| `specfile` | The body of a project's `./cmd/spec` | `specfile.Main(api.OpenAPI, api.AsyncAPI)` |

The Worker glue, in `go/worker/`, written into `build/` by the build:

| File | What it gives |
|---|---|
| `go.mjs` | `goWorker()`: `fetch` for the Worker, and `warm({ runtimes, paths })` (default 2 runtimes, at most 4) |
| `websocket.mjs` | The WebSocket adapter |
| `hub.mjs` | `Hub`, the Durable Object class |
| `tinygo-clock.mjs` | Makes Go timers fire on Cloudflare |
