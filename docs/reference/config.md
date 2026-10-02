---
title: Configuration and pins
nav_order: 4
parent: Reference
---

# Configuration and pins: variables, bindings, files, versions

Everything a project can be set with, every file it has, and where each version is pinned. Read it to point a project at your Cloudflare account, to find which file to edit, or to move a pin. Paths are a project's; the values are those of `examples/notes-go/`.

## Environment variables

`mise.toml` sets the first two under `[env]`: each takes the value from your environment when it is set there, the default otherwise.

| Variable | Default | Who reads it |
|---|---|---|
| `API_PORT` | 5174 | `run`, `dev`, `mcp-test`, `charter migrate-local` |
| `API_URL` | `https://<name>.<subdomain>.workers.dev` | `spec` and `spec:check` (the server the specs name); `live-test`, `soak`, `bench`, `perf` (the Worker they call); `migrate` and the `perf` tasks (the Worker's name: the first label of the host) |
| `PORT` | none | The server. Natively it listens there, or on 9900. Under `cf dev`, `vite.config.ts` listens there, or on 5173. The tasks set it from `API_PORT` |
| `APP_NAME` | natively `<name> (go run)` | The hello operation. On Cloudflare it is a binding |
| `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` | none | `cf`, instead of `cf auth login`; the bench's CPU figures; the deploy workflow, as repository secrets |
| `FERN_TOKEN` | none | Fern. `mise run doctor` warns without it; local generation ran without one on 2026-10-01 |
| `GITHUB_REF_TYPE`, `GITHUB_REF_NAME`, `GH_TOKEN` | set by GitHub Actions | `charter release`, `release-tags`, `release-tool`: on a version tag they publish; `gh` |

`mise.local.toml` beside `mise.toml` overrides it on your machine and is not committed ([when to use it](../guides/deploy.md#the-workers-url)). Secrets go in neither file: git also ignores `.dev.vars*` and `.env*`.

The other examples: `examples/notes-ts/` uses port 5173, `examples/showcase-go/` 5175, `examples/showcase-ts/` 5176 and a second URL, `API_MOCK_URL`, for the Worker its WebSocket test connects to.

## Routes

| Path | What |
|---|---|
| `/api/hello`, `/api/notes`, `/api/notes/watch` | The notes API: REST and the SSE stream |
| `/api/notes/live` | The WebSocket |
| `/api/openapi.json`, `/api/asyncapi.json` | The specs, generated on request with the request's origin as their server |
| `/api/mcp` | The MCP endpoint |

`mise run run` and `mise run dev` serve them on `API_PORT`: run one at a time. `test:native` and `test:workerd` pick a free port each run.

## The Worker's bindings

`cloudflare.config.ts` declares the Worker: its name, its compatibility date, its entry (`worker.mjs`) and its bindings.

| Binding | What it is | How Go reads it (`platform_js.go`) |
|---|---|---|
| `DB` | The D1 database, named `<worker>-db` on Cloudflare | `d1.Open("DB")` |
| `HUB` | The Durable Object namespace of the hub, the library's class `Hub` | `hub.DurableObject[api.Note]("HUB", "notes")` |
| `APP_NAME` | A text variable: the Worker's name | `cloudflare.Getenv("APP_NAME")` |

The handlers never touch a binding: they get an `api.Env` (`Var`, `Store`, `Hub`), filled by `platform_js.go` on Cloudflare and `platform_other.go` natively. `Store` and `Hub` are opened per request.

**To add a binding:** declare it in `cloudflare.config.ts`, read it in `platform_js.go`, give the native build a stand-in in `platform_other.go`, and add a field to `api.Env`.

## The files of a project

**Edit** marks what you change; **generated** what a task writes ([what is generated](../README.md#what-is-generated)).

| Path | What it is |
|---|---|
| `mise.toml` | The tasks, the tool pins, `API_PORT` and `API_URL`. Edit |
| `go.mod`, `go.sum` | The module, the library's pin |
| `main.go` | The entry of both builds: `transport.Run(api.Handler(env()))` |
| `platform_js.go`, `platform_other.go` | The bindings on Cloudflare; the stand-ins natively |
| `api/contract.go` | **The contract.** Edit |
| `api/handlers.go` | The contract implemented, the specs' routes, `/api/mcp`. Edit |
| `api/store.go`, `api/store_js.go` | The storage: in memory, and on D1. Edit |
| `api/spec.go` | Both specs from the contract |
| `api/*_test.go` | The Go tests. Edit |
| `./cmd/spec/main.go` | The command behind `mise run spec` |
| `worker.mjs` | The Worker's entry. It imports the glue from `build/` |
| `cloudflare.config.ts`, `vite.config.ts` | The Worker; the dev server's port |
| `package.json`, `package-lock.json` | The pins of `cf`, Fern and what the tests import |
| `migrations/` | The D1 schema: a file per change. Edit |
| `fern/generators.yml`, `fern/fern.config.json`, `fern/docs.yml` | Fern's settings. Edit |
| `fern/openapi.json`, `fern/asyncapi.json` | Generated: the specs |
| `sdk/go/` | Generated: the committed Go SDK |
| `test/` | The test programs a deploy must pass |
| `docs/`, `README.md`, `AGENTS.md`, `CLAUDE.md` | Written once by `charter new`. Yours, but `docs/writing.md` is generated |
| `.github/workflows/` | Generated: the four workflows |
| `build/`, `sdk/out/`, `dist/`, `node_modules/` | Written by tasks, not committed |

## Pinned versions

Every tool has an exact version in one file. Versions below are those of 2026-10-02.

| What | Version | Where the pin lives |
|---|---|---|
| The tool (`charter`) | the release the project was made from | `mise.toml`: `"go:github.com/joeblew999/charter/cmd/charter"` |
| The library | the same release | `go.mod`: `github.com/joeblew999/charter/go` |
| TinyGo | 0.42.0 | The tool (`cmd/charter/wasm.go`), beside the patches made for it. A project does not pin TinyGo |
| binaryen (`wasm-opt`, which TinyGo runs) | 133 | The tool, beside TinyGo |
| Go, Node, Rust, gh, fnox | 1.27.1, 26.10.0, 1.98.1, 2.101.0, 1.35.2 | `mise.toml` |
| Huma, workers-go | 2.39.1, 0.36.0 | `go.mod` |
| The Cloudflare CLI (`cf`), Fern's CLI (`fern-api`) | 1.0.0-beta.5, 5.140.0 | `package.json` |
| Fern's generators | Go SDK 1.64.0, TypeScript SDK 3.98.0, CLI 0.45.1 | `fern/generators.yml` |
| oRPC, in the TypeScript examples | 2.0.0-beta.40 | their `package.json` |
| The Workers compatibility date | 2026-09-25 | `cloudflare.config.ts` |
| Actions and runners in the workflows | exact versions | The tool's templates (`cmd/charter/workflows/`) |

- **Lockfiles are committed.**
- **Moving TinyGo is a change to the tool,** tested with its patches. Projects get it by updating the tool ([Releases](releases.md#how-a-project-updates)).
- **Not exact:** `vite` is a range and `@cloudflare/vite-plugin` follows the `beta` tag in `package.json`. The lockfile holds what was installed.
- **A project made with `charter new -from <checkout>`** pins nothing for the first two rows: its tasks run the tool from the checkout, and `go.mod` has a `replace` line. Remove both once you depend on a release.
