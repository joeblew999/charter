---
title: Test and CI
nav_order: 8
parent: Guides
---

# Test and CI: locally, on Cloudflare, on GitHub

How to know a change is right: which tests a project has, what each proves, where each runs, and how to add your own.

```sh
mise run check        # LOCAL: everything that needs no Cloudflare account
mise run deploy       # REMOTE
mise run live-test    # REMOTE: the same test programs against the deployed Worker
```

## What each test proves

| Task | Where | What it proves |
|---|---|---|
| `lint`, `test`, `spec:check` | Local | The code compiles for the host and for Wasm; the handlers do what the contract says; the committed specs are the contract's |
| `test:native` | Local | SSE, the WebSocket and MCP work over real HTTP, against the native build |
| `test:workerd` | Local | The build that ships works: TinyGo, the size limit, D1, the hub, under workerd |
| `live-test` | Cloudflare; writes test notes | The same programs, and the generated TypeScript SDK, on Cloudflare |
| `soak` | Cloudflare; **redeploys the Worker** | No stream loses, repeats or reorders a note ([the matrix](../realtime.md#how-it-is-tested)) |
| `bench` | Cloudflare; read-only | What a request costs ([performance](performance.md)) |

`mise run check` runs the local ones, and checks that nothing generated was edited by hand. What every task does: [Tasks](../reference/tasks.md).

- **The local server tasks pick a free port** and stop what they start, so two checks can run at once. They are silent when they pass.
- **`go test` cannot see what TinyGo lacks.** That only shows in `test:workerd`.
- **The remote tests need generated SDKs,** and make missing ones first (Docker). A stale one is not regenerated: after a contract change, `mise run sdk:clean`.
- **The soak is in no workflow.** Run it by hand after changing a stream, the hub or the Worker's entry. Its flags go after the task's name: `--idle <minutes>`, `--no-deploy`, `--seconds`, `--deploy-at`, `--drop-at`, `--stream-seconds`.
- **One program against a server you started:** `node test/live-test.mjs http://localhost:5174`.

## Why both local and deployed

The local runs catch logic, the contract, what TinyGo lacks and the Wasm size. They cannot catch what Cloudflare's own runtime does differently. The case that made this a rule: Go timers hung on Cloudflare and not under local workerd. 6 of 10 streams with a 2-second limit never ended, and every local check passed ([tinygo-org/tinygo#5798](../upstream.md)). A green `mise run check` is not a verdict on a deploy.

## On GitHub

`charter new` writes four workflows into `.github/workflows/`. Every step that does work is a mise task, so a failing step runs the same on your machine.

| Workflow | When | What it runs |
|---|---|---|
| `check` | Every push to main and every pull request | `setup`, `check` |
| `sdk-check` | The same | For the groups `go` and `typescript`: `sdk:gen`, `sdk:check`. Then `sdk:publish:check` |
| `deploy` | By hand | `cloudflare:token`, `setup`, `deploy`, `live-test` ([Deploy](deploy.md#deploy-from-github)) |
| `release` | A version tag; a dry run on a pull request | `sdk:dist`, `sdk:dist:cli`, `release`, `release:tags` ([Release](sdks.md#release)) |

- **Never edit a workflow.** `mise run workflows` writes them again from the tool's templates, and `mise run check` fails if they differ.
- **The docs site:** `mise run docs:setup` writes its config, `mise run docs:pages` turns GitHub Pages on for `docs/`, `mise run docs:lint` checks the pages.

## Add a test of your own

**A Go test,** in `api/`. `api/api_test.go` has the helpers: `server(t)` starts the handlers on a real HTTP server with an in-memory store, and `do(t, method, url, body)` returns the status, the body and the headers.

```go
func TestCreateRefusesAnEmptyBody(t *testing.T) {
	srv, _ := server(t)
	if status, body, _ := do(t, "POST", srv.URL+"/api/notes", `{"body":""}`); status != 422 {
		t.Fatalf("HTTP %d %s", status, body)
	}
}
```

**A Node program that takes a URL,** for what must hold over real HTTP and on Cloudflare. Print a `PASS` or `FAIL` line per check and exit with 1 if one failed. Add it to three tasks in `mise.toml`:

- **`test:native` and `test:workerd`:** another `-run "node test/hello-test.mjs http://localhost:{port}"` at the end. The tool replaces `{port}`.
- **`live-test`:** `&& node test/hello-test.mjs "$API_URL"` at the end.

## What is not tested

The Go SDK and the CLI against your server, except in the soak. Load: `bench` sends requests one after another. A pull request's deploy: no workflow deploys a branch.
