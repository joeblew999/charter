---
title: Tasks
nav_order: 1
parent: Reference
---

# Tasks: every `mise run` task

Run in the project's folder; words after the name go to the task, flags after `--`. **REMOTE**: reads or changes something on Cloudflare or GitHub. `mise tasks` lists them.

Most tasks are written once, in charter's `tasks/` folders. A project includes `tasks/shared` and the one of its language (`[task_config] includes` in its `mise.toml`, from GitHub at the release it pins), and its `mise.toml` holds only its own tasks: `check`, `lint`, `deploy` and the tests. A task in `mise.toml` takes the place of the shared one with its name. The tasks call the tool through the setting `CHARTER_TOOL`: `charter` in a project, this checkout's in the examples here.

## A Go project

What `charter new` makes, and `examples/notes-go/`.

| Task | What it does |
|---|---|
| `setup` | Installs the npm packages: `cf`, Fern, what the tests import |
| `check` | Every local check: `lint`, `test`, `spec:check`, `sdk:publish:fresh`, `test:native`, `test:workerd`, `workflows:check`, `docs:check` |
| `doctor` | Says what the tasks need and lack |
| `run` | The API natively on `API_PORT`, in memory |
| `build` | The Wasm and the Worker glue into `build/`; fails over 3,000,000 bytes gzipped |
| `dev` | `build`, then the Worker under workerd (`cf dev`) |
| `migrate:local` | Applies `migrations/` to the local D1 of a running `dev` |
| `spec` | Writes `fern/openapi.json` and `fern/asyncapi.json` from the contract |
| `spec:check` | Fails if a committed spec is stale |
| `lint` | `gofmt`, `go vet` for the host and for Wasm |
| `test` | `go test ./...` |
| `test:native` | The live and MCP tests against the native build, on a free port |
| `test:workerd` | The same against the Wasm under workerd |
| `mcp-test` | The MCP test against a running `run` or `dev` |
| `deploy` | REMOTE. `build`, `cf deploy`, `migrate` |
| `migrate` | REMOTE. Applies pending migrations to `<worker>-db` |
| `live-test` | REMOTE, writes test notes. SSE, WebSocket, the TypeScript SDK, MCP |
| `soak` | REMOTE, redeploys. Every client against every real-time scenario; `--idle <min>` |
| `bench` | REMOTE, read-only. Times every GET operation |
| `perf` | REMOTE. `deploy`, then the bench of the new isolate |
| `perf:try` | REMOTE. `-- -name <experiment>`: a scratch Worker, built, benched, deleted |
| `perf:clean` | REMOTE. Deletes what a `perf:try` left |
| `cloudflare:token` | Fails unless `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` are set |
| `cloudflare:secrets` | REMOTE. Copies those two from fnox into the repo's GitHub secrets |
| `upstream:status` | REMOTE, read-only. Every `Upstream:` tag in the code, with its issue's state |
| `sdk:list` | The groups in `fern/generators.yml` |
| `sdk:check-spec` | Fern's validation of the specs and its settings |
| `sdk:gen <group>` | Generates one SDK into `sdk/out/<group>` (Docker) |
| `sdk:check <group>` | Go: build, vet, tests against WireMock. TypeScript: typecheck |
| `sdk:ready [group...]` | Generates what the tests use, if missing, and builds the CLI |
| `sdk:publish` | Generates and checks the Go SDK, copies it into `sdk/go/` |
| `sdk:publish:check` | Fails if `sdk/go/` is not what the specs generate now |
| `sdk:publish:fresh` | Fails if the specs changed since `sdk/go/` was made (a hash, no Docker) |
| `sdk:cli:build [-linux]` | HEAVY. Builds the generated Rust CLI |
| `sdk:dist` | Generates, checks and archives every SDK and the specs into `dist/` |
| `sdk:dist:cli [-linux]` | HEAVY. The CLI into `dist/` |
| `sdk:docs` | Previews the API's reference site on port 3030 |
| `sdk:clean` | Removes `sdk/out` and stops leftover WireMock containers |
| `release` | REMOTE on a version tag: attaches `dist/*` to its GitHub Release. Elsewhere a dry run |
| `release:tags` | REMOTE on a version tag: tags `sdk/go/vX.Y.Z`. Elsewhere a dry run |
| `workflows`, `workflows:check` | Writes `.github/` from the tool's templates; fails if it differs |
| `docs:setup` | Writes the docs site's config, `docs/writing.md`, `docs/llms.txt` |
| `docs:lint`, `docs:check` | Checks `docs/`; `docs:check` also fails if the config is stale |
| `docs:review` | Has Claude bring `docs/` into line with `docs/writing.md` |
| `docs:pages` | REMOTE. Turns GitHub Pages on for `docs/` |

## The other projects

| Project | Differences |
|---|---|
| `examples/notes-ts/` | No `run`, `build`, `test:native`, `test:workerd`, `perf`. `lint` typechecks, `test` runs the feed's tests |
| `conformance/showcase-go/` | No `migrate`, `live-test`, `soak`, `bench` |
| `showcase-test` | In `conformance/showcase-go/`: the SDK test against a running `run` or `dev` |
| `conformance/showcase-ts/` | `test:workerd` runs the TypeScript SDK inside the Worker; `deploy` deploys it twice |

## This repo

| Task | What it does |
|---|---|
| `check` | `charter:check`, `go:check`, `ts:check`, `projects:check`. Needs Docker |
| `projects:check` | `ts:build`, then `check` in every project under `examples/` and `conformance/` |
| `charter:check` | The tool: gofmt, vet, tests, the generated workflows and docs config, the docs lint |
| `go:check` | `go:lint` and `go:test`: the library, for the host and for Wasm |
| `go:lint`, `go:test` | The library's lint (gofmt, vet, vet for Wasm) and its tests, on their own |
| `ts:setup` | Installs the TypeScript library's packages and builds it |
| `ts:build` | Compiles the TypeScript library into `ts/dist/`, which its package exports. After changing `ts/src/` |
| `ts:check` | The TypeScript library: typecheck (`ts:lint`) and tests (`ts:test`) |
| `ts:lint`, `ts:test` | The TypeScript library's typecheck and its tests, on their own |
| `compare` | REMOTE. The same bench against the TypeScript and the Go notes Workers |
| `charter:release` | GoReleaser on the tool. REMOTE on a version tag; elsewhere a snapshot |
| `ts:dist` | Packs the TypeScript library into `dist/charter-ts-X.Y.Z.tgz`, versioned as the tag (none: `0.0.0-dev`) |
| `ts:release` | REMOTE on a version tag: attaches `dist/*` to its GitHub Release. Elsewhere a dry run |
| `release:tags` | REMOTE on a version tag: tags `go/vX.Y.Z` and `examples/notes-go/sdk/go/vX.Y.Z` |
| `setup`, `doctor`, `sdk:clean`, `workflows`, `upstream:status`, `docs:setup`, `docs:lint`, `docs:review`, `docs:pages`, `cloudflare:secrets` | As in a project, for the whole repo |

## Windows and macOS

| | On Windows (`windows-2025`) and macOS (`macos-15`), on every push |
|---|---|
| Checked | `charter:check`, `go:check`, `ts:check`; in `examples/notes-go/` and `examples/notes-ts/`, `setup` and `check`. A project made by `charter new` gets the same two jobs |
| Not checked | Fern's tasks and those that need a generated SDK: Fern generates in Linux containers, which GitHub's Windows and macOS runners do not run. The REMOTE tasks |
| Different on Windows | A stopped server is ended by force with what it started (`taskkill`); git must check out LF line ends (`.gitattributes` says so) |
