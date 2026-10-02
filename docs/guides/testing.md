---
title: Test and CI
nav_order: 8
parent: Guides
---

# Test and CI: locally, on Cloudflare, on GitHub

How to know a change is right: which tests a project has, what each proves, where each runs, and how to add your own. Read it before you deploy a change.

```sh
mise run check        # LOCAL: everything that needs no Cloudflare account
mise run deploy       # REMOTE
mise run live-test    # REMOTE: the same test programs against the deployed Worker
```

## Local tests

| Task | What it runs | What it proves |
|---|---|---|
| `mise run lint` | `gofmt`, and `go vet` for the host and for Wasm | The code compiles for both targets |
| `mise run test` | `go test ./...` | The handlers do what the contract says; the MCP tools; the contract's rules (examples, tools) |
| `mise run spec:check` | The spec command in compare mode | The committed specs are what the contract gives |
| `mise run sdk:publish:fresh` | A hash comparison | `sdk/go/` was made from the specs as they are |
| `mise run test:native` | Starts the API natively, then the live test and the MCP test against it | SSE, the WebSocket and MCP work over real HTTP |
| `mise run test:workerd` | Builds the Wasm, starts it under workerd with a local D1, migrates, then the same two programs | The build that ships works: TinyGo, the size limit, D1, the hub |
| `mise run workflows:check`, `mise run docs:check` | Comparisons with the tool's templates, and the docs lint | Nothing generated was edited by hand |
| `mise run check` | All of the above | |

- **The two server tasks pick a free port,** start the server, run the programs and stop it. Two checks can run at once.
- **They are silent when they pass.** A failing program's output is shown.
- **`go test` cannot see what TinyGo lacks.** A package TinyGo cannot build only shows in `test:workerd`.

To run one program against a server you started (`mise run run`): `node test/live-test.mjs http://localhost:5174`.

## Tests against the deployed Worker

They call `API_URL` ([the Worker's URL](deploy.md#the-workers-url)).

| Task | What it does to the Worker | What it proves |
|---|---|---|
| `mise run live-test` | Writes a few test notes | SSE and the WebSocket, raw and through the generated TypeScript SDK, then MCP: on Cloudflare |
| `mise run soak` | Writes a note every 2 seconds and **redeploys the Worker** in the middle | No stream loses, repeats or reorders a note ([the matrix](../realtime.md#how-it-is-tested)) |
| `mise run bench` | Read-only | What a request costs ([Measure and improve performance](performance.md)) |

- **They need generated SDKs,** and make missing ones first (Docker): the TypeScript SDK for the live test; the Go SDK and the CLI too for the soak.
- **A stale SDK is not regenerated.** After a contract change: `mise run sdk:clean`.
- **The test notes stay** in the database.

The soak takes flags after the task's name: `--idle <minutes>` (the long-idle case, no redeploy), `--no-deploy`, `--seconds`, `--deploy-at`, `--drop-at`, `--stream-seconds`. It is in no workflow: run it by hand after changing a stream, the hub or the Worker's entry.

## Why both

| Where | What it catches | What it cannot |
|---|---|---|
| Natively and under local workerd | Logic, the contract, what TinyGo lacks, the Wasm size | What Cloudflare's own runtime does differently |
| The deployed Worker | What only production does | Anything before the deploy |

The case that made this a rule: Go timers hung on Cloudflare and not under local workerd. 6 of 10 streams with a 2-second limit never ended, and every local check passed ([tinygo-org/tinygo#5798](../upstream.md)). A green `mise run check` is not a verdict on a deploy.

## On GitHub

`charter new` writes four workflows into `.github/workflows/`. Every step that does work is a mise task, so a failing step runs the same on your machine.

| Workflow | When | What it runs |
|---|---|---|
| `check` | Every push to main and every pull request | `setup`, `check` |
| `sdk-check` | The same | For the groups `go` and `typescript`: `sdk:gen`, `sdk:check`. Then `sdk:publish:check` |
| `deploy` | By hand | `cloudflare:token`, `setup`, `deploy`, `live-test` ([Deploy](deploy.md#deploy-from-github)) |
| `release` | A version tag; a dry run on a pull request | `sdk:dist`, `sdk:dist:cli`, `release`, `release:tags` ([Release](sdks.md#release)) |

- **Never edit a workflow.** `mise run workflows` writes them again from the tool's templates, and `mise run check` fails if they differ. A newer tool may bring newer templates ([Releases](../reference/releases.md)).
- **The docs site:** `mise run docs:setup` writes its config, `mise run docs:pages` turns GitHub Pages on for `docs/`, `mise run docs:lint` checks the pages, `mise run docs:review` has Claude review them.

## Add a test of your own

**A Go test,** in `api/`, package `api`. `api/api_test.go` has the helpers: `server(t)` starts the handlers on a real HTTP server with an in-memory store, and `do(t, method, url, body)` returns the status, the body and the headers.

```go
func TestCreateRefusesAnEmptyBody(t *testing.T) {
	srv, _ := server(t)
	status, body, _ := do(t, "POST", srv.URL+"/api/notes", `{"body":""}`)
	if status != 422 {
		t.Fatalf("HTTP %d %s", status, body)
	}
}
```

**A Node program that takes a URL,** for what must hold over real HTTP and on Cloudflare. Print a `PASS` or `FAIL` line per check and exit with 1 if one failed. Then add it to three tasks in `mise.toml`:

- **`test:native` and `test:workerd`:** another `-run "node test/hello-test.mjs http://localhost:{port}"` at the end. The tool replaces `{port}`.
- **`live-test`:** `&& node test/hello-test.mjs "$API_URL"` at the end.

## What is not tested

- **The Go SDK and the CLI against your server,** except in the soak.
- **Load.** `bench` sends requests one after another.
- **A pull request's deploy.** No workflow deploys a branch.
