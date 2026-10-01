---
title: Test programs (test/)
nav_order: 7
parent: This repository
---
# test/: the tests both servers must pass

One set of test programs for the oRPC Worker (`api/`) and the Go Worker (`api-go/`), plus one for the two showcases. They only know the server's URL and, where they use a generated SDK, which Fern folder it came from, so they also check that the servers are interchangeable. Read this page to run a test, to see what a check covers, or before deploying.

## Running them

```sh
mise run check                 # LOCAL: every local check (Docker, no Cloudflare account)
mise run api:live-test         # REMOTE: the deployed oRPC Worker
mise run api-go:live-test      # REMOTE: the deployed Go Worker, then its MCP endpoint
mise run api:soak              # REMOTE, redeploys orpc-api: the real-time matrix
mise run api-go:soak           # REMOTE, redeploys orpc-api-go: the same, with the SDKs and CLI made from the Go specs
```

`mise run check` is `api:check`, `api-go:check`, `showcase-go:check`, `dev:check`, `sdk:demo`, `showcase:check` and `sdk:harness:test`. The checks that start a server pick free ports themselves, so several can run at once.

The remote tasks write test notes into the deployed Worker's database, and the soak tasks redeploy it. Anything after the task name goes to the program: `mise run api-go:soak --idle 20`.

## The test programs

| Path | What it does | Run by |
|---|---|---|
| `test/live-test.mjs` | Quick check with raw clients: an SSE stream and a WebSocket both receive a new note, then SSE resumes with `Last-Event-ID` after a reconnect (3 checks) | `mise run api:live-test`, `mise run api-go:live-test`; locally inside `mise run api-go:check`, natively and under workerd |
| `test/sdk-live-test.mjs` | The same through Fern's TypeScript SDK: `notes.watch()` and `liveNotes.connect()` (2 checks) | the two `live-test` tasks |
| `test/mcp-test.mjs` | The MCP endpoint (`/api/mcp`, the Go server only) with the official TypeScript MCP client, in both protocol eras: the tools are the contract's operations, and a tool call does what the REST route does ([mcp.md](mcp.md)) | `mise run api-go:mcp-test` against a running server; inside `mise run api-go:check`; and `mise run api-go:live-test` |
| `test/showcase-test.mjs` | A showcase server through Fern's TypeScript SDK, every Fern feature: OAuth, pagination, idempotency, SSE, upload, the signed webhook, the WebSocket both ways, audiences. Unless `--open` is given it also checks that tokens are enforced ([showcase-go.md](showcase-go.md)) | `mise run showcase-go:test` against a running server; inside `mise run showcase-go:check`, natively and under workerd |
| `test/soak.mjs` | The real-time matrix: 7 clients through a planned end, a hub restart by redeploy, a client drop and a long idle. PASS is every note exactly once, in order ([realtime.md](realtime.md#how-it-is-tested)) | `mise run api:soak`, `mise run api-go:soak` |
| `test/soak-go/` | The Go SDK client that `test/soak.mjs` runs: one `Notes.Watch()` call | built by `test/soak.mjs` |

Run directly, each takes a URL:

```sh
node test/live-test.mjs <url>
node test/sdk-live-test.mjs <url> [api|api-go]
node test/mcp-test.mjs <url>
node test/showcase-test.mjs <url> [showcase-go|showcase] [--webhook-port <port>] [--open]
node test/soak.mjs <url> [--sdk api-go] [--deploy-task api-go:deploy] [--no-deploy] [--seconds 100] [--deploy-at 30] [--drop-at 65] [--stream-seconds 15] [--idle 20]
```

- **They need the npm packages in `sdk/`** (`mise run setup`): `ws` and the MCP client are imported from there.
- **The SDK tests need the generated SDKs** in `sdk/out/`. The tasks make them first with `mise run sdk:ready`, which needs Docker the first time; the soak also needs the Fern CLI built (Rust).
- **`--webhook-port`** makes the showcase test listen for the webhook: start the server with `WEBHOOK_URL=http://localhost:<port>/webhook`. `mise run showcase-go:test` leaves that check out.
- **`--open`** is for the oRPC showcase, which checks no tokens. Its URL ends in `/api/mock`.
- **`--no-deploy`** runs the soak without the hub restart; `--idle <minutes>` runs only the long-idle case.

## Unit tests

They live with the code.

| Where | Runner | Task |
|---|---|---|
| `api/test/` | vitest, in Node: `follow()` | `mise run api:test` |
| `api-go/` (`*_test.go` in each package) | `go test`: the feed, the API, the MCP endpoint, the Go showcase, and the same SDK surface as the two oRPC contracts | `mise run api-go:test` |
| `sdk/harness/test/` | Node's test runner: the oRPC showcase server's routes, and its generated specs against the surface of the hand-written ones | `mise run showcase:test` |
| `dev/` (`*_test.go`) | `go test`: the workflow templates, version tags, and a project made by `dev new` | `mise run dev:check` |

## Local and remote: both, with the same programs

Every test program takes a URL, so one file tests a local server and the deployed Worker. Both runs are needed, because each catches what the other can't:

| Where | What runs it | What it catches |
|---|---|---|
| Native Go build, and the Wasm under local workerd (`cf dev`) | `mise run check`, and the check workflows on every push to main and pull request ([dev.md](dev.md#github-workflows)) | Logic, the contract, TinyGo's gaps (`go test` can't see those) |
| The deployed Worker on Cloudflare | `mise run api:live-test`, `mise run api-go:live-test` (also the last step of the `api-deploy` workflow), and the soak tasks | What only production does. Go timers hung there and nowhere else: local workerd has a real clock, Cloudflare's only moves on I/O ([upstream.md](upstream.md)) |

A green local run is not a verdict on a deploy. Deploy through `api-deploy`, or run the live test yourself afterwards.

## What is not tested

- **The oRPC Worker has no local run of the live test.** `mise run api:check` is typecheck, unit tests and spec drift; its real-time paths are only tested deployed.
- **The Go showcase has no task for the deployed Worker.** The command is in [showcase-go.md](showcase-go.md#tasks-from-the-repo-root).
- **The soak is not in CI.** It redeploys a Worker and takes minutes; it is run by hand.
