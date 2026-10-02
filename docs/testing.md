---
title: Test programs (examples/notes-go/test/)
nav_order: 7
parent: This repository
---
# examples/notes-go/test/: the tests both servers must pass

One set of test programs for the oRPC Worker (`examples/notes-ts/`) and the Go Worker (`examples/notes-go/`), plus one for the two showcases. They only know the server's URL and, where they use a generated SDK, the project they are run in (its `sdk/out/` and its `node_modules/`), so they also check that the servers are interchangeable. Read this page to run a test, to see what a check covers, or before deploying.

## Running them

```sh
mise run check             # LOCAL, at the root: every local check (Docker, no Cloudflare account)
mise run live-test         # REMOTE, in examples/notes-ts or examples/notes-go: the deployed Worker (the Go one: then its MCP endpoint)
mise run soak              # REMOTE, in the same two, redeploys the Worker: the real-time matrix, with the SDKs and CLI made from that project's specs
```

At the root, `mise run check` is `charter:check`, `go:check` and each example's own `check`, one example after the other. The checks that start a server pick free ports themselves, so several can run at once.

The remote tasks write test notes into the deployed Worker's database, and the soak tasks redeploy it. Anything after the task name goes to the program: `mise run soak --idle 20`.

## The test programs

| Path | What it does | Run by |
|---|---|---|
| `examples/notes-go/test/live-test.mjs` | Quick check with raw clients: an SSE stream and a WebSocket both receive a new note, then SSE resumes with `Last-Event-ID` after a reconnect (3 checks) | `mise run live-test` in both notes examples; locally inside the Go one's `mise run check`, natively and under workerd |
| `examples/notes-go/test/sdk-live-test.mjs` | The same through Fern's TypeScript SDK: `notes.watch()` and `liveNotes.connect()` (2 checks) | the two `live-test` tasks |
| `examples/notes-go/test/mcp-test.mjs` | The MCP endpoint (`/api/mcp`, the Go server only) with the official TypeScript MCP client, in both protocol eras: the tools are the contract's operations, and a tool call does what the REST route does ([mcp.md](mcp.md)) | `mise run mcp-test` against a running server; inside `mise run check`; and `mise run live-test` |
| `examples/showcase-go/test/showcase-test.mjs` | A showcase server through Fern's TypeScript SDK, every Fern feature: OAuth, pagination, idempotency, SSE, upload, the signed webhook, the WebSocket both ways, audiences. Unless `--open` is given it also checks that tokens are enforced ([showcase-go.md](showcase-go.md)) | `mise run showcase-test` in `examples/showcase-go` against a running server; inside its `mise run check`, natively and under workerd |
| `examples/notes-go/test/soak.mjs` | The real-time matrix: 7 clients through a planned end, a hub restart by redeploy, a client drop and a long idle. PASS is every note exactly once, in order ([realtime.md](realtime.md#how-it-is-tested)) | `mise run soak` in both notes examples |
| `examples/notes-go/test/soak-go/` | The Go SDK client that `examples/notes-go/test/soak.mjs` runs: one `Notes.Watch()` call | built by `examples/notes-go/test/soak.mjs` |

Run directly, each takes a URL, and is run from the folder of the project it tests (here: in `examples/notes-go`, and for the showcase test in `examples/showcase-go`):

```sh
node test/live-test.mjs <url>
node test/sdk-live-test.mjs <url>
node test/mcp-test.mjs <url>
node test/showcase-test.mjs <url> [--webhook-port <port>] [--open]
node test/soak.mjs <url> [--no-deploy] [--seconds 100] [--deploy-at 30] [--drop-at 65] [--stream-seconds 15] [--idle 20]
```

- **They need the project's npm packages** (`mise run setup`): `ws` and the MCP client are imported from the `node_modules/` of the folder they are run in.
- **The SDK tests need the generated SDKs** in that project's `sdk/out/`. The tasks make them first with `mise run sdk:ready`, which needs Docker the first time; the soak also needs the Fern CLI built (Rust).
- **`--webhook-port`** makes the showcase test listen for the webhook: start the server with `WEBHOOK_URL=http://localhost:<port>/webhook`. `mise run showcase-test` leaves that check out.
- **`--open`** is for the oRPC showcase, which checks no tokens. Its URL ends in `/api/mock`.
- **`--no-deploy`** runs the soak without the hub restart; `--idle <minutes>` runs only the long-idle case.

## Unit tests

They live with the code.

| Where | Runner | Task |
|---|---|---|
| `examples/notes-ts/test/` | vitest, in Node: `follow()` | `mise run test` there |
| `examples/notes-go/`, `examples/showcase-go/` (`*_test.go` in each package) | `go test`: the API, the MCP endpoint, the Go showcase and its SDK surface against the oRPC showcase's | `mise run test` in each |
| `go/` (`*_test.go` in each package) | `go test`: the library | `mise run go:test` |
| `examples/surface_test.go` | `go test`: the two notes examples' committed specs give Fern the same surface | `mise run charter:check` |
| `examples/showcase-ts/test/` | Node's test runner: the oRPC showcase server's routes, and its generated specs against the surface of the hand-written ones | `mise run test` there |
| `cmd/charter/` (`*_test.go`) | `go test`: what marks a project, the workflow templates, version tags, the examples' pins and groups, and a project made by `charter new` | `mise run charter:check` |

## Local and remote: both, with the same programs

Every test program takes a URL, so one file tests a local server and the deployed Worker. Both runs are needed, because each catches what the other can't:

| Where | What runs it | What it catches |
|---|---|---|
| Native Go build, and the Wasm under local workerd (`cf dev`) | `mise run check`, and the check workflows on every push to main and pull request ([dev.md](dev.md#github-workflows)) | Logic, the contract, TinyGo's gaps (`go test` can't see those) |
| The deployed Worker on Cloudflare | `mise run live-test` in each notes example (also the last step of the `deploy` workflow), and the soak tasks | What only production does. Go timers hung there and nowhere else: local workerd has a real clock, Cloudflare's only moves on I/O ([upstream.md](upstream.md)) |

A green local run is not a verdict on a deploy. Deploy through `deploy`, or run the live test yourself afterwards.

## What is not tested

- **The oRPC Worker has no local run of the live test.** `mise run check` is typecheck, unit tests and spec drift; its real-time paths are only tested deployed.
- **The Go showcase has no task for the deployed Worker.** The command is in [showcase-go.md](showcase-go.md#tasks-from-the-repo-root).
- **The soak is not in CI.** It redeploys a Worker and takes minutes; it is run by hand.
