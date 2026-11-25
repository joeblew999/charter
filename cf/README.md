# cf: Cloudflare from Go

| Package | What it is |
|---|---|
| `transport` | `transport.Run(handler)`: the handler as a Worker, or as a plain server off Workers. `transport.CFOf(r)`: Cloudflare's view of the request |
| `bindings` | `Var`, `Allow` (a rate limit), `StaticAssets`, `HTTPClient` |
| `d1` | `d1.Open("DB")`: D1 on Workers, SQLite in memory with your migrations off them. `Query`, `Exec`, `Batch` |
| `hub` | `hub.DurableObject[T](binding, name)`: publish to and follow a Durable Object that fans messages out |
| `ratelimit` | A rate limit declared in an API's contract |
| `worker/` | The JavaScript that runs the Go: `go.mjs` keeps Go runtimes between requests, `hub.mjs` is the hub's Durable Object class. `charter build` copies it into `build/` |

A binding's name is the one in `wrangler.jsonc`.
