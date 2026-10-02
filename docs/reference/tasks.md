---
title: Tasks
nav_order: 1
parent: Reference
---

# Tasks: every `mise run` task

Every task of a project, and of this repo. Task names are the same in every project, with no prefix. Each is one line in the project's `mise.toml`; what needs more is a command of the tool ([The charter command](charter.md)).

```sh
mise tasks                     # every task, with its description
mise run check                 # run one, in the project's folder
mise run sdk:gen typescript    # words after the name are passed to the task
mise run bench -- -write       # flags go after --
```

- **Needs:** besides `mise install`. "npm" means `mise run setup` was run. "Cloudflare" is a login (`./node_modules/.bin/cf auth login`) or `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`.
- **REMOTE** tasks read or change something on Cloudflare or GitHub. The others touch only your machine.

## A Go project

What `charter new` makes, and `examples/notes-go/`.

### The project

| Task | What it does | Needs |
|---|---|---|
| `setup` | Installs the npm packages: `cf`, Fern, what the tests import | Network |
| `check` | Every local check: `lint`, `test`, `spec:check`, `sdk:publish:fresh`, `test:native`, `test:workerd`, `workflows:check`, `docs:check` | npm |
| `doctor` | Says what the tasks need and lack: npm packages, Docker, Go, Rust, `gh`, `FERN_TOKEN`, leftover containers | |
| `upstream:status` | REMOTE, read-only. Every `Upstream:` tag in the code with its issue's state | `gh` |

### The API

| Task | What it does | Needs |
|---|---|---|
| `run` | The API natively on `API_PORT`, in memory | |
| `build` | The Worker's Wasm and the glue into `build/`. Fails over 3,000,000 bytes gzipped | |
| `dev` | `build`, then the Worker under workerd (`cf dev`) on `API_PORT`. After a Go change: `build` again | npm |
| `migrate:local` | Applies `migrations/` to the local D1 of a running `dev`, each file once | `dev` running |
| `spec` | Writes `fern/openapi.json` and `fern/asyncapi.json` from the contract, with `API_URL` as their server | |
| `spec:check` | Fails if a committed spec is stale | |
| `lint` | `gofmt`, `go vet` for the host and for Wasm | |
| `test` | `go test ./...` | |
| `test:native` | Starts the native build on a free port; runs `test/live-test.mjs` and `test/mcp-test.mjs` against it | npm |
| `test:workerd` | `build`, starts the Wasm under workerd on a free port, migrates, runs the same two | npm |
| `mcp-test` | The MCP test against a server already running on `API_PORT`. Writes test notes | npm, `run` or `dev` running |

### Cloudflare

| Task | What it does | Needs |
|---|---|---|
| `deploy` | REMOTE. `build`, `cf deploy`, then `migrate` | npm, Cloudflare |
| `migrate` | REMOTE. Applies pending `migrations/` to the database `<worker>-db` | npm, Cloudflare, deployed once |
| `live-test` | REMOTE, writes test notes. SSE and WebSocket, raw and through the TypeScript SDK, then MCP, against `API_URL` | npm; Docker the first time |
| `soak` | REMOTE, redeploys the Worker. Every client against every real-time scenario. `--idle <min>` for the long-idle case | npm, Cloudflare; Docker and `cargo` the first time |
| `bench` | REMOTE, read-only. Every GET operation: wall time, CPU, the CPU of each request. `-write` adds the others | The two `CLOUDFLARE_` variables |
| `perf` | REMOTE, writes test notes. `deploy`, then the bench of the new isolate from its first request | As `deploy` and `bench` |
| `perf:try` | REMOTE. `-name <experiment> [-build '<flags>'] [-prebuilt] [-keep]`: a scratch Worker of its own, built, deployed, benched, deleted | As `perf` |
| `perf:clean` | REMOTE. Deletes the scratch Workers and databases a `perf:try` left | Cloudflare |
| `cloudflare:token` | Fails unless the two `CLOUDFLARE_` variables are set | |
| `cloudflare:secrets` | REMOTE, once per repo. Copies the two from fnox into the repo's GitHub secrets | fnox, `gh` |

### SDKs

| Task | What it does | Needs |
|---|---|---|
| `sdk:list` | The groups `fern/generators.yml` defines | |
| `sdk:check-spec` | Fern's validation of the specs and its settings | npm |
| `sdk:gen <group>` | Generates one SDK into `sdk/out/<group>` | npm, Docker |
| `sdk:check <group>` | Proves it works. Go: build, vet, Fern's tests against WireMock. TypeScript: typecheck | Generated; Docker for Go |
| `sdk:ready [group...]` | Generates what the tests use, if missing (`typescript-dist`, `go`, `cli`), and builds the CLI | npm, Docker, `cargo` |
| `sdk:publish` | Generates and checks the Go SDK, copies its sources into `sdk/go/` | npm, Docker |
| `sdk:publish:check` | Fails if `sdk/go/` is not what the specs generate now | npm, Docker |
| `sdk:publish:fresh` | Fails if the specs changed since `sdk/go/` was made. A hash, no Docker | |
| `sdk:cli:build [-linux]` | HEAVY. Builds the generated Rust CLI in `sdk/out/cli` | Generated, `cargo`; Docker with `-linux` |
| `sdk:dist` | Generates, checks and archives every SDK and the specs into `dist/` | npm, Docker |
| `sdk:dist:cli [-linux]` | HEAVY. Generates and builds the CLI into `dist/` | npm, Docker, `cargo` |
| `sdk:docs` | Previews the API's reference site on port 3030 | npm |
| `sdk:clean` | Removes `sdk/out` and stops leftover WireMock containers | |

### The repo around it

| Task | What it does | Needs |
|---|---|---|
| `release` | REMOTE on a version tag: attaches `dist/*` to its GitHub Release. Elsewhere: a dry run | `gh` |
| `release:tags` | REMOTE on a version tag: tags `sdk/go/vX.Y.Z`. Elsewhere: a dry run | `gh` |
| `workflows` | Writes `.github/workflows/` from the tool's templates | |
| `workflows:check` | Fails if a workflow differs from its template | |
| `docs:setup` | Writes the docs site's config, `docs/writing.md` and `docs/llms.txt` | `gh`, a GitHub repo |
| `docs:lint` | Checks `docs/`: front matter, links, the index, tasks and paths that do not exist | |
| `docs:check` | Fails if the docs config is stale or the lint fails | `gh` |
| `docs:review` | Has Claude bring `docs/` into line with `docs/writing.md` | The `claude` command |
| `docs:pages` | REMOTE, once per repo. Turns GitHub Pages on for `docs/` on main | `gh` |

## The other examples

They have the tasks above, with these main differences. `mise tasks` in each folder is the exact list.

| Project | Lacks | Differs or adds |
|---|---|---|
| `examples/notes-ts/` | `run`, `build`, the two `test:` tasks, `mcp-test`, the `perf` tasks, `sdk:publish`, the repo tasks but `release` | `lint` typechecks; `test` runs the feed's unit tests; `check` is `lint`, `test`, `spec:check`; `live-test` and `soak` run the programs in `examples/notes-go/test/` |
| `examples/showcase-go/` | Storage, so no `migrate`; `live-test`, `soak`, `bench`, the `perf` tasks, `sdk:publish`, `sdk:dist`, the repo tasks | `test:native` and `test:workerd` run the SDK test (`test/showcase-test.mjs`) with a webhook receiver; `showcase-test` runs it against a running server |
| `examples/showcase-ts/` | `run`, `build`, `migrate`, `soak`, `bench`, the `perf` tasks, `sdk:publish`, `sdk:dist`, the repo tasks | `test:workerd` runs Fern's TypeScript SDK inside the Worker under `cf dev`; `live-test` the same on the deployed Worker; `deploy` deploys it twice |

## This repo

Run at the root. `mise run check` here runs the tool's and the library's checks, then `check` in every example.

| Task | What it does | Needs |
|---|---|---|
| `setup` | `setup` in every example | Network |
| `check` | `charter:check`, `go:check`, `examples:check` | npm, Docker |
| `examples:check` | `check` in every example, one after the other | npm, Docker |
| `charter:check` | The tool: gofmt, vet, tests (with the test that holds the two notes examples to one surface), the generated workflows and docs config match their templates, the docs lint | `gh` |
| `go:check` | `go:lint` and `go:test`: the library's lint and tests, for the host and for Wasm | |
| `compare` | REMOTE, writes test notes. The same bench against the TypeScript notes Worker and the Go one. About three minutes | The two `CLOUDFLARE_` variables |
| `doctor` | `doctor` in every example | |
| `sdk:clean` | `sdk:clean` in every example | |
| `upstream:status` | REMOTE, read-only. Every `Upstream:` tag under the repo, with its issue's state | `gh` |
| `workflows` | Writes this repo's workflows | |
| `charter:release` | GoReleaser on the tool. REMOTE on a version tag; elsewhere a snapshot into `dist/` | |
| `release:tags` | REMOTE on a version tag: tags the modules others import, `go/vX.Y.Z` and `examples/notes-go/sdk/go/vX.Y.Z` | `gh` |
| `docs:setup`, `docs:lint`, `docs:review`, `docs:pages` | As in a project | |
| `cloudflare:secrets` | As in a project | |
