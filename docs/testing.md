# test/: the tests both servers must pass

One set of test programs for the oRPC Worker (`api/`) and the Go Worker (`api-go/`). They only know the API's URL and, where they use a generated SDK, which Fern folder it came from (`api` or `api-go`), so they also check that the two servers are interchangeable.

| File | What it does | Run it |
|---|---|---|
| `live-test.mjs` | Quick check with raw clients: an SSE stream and a WebSocket both receive a new note, then SSE resume after a reconnect | `mise run api:live-test`, `mise run api-go:live-test`; locally inside `mise run api-go:check` |
| `sdk-live-test.mjs` | The same through Fern's TypeScript SDK: `notes.watch()` and `liveNotes.connect()` | part of the two `live-test` tasks |
| `soak.mjs` | The real-time matrix: 7 clients × (planned end, hub restart by redeploy, client drop, long idle). PASS = every note exactly once, in order | `mise run api:soak`, `mise run api-go:soak` |
| `soak-go/` | The Go SDK client that `soak.mjs` runs | built by `soak.mjs` |

```sh
node test/live-test.mjs <url>
node test/sdk-live-test.mjs <url> [api|api-go]
node test/soak.mjs <url> [--sdk api-go] [--deploy-task api-go:deploy] [--no-deploy] [--seconds 100] [--idle 20]
```

The design they test is in [plans/realtime.md](plans/realtime.md). Unit tests live with the code: `api/test/` (vitest) and `api-go/**/_test.go`.
