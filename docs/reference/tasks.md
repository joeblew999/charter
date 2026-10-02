---
title: Tasks
nav_order: 2
parent: Reference
---

# Tasks: every `mise run` task in a project

Every task of a project made by `dev new`: what it does, what it needs, and whether it stays on your machine. Read it to find the task for a job, or to know what a task will touch before you run it. The list is what `mise tasks` printed in a new project on 2026-10-01: 40 tasks.

```sh
mise tasks                            # every task, with its description
mise run check                        # run one
mise run sdk:gen api-go typescript    # words after the task's name are passed to it
```

Every task is one line in `mise.toml`. Most call a command of the tool the tasks run (`dev`): [The dev tool](dev.md) has each command's flags.

## How to read the tables

- **Needs** is what must be there besides `mise install`. "npm packages" means `mise run setup` was run.
- **Local** tasks touch only your machine. **Remote** tasks read or change something on Cloudflare or GitHub. The task's own description starts with `REMOTE` for those.
- **A Cloudflare login** is `api-go/node_modules/.bin/cf auth login`, or the variables `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` ([Configuration](config.md#environment-variables)).

## The whole project

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `setup` | Installs the npm packages of `api-go/` (the Cloudflare CLI, `cf`) and `sdk/` (Fern), from their lockfiles | Network | Local |
| `check` | Runs `api-go:check` and `dev:check`: every local check | npm packages | Local |
| `doctor` | Checks what the tasks need: npm packages, Docker, Go, TinyGo, `wasm-opt`, Rust, `gh`. Lists the APIs. Fails if npm packages are missing or Docker is not running | Nothing | Local |
| `upstream:status` | Lists every `Upstream:` tag in the code with the state of its issue. `CLOSED` means that workaround can go | A git repository, `gh` | Remote: reads issues on GitHub |

## The API

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `api-go:run` | Runs the API natively on `API_GO_PORT` (5174), with an in-memory store. Notes are gone when it stops | Nothing | Local |
| `api-go:build` | Builds the Worker's Wasm into `api-go/build/`, tuned for Workers (`dev wasm-build`: [what it changes](dev.md#wasm-build)). Fails if `api-go/build/app.wasm` is over 3,000,000 bytes gzipped | Nothing | Local |
| `api-go:dev` | Runs `api-go:build`, then the Worker under workerd (`cf dev`) on `API_GO_PORT`, with a local D1 database and the hub. After a Go change, run `api-go:build` in another shell: `cf dev` reloads | npm packages | Local |
| `api-go:migrate:local` | Applies `migrations/` to the local D1 database of a running `api-go:dev`, each file once | `api-go:dev` running | Local |
| `api-go:spec` | Writes `sdk/fern/apis/api-go/openapi.json` and `sdk/fern/apis/api-go/asyncapi.json` from the contract, with `API_GO_URL` as their server | Nothing | Local |
| `api-go:spec:check` | Fails if a committed spec differs from what the contract gives | Nothing | Local |
| `api-go:lint` | `gofmt`, and `go vet` for your machine and for Wasm | Nothing | Local |
| `api-go:test` | `go test ./...` in `api-go/` | Nothing | Local |
| `api-go:test:native` | Starts the native build on a free port and runs the real-time test (`test/live-test.mjs`) and the MCP test (`test/mcp-test.mjs`) against it | npm packages | Local |
| `api-go:test:workerd` | Runs `api-go:build`, starts the Wasm under workerd on a free port, applies the migrations, and runs the same two tests | npm packages | Local |
| `api-go:mcp-test` | Runs the MCP test against a server already running on `API_GO_PORT`. It writes test notes | npm packages, `api-go:run` or `api-go:dev` running | Local |
| `api-go:check` | Runs `api-go:lint`, `api-go:test`, `api-go:spec:check`, `sdk:publish:fresh`, `api-go:test:native` and `api-go:test:workerd` | npm packages | Local |
| `api-go:deploy` | Runs `api-go:build`, deploys the Worker (`cf deploy`, which creates its D1 database), then applies pending migrations as `api-go:migrate` does | npm packages, a Cloudflare login | Remote: changes the Worker and its database |
| `api-go:migrate` | Applies pending files of `migrations/` to the D1 database `<name>-db` | npm packages, a Cloudflare login, the Worker deployed once | Remote: changes the database |
| `api-go:live-test` | Tests the deployed Worker at `API_GO_URL`: SSE and the WebSocket, raw and through the TypeScript SDK, then MCP. It writes test notes. It generates the `typescript-dist` SDK first when that is missing | npm packages; Docker the first time | Remote: writes notes |
| `api-go:soak` | Runs every client against every real-time scenario on the deployed Worker, and redeploys it once in the middle. Add `--idle <minutes>` for the long-idle case. It generates the SDKs and builds the Fern CLI first when they are missing | npm packages, a Cloudflare login; Docker and `cargo` the first time | Remote: redeploys the Worker, writes notes |
| `api-go:bench` | Calls every GET operation of the deployed Worker that the spec has examples for, after 30 s of warm-up, and prints wall time, the CPU time Cloudflare recorded, and the CPU of each request. About a minute and a half | The Worker deployed; `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment or in fnox | Remote: read-only |
| `api-go:perf` | Runs `api-go:deploy`, then benches the new isolate from its first request: 8 requests at once, then every operation with writes, with the CPU of each request. One command to see what a change costs on Cloudflare. About two minutes | As `api-go:deploy` and `api-go:bench` | Remote: changes the Worker and its database, writes test notes |

## SDKs

An `<api>` is a folder under `sdk/fern/apis/`. A new project has one, `api-go`. A `<group>` is one SDK its `generators.yml` defines: `go`, `typescript`, `typescript-dist` or `cli`.

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `sdk:list` | Lists the APIs and the groups each defines | Nothing | Local |
| `sdk:check-spec <api>` | Validates the API's specs and settings with `fern check` | npm packages | Local |
| `sdk:gen <api> <group>` | Generates one SDK with Fern into `sdk/out/<api>/<group>`, replacing what is there | npm packages, Docker | Local |
| `sdk:check <dir>` | Proves a generated SDK works. Go: build, vet, and its tests against a WireMock container. TypeScript: a typecheck | The SDK generated; Docker for a Go SDK | Local |
| `sdk:ready <api> [group...]` | Generates and builds what the tests use, only where it is missing: the `typescript-dist` and `go` SDKs and the Fern CLI, or only the groups named | npm packages; Docker and `cargo` when something is missing | Local |
| `sdk:publish` | Generates the Go SDK of `api-go` fresh, checks it as `sdk:check` does, and copies its sources into `sdk/go`: the committed Go module another repo fetches with `go get`. Adds `sdk/go` to `go.work`. Commit the result | npm packages, Docker | Local |
| `sdk:publish:check` | Generates the Go SDK again and fails if `sdk/go` differs. Passes when there is no `sdk/go` yet | npm packages, Docker | Local |
| `sdk:publish:fresh` | Fails if the specs changed since `sdk/go` was generated from them. It compares a hash, so it is quick and needs no Docker; `api-go:check` runs it. Passes when there is no `sdk/go` yet | Nothing | Local |
| `sdk:cli:build [-linux] <dir>` | Builds a generated Rust CLI, for example `sdk/out/api-go/cli`. Heavy: the first build takes minutes at full CPU. `-linux` builds for Linux in Docker | The CLI generated, `cargo`; Docker with `-linux` | Local |
| `sdk:dist` | Generates the Go and TypeScript SDKs of `api-go` fresh, checks them, and archives them and the specs into `dist/` | npm packages, Docker | Local |
| `sdk:dist:cli [-linux] <api>` | Generates and builds the API's Fern CLI into `dist/`. Heavy | npm packages, Docker, `cargo` | Local |
| `sdk:clean` | Removes `sdk/out/` and stops leftover WireMock containers | Nothing | Local |

## Docs

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `docs:setup` | Writes the docs site's config, style, `docs/writing.md` and `docs/llms.txt`, filled in with the repo's name and description | `gh`, and the repo on GitHub | Remote: reads the repo's name from GitHub |
| `docs:lint` | Checks `docs/` for what a program can check: front matter, links, tasks and paths that don't exist | A git repository | Local |
| `docs:review` | Has Claude bring `docs/` into line with `docs/writing.md`. It edits files | The `claude` command | Local |
| `docs:pages` | Turns on GitHub Pages for `docs/` on `main`. Once per repo | `gh` with admin access to the repo | Remote: changes a GitHub setting |

## GitHub workflows, secrets and releases

What a release ships is built by `sdk:dist` and `sdk:dist:cli`, in [SDKs](#sdks) above.

| Task | What it does | Needs | Local or remote |
|---|---|---|---|
| `dev:workflows` | Writes the GitHub workflows into `.github/workflows/`. Never edit the written files | Nothing | Local |
| `dev:check` | Fails if a workflow or the docs site's config differs from what the tool writes, or if `docs:lint` finds a problem. Before `dev:workflows` and `docs:setup` have been run, those two checks pass | A git repository; `gh` and the repo on GitHub once `docs/_config.yml` exists | Local, then remote: reads the repo's name from GitHub |
| `cloudflare:token` | Fails unless `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` are set. The deploy workflow runs it first | Nothing | Local |
| `cloudflare:secrets` | Copies those two variables from fnox into the repo's GitHub Actions secrets, without printing them. Once per repo | `gh`, the two values stored in fnox | Remote: sets GitHub secrets |
| `release` | On a version tag in a GitHub workflow: attaches the files in `dist/` to the tag's GitHub Release. Anywhere else: a dry run that lists `dist/` | Files in `dist/`; `gh` for a real run | Remote on a tag; otherwise local |
| `release:tags` | On a version tag: adds the tags `api-go/vX.Y.Z` and, when `sdk/go` exists, `sdk/go/vX.Y.Z` on the same commit, which Go needs to find a version of those modules. Anywhere else: a dry run | `gh` for a real run | Remote on a tag; otherwise local |

## Tasks that exist only in the orpc-api repo

The orpc-api repo has every task above, and these as well. They belong to its second server (TypeScript), its two showcase APIs, its SDK test Worker and the release of the tool itself.

| Tasks | What they are for |
|---|---|
| `api:dev`, `api:migrate:local`, `api:spec`, `api:spec:check`, `api:typecheck`, `api:test`, `api:check`, `api:deploy`, `api:migrate`, `api:live-test`, `api:soak`, `api:bench` | The oRPC Worker in TypeScript: the same set as `api-go:*` |
| `showcase-go:run`, `showcase-go:build`, `showcase-go:dev`, `showcase-go:spec`, `showcase-go:spec:check`, `showcase-go:lint`, `showcase-go:test`, `showcase-go:test:native`, `showcase-go:test:workerd`, `showcase-go:check`, `showcase-go:deploy` | The Go showcase: a second API that uses every Fern feature |
| `showcase:spec`, `showcase:spec:check`, `showcase:test`, `showcase:typecheck`, `showcase:check` | The same showcase written with oRPC |
| `sdk:harness:test`, `sdk:harness:deploy` | The Worker that runs Fern's TypeScript SDK inside workerd |
| `sdk:demo` | A small end-to-end run of Fern on a sample spec |
| `sdk:docs` | A local preview of the API docs Fern generates |
| `sdk:cloudflare` | Adds Cloudflare's own API as an API for Fern |
| `dev:release` | Publishes the tool itself with GoReleaser ([Releases](releases.md)) |

In that repo `check` runs more (both servers, both showcases, the SDK test Worker) and needs Docker, `setup` installs four npm folders, and `release:tags` also adds `dev/vX.Y.Z`.

## Limits

- **Some descriptions in a new project's `mise.toml` describe the orpc-api repo, not the project.** `mise tasks` there says `check` needs Docker, `setup` installs `api/` and `sdk/harness/`, and `sdk:dist` archives "both APIs". The tables above say what the tasks do in a project.
- **What was run for this page:** `mise tasks`, `api-go:spec:check`, `sdk:list`, `doctor` and `release:tags` (a dry run) were run in a new project on 2026-10-01. The other rows are read from the project's `mise.toml` and the tool's source.
