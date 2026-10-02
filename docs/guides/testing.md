---
title: Test locally and on Cloudflare
nav_order: 7
parent: Guides
---

# Test locally and on Cloudflare

This page gets you from "I changed something" to "it is right, on my machine and on Cloudflare". It lists the tests a project has, what each one proves, and how to add your own. Read it before you deploy a change.

The local commands and their output below were run on 2026-10-01 in a project made by `dev new -name billing-api`. The three remote tasks were not run for this page: what they do is read from `mise.toml` and `test/`, and their results are those recorded for orpc-api's own Go Worker ([findings](../findings.md)).

## The short version

```sh
mise run check               # LOCAL: everything below that needs no Cloudflare account. About half a minute
mise run api:go:deploy       # REMOTE
mise run api:go:live-test    # REMOTE: the same test programs against the deployed Worker
```

## Local tests

| Task | What it runs | What it proves |
|---|---|---|
| `mise run api:go:lint` | `gofmt`, and `go vet` for your machine and for Wasm | The code is formatted and compiles for both targets |
| `mise run api:go:test` | `go test ./...` in `api/go/` | The handlers do what the contract says: status codes, paging, the stream's catch-up and end, the MCP tools |
| `mise run api:go:spec:check` | The spec writer in compare mode | The committed specs are what the contract gives now |
| `mise run api:go:test:native` | Starts the API natively, then the live test and the MCP test against it | SSE, the WebSocket and MCP work over real HTTP, with real clients |
| `mise run api:go:test:workerd` | Builds the Wasm with TinyGo, starts it under the local Workers runtime (workerd) with a local D1, applies the migrations, then the same two programs | The build that ships works: TinyGo, the Wasm size limit, D1 and the hub |
| `mise run api:go:check` | The five above | |
| `mise run check` | `api:go:check` and `dev:check` (workflows, docs config and docs lint) | |

- **The two server tasks pick a free port,** start the server, wait until it answers, run the programs, and stop it. Nothing is left running, and two checks can run at once.
- **They are silent when they pass.** A failing program's output is shown.
- **`go test` cannot see what TinyGo lacks.** It builds with the standard Go compiler. A package that TinyGo cannot build, or a function it has not implemented, only shows in `api:go:test:workerd`.
- **The native run has no database:** notes are in memory. The workerd run uses a local D1, created from `migrations/`.

The end of `mise run check` here:

```
[api:go:spec:check] specs match the Go contract
[api:go:test] ok  	github.com/acme/billing-api/api/go/api	9.717s
[api:go:build] api/go/build/app.wasm: 2468140 B, 874803 B gzipped (limit 3000000)
[api:go:test:workerd] Finished in 8.08s
Finished in 31.20s
```

To run one program against a server you started yourself (`mise run api:go:run` in another shell):

```sh
node test/live-test.mjs http://localhost:5174    # 3 checks: SSE, WebSocket, resume after a reconnect
node test/mcp-test.mjs http://localhost:5174     # 29 checks: the MCP endpoint with the official TypeScript client
```

```
PASS  SSE /api/notes/watch: {"id":1,"body":"live 1790847371588","created_at":"2026-10-01 09:36:13"}
PASS  WebSocket /api/notes/live (Durable Object): {"id":1,"body":"live 1790847371588","created_at":"2026-10-01 09:36:13"}
PASS  SSE resume: missed note replayed after reconnect (Last-Event-ID): {"id":3,"body":"live 1790847371588 b","created_at":"2026-10-01 09:36:14"}
```

## Tests against the deployed Worker

They call `API_GO_URL` ([the Worker's URL](deploy.md#the-workers-url)).

| Task | What it does to the Worker | What it proves |
|---|---|---|
| `mise run api:go:live-test` | Writes a few test notes | The live test, the same through the generated TypeScript SDK (`test/sdk-live-test.mjs`), and the MCP test, all pass on Cloudflare |
| `mise run api:go:soak` | Writes a test note every 2 seconds and **redeploys the Worker** in the middle | No stream loses, repeats or reorders a note, whatever happens to the connection |
| `mise run api:go:bench` | Read-only | What a request costs in wall time ([what it costs to run](deploy.md#what-it-costs-to-run)) |

- **They need generated SDKs,** and make them first if they are missing (`mise run sdk:ready`): the TypeScript SDK for the live test; for the soak also the Go SDK and the CLI, which is a Rust build. That needs Docker.
- **A missing SDK is generated, a stale one is not.** After a contract change, run `mise run sdk:clean` before these tasks.
- **The test notes stay** in the database. Their bodies start with `live`, `sdk live`, `mcp` or `soak`.

### The soak

```sh
mise run api:go:soak             # about two minutes
mise run api:go:soak --idle 20   # the long-idle case: a little over 20 minutes, no redeploy
```

Seven clients follow the stream at once: raw SSE with `after`, raw SSE with `Last-Event-ID`, the TypeScript SDK's `notes.watch()`, the Go SDK's `Notes.Watch()`, the CLI's `notes watch`, a raw WebSocket, and the TypeScript SDK's `liveNotes.connect()`. Each follows one rule: when the stream ends, for any reason, call again from the last note id received.

A note is created every 2 seconds for 100 seconds, while four things happen:

| Scenario | When | What it stands for |
|---|---|---|
| Planned end | Every SSE stream ends after 15 seconds | The normal end of a stream |
| Hub restart | At 30 seconds the Worker is redeployed (`mise run api:go:deploy`) | A deploy while clients are connected |
| Client drop | At 65 seconds every client is cut off for 6 seconds | A network failure |
| Long idle (`--idle 20` only) | 20 minutes with no notes, then one | A quiet stream that must still deliver |

A client passes when it received every note, exactly once, in order. A line per client:

```
PASS  Go SDK Notes.Watch(): 14/14, missing 0, duplicates 0, in order, latency p50 0.0s max 6.1s, 5 connections
        ends: exit 0 ×3, exit null ×2
```

That line is from a run against the native server on this machine, without the redeploy:

```sh
mise run sdk:ready api-go        # the SDKs and the CLI the soak uses, if missing
node test/soak.mjs http://localhost:5174 --sdk api-go --no-deploy --seconds 30 --drop-at 15 --stream-seconds 10
```

All seven clients passed it. On Cloudflare, the full soak with the redeploy and the 20-minute idle case passed for orpc-api's own Go Worker; neither was run for a new project.

Anything after the task's name goes to the program: `--seconds`, `--deploy-at`, `--drop-at`, `--stream-seconds`, `--idle`, `--no-deploy`.

At this commit the redeploy inside the soak ends with an error in a new project, because its migration step fails ([Deploy to Cloudflare](deploy.md#migrations)). Read from the code, not run: the Worker is redeployed before that step, and the soak prints `redeploy exited 1` and goes on.

The soak is not in any GitHub workflow: it redeploys and takes minutes. Run it by hand after changing anything about streams, the hub, or the Worker's JavaScript (`api/go/worker.mjs`, and the library's `go/worker/`).

## Why both local and remote

Every test program takes a URL, so the same file tests a local server and the deployed Worker. Both runs are needed, because each catches what the other cannot.

| Where | What it catches | What it cannot |
|---|---|---|
| Natively and under local workerd | Logic, the contract, what TinyGo lacks, the Wasm size | Anything Cloudflare's own runtime does differently |
| The deployed Worker | What only production does | Nothing before the deploy |

The case that made this a rule: Go timers hung on Cloudflare and not under local workerd. 6 of 10 streams with a 2-second limit never ended, because the production clock moves in whole milliseconds. Every local check passed. The soak against the deployed Worker showed it.

A green `mise run check` is not a verdict on a deploy. Run the live test after each one.

## Add a test of your own

### A Go test, beside the handlers

Put it in `api/go/api/`, in the package `api`. `api/go/api/api_test.go` has the helpers: `server(t)` starts the handlers on a real HTTP server with an in-memory store, and `do(t, method, url, body)` returns the status, the body and the headers.

```go
package api

import "testing"

func TestCreateRefusesAnEmptyBody(t *testing.T) {
	srv, _ := server(t)
	status, body, _ := do(t, "POST", srv.URL+"/api/notes", `{"body":""}`)
	if status != 422 {
		t.Fatalf("HTTP %d %s", status, body)
	}
}
```

`mise run api:go:test` runs it, and so does `mise run check`. Nothing to register.

### A Node test program, taking a URL

For what must be true over real HTTP, on Cloudflare as well. Take the URL as the argument, print a `PASS` or `FAIL` line per check, and exit with 1 if one failed. Save it in `test/` as `hello-test.mjs`:

```js
// GET /api/hello answers with the greeting. Usage: node test/hello-test.mjs <url>
const origin = process.argv[2];
const res = await fetch(`${origin}/api/hello`);
const ok = res.status === 200 && (await res.json()).message.startsWith("Hello from");
console.log(`${ok ? "PASS" : "FAIL"}  GET /api/hello`);
process.exit(ok ? 0 : 1);
```

Then add it to three tasks in `mise.toml`, so it runs natively, under workerd and deployed:

- **`api:go:test:native` and `api:go:test:workerd`:** add another `-run "node ../../test/hello-test.mjs http://localhost:{port}"` at the end of the line. The tool replaces `{port}` with the port it chose.
- **`api:go:live-test`:** add `&& node test/hello-test.mjs "$API_GO_URL"` at the end.

Both the Go test and the program above were run as written, natively. To use the generated SDK in a test program, import it the way `test/sdk-live-test.mjs` does. Packages a program needs (`ws`, the MCP client) are installed in `sdk/`, and loaded from there as `test/live-test.mjs` does.

## What is not tested

- **The Go SDK and the CLI against your server,** except in the soak.
- **Load.** `api:go:bench` times 20 requests one after another.
- **The pull request's deploy.** No workflow deploys a branch; the live test runs against the one deployed Worker.
