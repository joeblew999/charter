---
title: Tasks
nav_order: 2
parent: Reference
---

# Tasks: every `mise run` task in a project

Every task of a project made by `charter new`: what it does, what it needs, and whether it stays on your machine. Read it to find the task for a job, or to know what a task will touch before you run it. The list is what `mise tasks` printed in a new project on 2026-10-01: 40 tasks.

```sh
mise tasks                            # every task, with its description
mise run check                        # run one
mise run sdk:gen typescript    # words after the task's name are passed to it
```

Every task is one line in `mise.toml`. Most call a command of the tool the tasks run (`charter`): [The charter tool](dev.md) has each command's flags.

## How to read the tables

- **Needs** is what must be there besides `mise install`. "npm packages" means `mise run setup` was run.
- **Local** tasks touch only your machine. **Remote** tasks read or change something on Cloudflare or GitHub. The task's own description starts with `REMOTE` for those.
- **A Cloudflare login** is `node_modules/.bin/cf auth login`, or the variables `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` ([Configuration](config.md#environment-variables)).

## The whole project

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `setup` | Installs the project's npm packages (the Cloudflare CLI `cf`, Fern, what the tests import), from its lockfile | Network | Local |
| `check` | Every local check: `lint`, `test`, `spec:check`, `sdk:publish:fresh`, `test:native`, `test:workerd`, `workflows:check` and `docs:check` | npm packages | Local |
| `doctor` | Checks what the tasks need: npm packages, Docker, Go, Rust, `gh`. Lists the SDK groups. Fails if npm packages are missing or Docker is not running | Nothing | Local |
| `upstream:status` | Lists every `Upstream:` tag in the code with the state of its issue. `CLOSED` means that workaround can go | A git repository, `gh` | Remote: reads issues on GitHub |

## The API

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `run` | Runs the API natively on `API_PORT` (5174), with an in-memory store. Notes are gone when it stops | Nothing | Local |
| `build` | Builds the Worker's Wasm into `build/`, tuned for Workers (`charter wasm-build`: [what it changes](dev.md#wasm-build)). Fails if `build/app.wasm` is over 3,000,000 bytes gzipped | Nothing | Local |
| `dev` | Runs `build`, then the Worker under workerd (`cf dev`) on `API_PORT`, with a local D1 database and the hub. After a Go change, run `build` in another shell: `cf dev` reloads | npm packages | Local |
| `migrate:local` | Applies `migrations/` to the local D1 database of a running `dev`, each file once | `dev` running | Local |
| `spec` | Writes `fern/openapi.json` and `fern/asyncapi.json` from the contract, with `API_URL` as their server | Nothing | Local |
| `spec:check` | Fails if a committed spec differs from what the contract gives | Nothing | Local |
| `lint` | `gofmt`, and `go vet` for your machine and for Wasm | Nothing | Local |
| `test` | `go test ./...` in the project's folder | Nothing | Local |
| `test:native` | Starts the native build on a free port and runs the real-time test (`test/live-test.mjs`) and the MCP test (`test/mcp-test.mjs`) against it | npm packages | Local |
| `test:workerd` | Runs `build`, starts the Wasm under workerd on a free port, applies the migrations, and runs the same two tests | npm packages | Local |
| `mcp-test` | Runs the MCP test against a server already running on `API_PORT`. It writes test notes | npm packages, `run` or `dev` running | Local |
| `deploy` | Runs `build`, deploys the Worker (`cf deploy`, which creates its D1 database), then applies pending migrations as `migrate` does | npm packages, a Cloudflare login | Remote: changes the Worker and its database |
| `migrate` | Applies pending files of `migrations/` to the D1 database `<name>-db` | npm packages, a Cloudflare login, the Worker deployed once | Remote: changes the database |
| `live-test` | Tests the deployed Worker at `API_URL`: SSE and the WebSocket, raw and through the TypeScript SDK, then MCP. It writes test notes. It generates the `typescript-dist` SDK first when that is missing | npm packages; Docker the first time | Remote: writes notes |
| `soak` | Runs every client against every real-time scenario on the deployed Worker, and redeploys it once in the middle. Add `--idle <minutes>` for the long-idle case. It generates the SDKs and builds the Fern CLI first when they are missing | npm packages, a Cloudflare login; Docker and `cargo` the first time | Remote: redeploys the Worker, writes notes |
| `bench` | Calls every GET operation of the deployed Worker that the spec has examples for, after 30 s of warm-up, and prints wall time, the CPU time Cloudflare recorded, and the CPU of each request. About a minute and a half | The Worker deployed; `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment or in fnox | Remote: read-only |
| `perf` | Runs `deploy`, then benches the new isolate from its first request: 8 requests at once, then every operation with writes, with the CPU of each request. One command to see what a change costs on Cloudflare. About two minutes | As `deploy` and `bench` | Remote: changes the Worker and its database, writes test notes |
| `perf:try` | `mise run perf:try -- -name <experiment>`: builds, deploys to a scratch Worker `<worker>-perf-<experiment>` with a database and hub of its own, benches it as `perf` does, and deletes it. `-build '<wasm-build flags>'` tries another build, `-keep` leaves the Worker. Several can run at once with different names. About 70 seconds | As `perf` | Remote: creates and deletes a scratch Worker and database |
| `perf:clean` | Deletes every scratch Worker `<worker>-perf-*` and its database that a run left | A Cloudflare login | Remote: deletes scratch Workers and databases only |

## SDKs

A project has one API: its Fern folder is `fern/`. A `<group>` is one SDK its `fern/generators.yml` defines: `go`, `typescript`, `typescript-dist` or `cli`.

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `sdk:list` | Lists the groups | Nothing | Local |
| `sdk:check-spec` | Validates the API's specs and settings with `fern check` | npm packages | Local |
| `sdk:gen <group>` | Generates one SDK with Fern into `sdk/out/<group>`, replacing what is there | npm packages, Docker | Local |
| `sdk:check <group>` | Proves the generated SDK in `sdk/out/<group>` works. Go: build, vet, and its tests against a WireMock container. TypeScript: a typecheck | The SDK generated; Docker for a Go SDK | Local |
| `sdk:ready [group...]` | Generates and builds what the tests use, only where it is missing: the `typescript-dist` and `go` SDKs and the Fern CLI, or only the groups named | npm packages; Docker and `cargo` when something is missing | Local |
| `sdk:publish` | Generates the Go SDK fresh, checks it as `sdk:check` does, and copies its sources into `sdk/go`: the committed Go module another repo fetches with `go get`. Commit the result | npm packages, Docker | Local |
| `sdk:publish:check` | Generates the Go SDK again and fails if `sdk/go` differs. Passes when there is no `sdk/go` yet | npm packages, Docker | Local |
| `sdk:publish:fresh` | Fails if the specs changed since `sdk/go` was generated from them. It compares a hash, so it is quick and needs no Docker; `check` runs it. Passes when there is no `sdk/go` yet | Nothing | Local |
| `sdk:cli:build [-linux]` | Builds the generated Rust CLI in `sdk/out/cli`. Heavy: the first build takes minutes at full CPU. `-linux` builds for Linux in Docker | The CLI generated, `cargo`; Docker with `-linux` | Local |
| `sdk:dist` | Generates the Go and TypeScript SDKs fresh, checks them, and archives them and the specs into `dist/` | npm packages, Docker | Local |
| `sdk:dist:cli [-linux]` | Generates and builds the API's Fern CLI into `dist/`. Heavy | npm packages, Docker, `cargo` | Local |
| `sdk:docs` | A local preview of the API docs Fern generates (`fern/docs.yml`) | npm packages | Local |
| `sdk:clean` | Removes `sdk/out/` and stops leftover WireMock containers | Nothing | Local |

## Docs

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `docs:setup` | Writes the docs site's config, style, `docs/writing.md` and `docs/llms.txt`, filled in with the repo's name and description | `gh`, and the repo on GitHub | Remote: reads the repo's name from GitHub |
| `docs:lint` | Checks `docs/` for what a program can check: front matter, links, tasks and paths that don't exist | A git repository | Local |
| `docs:check` | Fails if the docs site's config differs from what `docs:setup` writes, or if `docs:lint` finds a problem. Before `docs:setup` has been run, the first check passes | A git repository; `gh` and the repo on GitHub once `docs/_config.yml` exists | Local, then remote: reads the repo's name from GitHub |
| `docs:review` | Has Claude bring `docs/` into line with `docs/writing.md`. It edits files | The `claude` command | Local |
| `docs:pages` | Turns on GitHub Pages for `docs/` on `main`. Once per repo | `gh` with admin access to the repo | Remote: changes a GitHub setting |

## GitHub workflows, secrets and releases

What a release ships is built by `sdk:dist` and `sdk:dist:cli`, in [SDKs](#sdks) above.

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `workflows` | Writes the GitHub workflows into `.github/workflows/`. Never edit the written files | Nothing | Local |
| `workflows:check` | Fails if a workflow in `.github/workflows/` differs from what the tool writes | Nothing | Local |
| `cloudflare:token` | Fails unless `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` are set. The deploy workflow runs it first | Nothing | Local |
| `cloudflare:secrets` | Copies those two variables from fnox into the repo's GitHub Actions secrets, without printing them. Once per repo | `gh`, the two values stored in fnox | Remote: sets GitHub secrets |
| `release` | On a version tag in a GitHub workflow: attaches the files in `dist/` to the tag's GitHub Release. Anywhere else: a dry run that lists `dist/` | Files in `dist/`; `gh` for a real run | Remote on a tag; otherwise local |
| `release:tags` | On a version tag: when `sdk/go` exists, adds the tag `sdk/go/vX.Y.Z` on the same commit, which Go needs to find a version of that module (the project's own module is at the root, so `vX.Y.Z` is its version). Anywhere else: a dry run | `gh` for a real run | Remote on a tag; otherwise local |

## Tasks of the charter repo

The charter repo holds four projects in `examples/`, each with a `mise.toml` of its own and the task names above (a project has those that apply to it). `examples/notes-go` is the project `charter new` copies, so it has every task above. The repo's own `mise.toml` has only what is about the repo as a whole:

| Tasks | What they are for |
|---|---|
| `setup`, `examples:check`, `doctor`, `sdk:clean` | The same task in every example, one after the other (`charter each`) |
| `check` | `charter:check`, `go:check` and `examples:check`: every local check |
| `charter:check`, `workflows`, `charter:release`, `release:tags` | The tool itself: its checks, the workflows it writes into this repo, its release with GoReleaser ([Releases](releases.md)), and the tags of the modules in subdirectories (`go/vX.Y.Z`, `examples/notes-go/sdk/go/vX.Y.Z`) |
| `go:lint`, `go:test`, `go:check` | The Go library in `go/` |
| `compare` | The bench against the TypeScript notes Worker and then the Go one |
| `docs:setup`, `docs:lint`, `docs:review`, `docs:pages`, `upstream:status`, `cloudflare:secrets` | As in a project, for this repo |

## Limits

- **What was run for this page:** `mise tasks`, `spec:check`, `sdk:list`, `doctor` and `release:tags` (a dry run) were run in a new project on 2026-10-01. The other rows are read from the project's `mise.toml` and the tool's source.
