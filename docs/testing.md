# test/: the tests both servers must pass

One set of test programs for the oRPC Worker (`api/`) and the Go Worker (`api-go/`). They only know the API's URL and, where they use a generated SDK, which Fern folder it came from (`api` or `api-go`), so they also check that the two servers are interchangeable.

| File | What it does | Run it |
|---|---|---|
| `live-test.mjs` | Quick check with raw clients: an SSE stream and a WebSocket both receive a new note, then SSE resume after a reconnect | `mise run api:live-test`, `mise run api-go:live-test`; locally inside `mise run api-go:check` |
| `sdk-live-test.mjs` | The same through Fern's TypeScript SDK: `notes.watch()` and `liveNotes.connect()` | part of the two `live-test` tasks |
| `mcp-test.mjs` | The MCP endpoint (`/api/mcp`, the Go server only so far) with the official TypeScript MCP client, in both protocol eras: the tools are the contract's operations, and a tool call does what the REST route does | `mise run api-go:mcp-test`; locally inside `mise run api-go:check`, natively and under workerd |
| `soak.mjs` | The real-time matrix: 7 clients × (planned end, hub restart by redeploy, client drop, long idle). PASS = every note exactly once, in order | `mise run api:soak`, `mise run api-go:soak` |
| `soak-go/` | The Go SDK client that `soak.mjs` runs | built by `soak.mjs` |

```sh
node test/live-test.mjs <url>
node test/sdk-live-test.mjs <url> [api|api-go]
node test/mcp-test.mjs <url>
node test/soak.mjs <url> [--sdk api-go] [--deploy-task api-go:deploy] [--no-deploy] [--seconds 100] [--idle 20]
```

The design they test is in [plans/realtime.md](plans/realtime.md). Unit tests live with the code: `api/test/` (vitest), `api-go/**/_test.go`, and `sdk/harness/test/` (Node's test runner, `mise run showcase:test`: the showcase server's routes, and its generated specs against the surface of the hand-written ones).

## Local and remote: both, with the same programs

Every test program takes a URL, so one file tests a local server and the deployed Worker. Both runs are needed, because each catches what the other can't:

| Where | What runs it | What it catches |
|---|---|---|
| Native Go build, and the Wasm under local workerd (`cf dev`) | `mise run check`, and CI on every push | Logic, the contract, TinyGo's gaps |
| The deployed Worker on Cloudflare | `mise run api:live-test`, `api-go:live-test` (also the last step of the `api-deploy` workflow), and the soak tasks | What only production does. Go timers hung there and nowhere else: local workerd has a real clock, Cloudflare's only moves on I/O ([upstream.md](upstream.md)) |

A green local run is not a verdict on a deploy. Deploy through `api-deploy`, or run the live test yourself afterwards.
