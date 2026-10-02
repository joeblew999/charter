---
title: Configuration and pins
nav_order: 4
parent: Reference
---

# Configuration and pins: variables, bindings, files, versions

Everything a project can be set with, every file it has, and where each version is pinned. Paths are a project's; the values are those of `examples/notes-go/`.

## Environment variables

`mise.toml` sets the first two under `[env]`: your environment's value when set, the default otherwise.

| Variable | Default | Who reads it |
|---|---|---|
| `API_PORT` | 5174 | `run`, `dev`, `mcp-test`, `migrate:local` |
| `API_URL` | `https://<name>.<subdomain>.workers.dev` | `spec`, `spec:check`: the server the specs name. `live-test`, `soak`, `bench`, `perf`: the Worker they call. `migrate`, the `perf` tasks: the Worker's name, the first label of the host |
| `PORT` | none | The server: natively it listens there, or on 9900. The tasks set it from `API_PORT` |
| `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` | none | `cf`, instead of `cf auth login`; the bench's CPU figures; the deploy workflow, as repository secrets |
| `FERN_TOKEN` | none | Fern. `mise run doctor` warns without it; generation ran without one on 2026-10-01 |
| `GITHUB_REF_TYPE`, `GITHUB_REF_NAME` | set by GitHub Actions | The release commands: on a version tag they publish |

- **`mise.local.toml`** beside `mise.toml` overrides it on your machine and is not committed ([when to use it](../guides/deploy.md#the-workers-url)).
- **Secrets go in neither file.** Git also ignores `.dev.vars*` and `.env*`.
- **The other examples** use ports 5173 (`examples/notes-ts/`), 5175 (`examples/showcase-go/`) and 5176 (`examples/showcase-ts/`, which also has `API_MOCK_URL`: the second Worker its WebSocket test connects to).

## The Worker's bindings

`cloudflare.config.ts` declares the Worker: its name, compatibility date, entry (`worker.mjs`) and bindings.

| Binding | What it is | How Go reads it (`platform_js.go`) |
|---|---|---|
| `DB` | The D1 database, `<worker>-db` on Cloudflare | `d1.Open("DB")` |
| `HUB` | The Durable Object namespace of the hub, the library's class `Hub` | `hub.DurableObject[api.Note]("HUB", "notes")` |
| `APP_NAME` | A text variable: the Worker's name | `cloudflare.Getenv("APP_NAME")` |

The handlers never touch a binding: they get an `api.Env` (`Var`, `Store`, `Hub`), filled by `platform_js.go` on Cloudflare and `platform_other.go` natively.

**To add a binding:** declare it in `cloudflare.config.ts`, read it in `platform_js.go`, give the native build a stand-in in `platform_other.go`, add a field to `api.Env`.

## The files of a project

| Path | What it is | You |
|---|---|---|
| `mise.toml` | The tasks, the tool pins, `API_PORT`, `API_URL` | edit |
| `api/contract.go` | **The contract** | edit |
| `api/handlers.go` | The contract implemented, the specs' routes, `/api/mcp` | edit |
| `api/store.go`, `api/store_js.go` | The storage: in memory, and on D1 | edit |
| `api/spec.go`, `./cmd/spec/main.go` | Both specs from the contract; the command behind `mise run spec` | leave |
| `api/*_test.go`, `test/` | The Go tests; the programs a deploy must pass | edit |
| `main.go` | The entry of both builds | leave |
| `platform_js.go`, `platform_other.go` | The bindings on Cloudflare; the stand-ins natively | edit for a binding |
| `worker.mjs`, `cloudflare.config.ts`, `vite.config.ts` | The Worker's entry, its declaration, the dev server's port | edit for a binding |
| `go.mod`, `package.json` and their lockfiles | The pins | update on purpose |
| `migrations/` | The D1 schema: a file per change | add files |
| `fern/generators.yml`, `fern/fern.config.json`, `fern/docs.yml` | Fern's settings | edit |
| `docs/`, `README.md`, `AGENTS.md`, `CLAUDE.md` | Written once by `charter new` | edit |

Everything else is generated: [What is generated](../README.md#what-is-generated).

## Pinned versions

Every tool has an exact version in one place. Versions are those of 2026-10-02.

| What | Version | Where the pin lives |
|---|---|---|
| The tool (`charter`) | the release the project was made from | `mise.toml`: `"go:github.com/joeblew999/charter/cmd/charter"` |
| The library | the same release | `go.mod`: `github.com/joeblew999/charter/go` |
| TinyGo, binaryen (`wasm-opt`) | 0.42.0, 133 | The tool (`cmd/charter/wasm.go`), beside the patches made for that TinyGo. A project does not pin TinyGo |
| Go, Node, Rust, gh, fnox | 1.27.1, 26.10.0, 1.98.1, 2.101.0, 1.35.2 | `mise.toml` |
| Huma, workers-go | 2.39.1, 0.36.0 | `go.mod` |
| `cf`, Fern's CLI (`fern-api`) | 1.0.0-beta.5, 5.140.0 | `package.json` |
| Fern's generators | Go SDK 1.64.0, TypeScript SDK 3.98.0, CLI 0.45.1 | `fern/generators.yml` |
| oRPC, in the TypeScript examples | 2.0.0-beta.40 | their `package.json` |
| The Workers compatibility date | 2026-09-25 | `cloudflare.config.ts` |
| Actions and runners | exact versions | The tool's workflow templates |

- **Moving TinyGo is a change to the tool,** tested with its patches. Projects get it by updating the tool ([Releases](releases.md#how-a-project-updates)).
- **Not exact:** `vite` is a range and `@cloudflare/vite-plugin` follows the `beta` tag. The lockfile holds what was installed.
- **A project made with `charter new -from <checkout>`** pins nothing for the first two rows: both point at the checkout.
