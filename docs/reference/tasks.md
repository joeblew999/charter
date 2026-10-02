---
title: Tasks
nav_order: 1
parent: Reference
---

# Tasks: every `mise run` task

Every task of a project, and of this repo. Task names are the same in every project, with no prefix. Each is one line in `mise.toml`; what needs more is a command of the tool ([The charter command](charter.md)). They run on macOS, Linux and Windows ([what is covered there](#windows)).

```sh
mise tasks                     # every task, with its description
mise run sdk:gen typescript    # in the project's folder; words after the name go to the task
mise run bench -- -write       # flags go after --
```

**REMOTE** tasks read or change something on Cloudflare or GitHub: they need a Cloudflare login (`npx cf auth login`, or `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`) or `gh`. The others touch only your machine. Tasks that run `cf`, Fern or a test program need `mise run setup` first.

## A Go project

What `charter new` makes, and `examples/notes-go/`.

| Task | What it does |
|---|---|
| `setup` | Installs the npm packages: `cf`, Fern, what the tests import |
| `check` | Every local check: `lint`, `test`, `spec:check`, `sdk:publish:fresh`, `test:native`, `test:workerd`, `workflows:check`, `docs:check` |
| `doctor` | Says what the tasks need and lack: npm packages, Docker, Go, Rust, `gh` |
| `run` | The API natively on `API_PORT`, in memory |
| `build` | The Worker's Wasm and the glue into `build/`. Fails over 3,000,000 bytes gzipped |
| `dev` | `build`, then the Worker under workerd (`cf dev`). After a Go change: `build` again |
| `migrate:local` | Applies `migrations/` to the local D1 of a running `dev` |
| `spec` | Writes `fern/openapi.json` and `fern/asyncapi.json` from the contract, with `API_URL` as their server |
| `spec:check` | Fails if a committed spec is stale |
| `lint` | `gofmt`, `go vet` for the host and for Wasm |
| `test` | `go test ./...` |
| `test:native` | Starts the native build on a free port; runs the live test and the MCP test against it |
| `test:workerd` | The same against the Wasm under workerd, with a local D1 |
| `mcp-test` | The MCP test against a running `run` or `dev`. Writes test notes |
| `deploy` | REMOTE. `build`, `cf deploy`, `migrate` |
| `migrate` | REMOTE. Applies pending `migrations/` to the database `<worker>-db` |
| `live-test` | REMOTE, writes test notes. SSE and WebSocket, raw and through the TypeScript SDK, then MCP. Docker the first time |
| `soak` | REMOTE, redeploys the Worker. Every client against every real-time scenario. `--idle <min>` for the long-idle case |
| `bench` | REMOTE, read-only. Every GET operation: wall time, CPU, the CPU of each request |
| `perf` | REMOTE, writes test notes. `deploy`, then the bench of the new isolate from its first request |
| `perf:try` | REMOTE. `-- -name <experiment>`: a scratch Worker of its own, built, deployed, benched, deleted |
| `perf:clean` | REMOTE. Deletes what a `perf:try` left |
| `cloudflare:token` | Fails unless the two `CLOUDFLARE_` variables are set |
| `cloudflare:secrets` | REMOTE, once per repo. Copies the two from fnox into the repo's GitHub secrets |
| `upstream:status` | REMOTE, read-only. Every `Upstream:` tag in the code, with its issue's state |

### SDKs

Fern generates in Docker: `sdk:gen` and every task that generates need it running.

| Task | What it does |
|---|---|
| `sdk:list` | The groups `fern/generators.yml` defines |
| `sdk:check-spec` | Fern's validation of the specs and its settings |
| `sdk:gen <group>` | Generates one SDK into `sdk/out/<group>` |
| `sdk:check <group>` | Proves it works. Go: build, vet, tests against WireMock. TypeScript: typecheck |
| `sdk:ready [group...]` | Generates what the tests use, if missing, and builds the CLI |
| `sdk:publish` | Generates and checks the Go SDK, copies its sources into `sdk/go/` |
| `sdk:publish:check` | Fails if `sdk/go/` is not what the specs generate now |
| `sdk:publish:fresh` | Fails if the specs changed since `sdk/go/` was made. A hash, no Docker |
| `sdk:cli:build [-linux]` | HEAVY. Builds the generated Rust CLI in `sdk/out/cli` |
| `sdk:dist` | Generates, checks and archives every SDK and the specs into `dist/` |
| `sdk:dist:cli [-linux]` | HEAVY. Generates and builds the CLI into `dist/` |
| `sdk:docs` | Previews the API's reference site on port 3030 |
| `sdk:clean` | Removes `sdk/out` and stops leftover WireMock containers |

### The repo around it

| Task | What it does |
|---|---|
| `release` | REMOTE on a version tag: attaches `dist/*` to its GitHub Release. Elsewhere: a dry run |
| `release:tags` | REMOTE on a version tag: tags `sdk/go/vX.Y.Z`. Elsewhere: a dry run |
| `workflows`, `workflows:check` | Writes `.github/workflows/` from the tool's templates; fails if one differs |
| `docs:setup` | Writes the docs site's config, `docs/writing.md` and `docs/llms.txt` |
| `docs:lint`, `docs:check` | Checks `docs/`; also fails if the config is stale |
| `docs:review` | Has Claude bring `docs/` into line with `docs/writing.md` |
| `docs:pages` | REMOTE, once per repo. Turns GitHub Pages on for `docs/` |

## Windows

The tasks run natively on Windows: mise runs a task line with cmd.exe there, and every line has only what cmd.exe and sh both take.

| | On Windows |
|---|---|
| Checked on every push, on GitHub's `windows-2025` runner | At the root: `charter:check`, `go:check`. In `examples/notes-go/`: `setup`, then `check`: `lint`, `test`, `spec:check`, `sdk:publish:fresh`, `build` (TinyGo with the patched runtime), `test:native`, `test:workerd` (`cf dev`, workerd, the local D1), `workflows:check`, `docs:check`. A project made by `charter new` gets the same job, `check-windows` |
| Not checked there | The tasks that generate with Fern (`sdk:gen`, `sdk:check`, `sdk:ready`, `sdk:publish`, `sdk:dist`, the `sdk:cli` tasks) and those that need a generated SDK (`live-test`, `soak`, the `check` of `examples/showcase-go/` and `examples/showcase-ts/`): Fern generates in Linux containers, which GitHub's Windows runners do not run. With Docker Desktop they may work; nothing here has run them. The `check` of `examples/notes-ts/` and the REMOTE tasks have not been run on Windows either |
| Different | A stopped server is ended by force, with everything it started (`taskkill`), since Windows has no signal to ask with. The patched TinyGo root is hard links or copies where symbolic links are not allowed |

Git must check files out with LF line ends, which `.gitattributes` says: the checks compare generated files with the committed ones byte for byte.

## The other examples

The same names, with these main differences. `mise tasks` in each folder is the exact list.

| Project | Differences |
|---|---|
| `examples/notes-ts/` | No `run`, `build`, `test:native`, `test:workerd` or `perf`. `lint` typechecks, `test` runs the feed's unit tests, `check` is those and `spec:check`. `live-test` and `soak` run the programs in `examples/notes-go/test/` |
| `examples/showcase-go/` | No storage, so no `migrate`; no `live-test`, `soak` or `bench`. `test:native` and `test:workerd` run the SDK test with a webhook receiver; `showcase-test` runs it against a running server |
| `examples/showcase-ts/` | `test:workerd` runs Fern's TypeScript SDK inside the Worker under `cf dev`, `live-test` on the deployed one. `deploy` deploys it twice |

## This repo

Run at the root.

| Task | What it does |
|---|---|
| `setup`, `doctor`, `sdk:clean` | The same task in every example |
| `check` | `charter:check`, `go:check`, `examples:check`. Needs Docker |
| `examples:check` | `check` in every example, one after the other |
| `charter:check` | The tool: gofmt, vet, tests, the test that holds the two notes examples to one surface, the generated workflows and docs config, the docs lint |
| `go:check` | `go:lint` and `go:test`: the library, for the host and for Wasm |
| `compare` | REMOTE, writes test notes. The same bench against the TypeScript notes Worker and the Go one. About two minutes |
| `upstream:status` | REMOTE, read-only. Every `Upstream:` tag under the repo |
| `workflows` | Writes this repo's workflows |
| `charter:release` | GoReleaser on the tool. REMOTE on a version tag; elsewhere a snapshot into `dist/` |
| `release:tags` | REMOTE on a version tag: tags `go/vX.Y.Z` and `examples/notes-go/sdk/go/vX.Y.Z` |
| `docs:setup`, `docs:lint`, `docs:review`, `docs:pages`, `cloudflare:secrets` | As in a project |
