---
title: Test programs (examples/notes-go/test/)
nav_order: 7
parent: This repository
---
# test/: the tests both servers must pass

One set of test programs for the oRPC Worker (`examples/notes-ts/`) and the Go Worker (`examples/notes-go/`), plus one for the two showcases. They only know the server's URL and, where they use a generated SDK, which Fern folder it came from, so they also check that the servers are interchangeable. Read this page to run a test, to see what a check covers, or before deploying.

## Running them

```sh
mise run check                 # LOCAL: every local check (Docker, no Cloudflare account)
mise run live-test         # REMOTE: the deployed oRPC Worker
mise run live-test      # REMOTE: the deployed Go Worker, then its MCP endpoint
mise run soak              # REMOTE, redeploys the Worker: the real-time matrix
mise run soak           # REMOTE, redeploys charter-notes-go: the same, with the SDKs and CLI made from the Go specs
```

`mise run check` is `check`, `check`, `check`, `charter:check`, `check` and `test:workerd`. The checks that start a server pick free ports themselves, so several can run at once.

The remote tasks write test notes into the deployed Worker's database, and the soak tasks redeploy it. Anything after the task name goes to the program: `mise run soak --idle 20`.

## The test programs

| Path | What it does | Run by |
|---|---|---|
| `examples/notes-go/test/live-test.mjs` | Quick check with raw clients: an SSE stream and a WebSocket both receive a new note, then SSE resumes with `Last-Event-ID` after a reconnect (3 checks) | `mise run live-test`, `mise run live-test`; locally inside `mise run check`, natively and under workerd |
| `examples/notes-go/test/sdk-live-test.mjs` | The same through Fern's TypeScript SDK: `notes.watch()` and `liveNotes.connect()` (2 checks) | the two `live-test` tasks |
| `examples/notes-go/test/mcp-test.mjs` | The MCP endpoint (`/api/mcp`, the Go server only) with the official TypeScript MCP client, in both protocol eras: the tools are the contract's operations, and a tool call does what the REST route does ([mcp.md](mcp.md)) | `mise run mcp-test` against a running server; inside `mise run check`; and `mise run live-test` |
| `examples/showcase-go/test/showcase-test.mjs` | A showcase server through Fern's TypeScript SDK, every Fern feature: OAuth, pagination, idempotency, SSE, upload, the signed webhook, the WebSocket both ways, audiences. Unless `--open` is given it also checks that tokens are enforced ([showcase-go.md](showcase-go.md)) | `mise run test` against a running server; inside `mise run check`, natively and under workerd |
| `examples/notes-go/test/soak.mjs` | The real-time matrix: 7 clients through a planned end, a hub restart by redeploy, a client drop and a long idle. PASS is every note exactly once, in order ([realtime.md](realtime.md#how-it-is-tested)) | `mise run soak`, `mise run soak` |
| `examples/notes-go/test/soak-go/` | The Go SDK client that `examples/notes-go/test/soak.mjs` runs: one `Notes.Watch()` call | built by `examples/notes-go/test/soak.mjs` |

Run directly, each takes a URL:

```sh
node examples/notes-go/test/live-test.mjs <url>
node examples/notes-go/test/sdk-live-test.mjs <url>
node examples/notes-go/test/mcp-test.mjs <url>
node examples/notes-go/test/showcase-test.mjs <url> [showcase-go|showcase] [--webhook-port <port>] [--open]
node examples/notes-go/test/soak.mjs <url> [--no-deploy] [--seconds 100] [--deploy-at 30] [--drop-at 65] [--stream-seconds 15] [--idle 20]
```

- **They need the npm packages in `sdk/`** (`mise run setup`): `ws` and the MCP client are imported from there.
- **The SDK tests need the generated SDKs** in `sdk/out/`. The tasks make them first with `mise run sdk:ready`, which needs Docker the first time; the soak also needs the Fern CLI built (Rust).
- **`--webhook-port`** makes the showcase test listen for the webhook: start the server with `WEBHOOK_URL=http://localhost:<port>/webhook`. `mise run test` leaves that check out.
- **`--open`** is for the oRPC showcase, which checks no tokens. Its URL ends in `/api/mock`.
- **`--no-deploy`** runs the soak without the hub restart; `--idle <minutes>` runs only the long-idle case.

## Unit tests

They live with the code.

| Where | Runner | Task |
|---|---|---|
| `examples/notes-ts/test/` | vitest, in Node: `follow()` | `mise run test` |
| `examples/notes-go/` (`*_test.go` in each package) | `go test`: the feed, the API, the MCP endpoint, the Go showcase, and the same SDK surface as the two oRPC contracts | `mise run test` |
| `examples/showcase-ts/test/` | Node's test runner: the oRPC showcase server's routes, and its generated specs against the surface of the hand-written ones | `mise run test` |
| `cmd/charter/` (`*_test.go`) | `go test`: the workflow templates, version tags, and a project made by `charter new` | `mise run charter:check` |

## Local and remote: both, with the same programs

Every test program takes a URL, so one file tests a local server and the deployed Worker. Both runs are needed, because each catches what the other can't:

| Where | What runs it | What it catches |
|---|---|---|
| Native Go build, and the Wasm under local workerd (`cf dev`) | `mise run check`, and the check workflows on every push to main and pull request ([dev.md](dev.md#github-workflows)) | Logic, the contract, TinyGo's gaps (`go test` can't see those) |
| The deployed Worker on Cloudflare | `mise run live-test`, `mise run live-test` (also the last step of the `deploy` workflow), and the soak tasks | What only production does. Go timers hung there and nowhere else: local workerd has a real clock, Cloudflare's only moves on I/O ([upstream.md](upstream.md)) |

A green local run is not a verdict on a deploy. Deploy through `deploy`, or run the live test yourself afterwards.

## What is not tested

- **The oRPC Worker has no local run of the live test.** `mise run check` is typecheck, unit tests and spec drift; its real-time paths are only tested deployed.
- **The Go showcase has no task for the deployed Worker.** The command is in [showcase-go.md](showcase-go.md#tasks-from-the-repo-root).
- **The soak is not in CI.** It redeploys a Worker and takes minutes; it is run by hand.
