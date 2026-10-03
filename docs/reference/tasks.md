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
| `doctor` | Says what the tasks need and lack; Rust only with a CLI, and whether the CLI's pins agree with the `cli` group |
| `run` | The API natively on `API_PORT`, in memory |
| `build` | The Wasm and the Worker glue into `build/`; fails over 3,000,000 bytes gzipped |
| `dev` | `build`, then the Worker under workerd (`cf dev`) |
| `migrate:local` | Applies `migrations/` to the local D1 of a running `dev` |
| `spec` | Writes `fern/openapi.json` and `fern/asyncapi.json` from the contract |
| `spec:check` | Fails if a committed spec is stale |
| `spec:diff` | The specs against the previous release's: fails on a breaking change unless `-- -tag vX.Y.Z` is a major release ([Repos that use each other](../guides/repos.md#catch-a-breaking-change)) |
| `lint` | `gofmt`, `go vet` for the host and for Wasm |
| `test` | `go test ./...` |
| `test:native` | The live, MCP and auth tests against the native build, on a free port; the auth test serves a test issuer as Access and as an OpenID Connect issuer |
| `test:workerd` | The same against the Wasm under workerd |
| `mcp-test` | The MCP test against a running `run` or `dev` |
| `deploy` | REMOTE. `build`, `cf deploy`, `migrate` |
| `migrate` | REMOTE. Applies pending migrations to `<worker>-db` |
| `tail` | REMOTE, read-only. The deployed Worker's live logs: each request, its console output and exceptions, until Ctrl-C; `-- -for 30s`, `-- -worker <name>` |
| `live-test` | REMOTE, writes test notes. SSE, WebSocket, the TypeScript SDK, MCP |
| `soak` | REMOTE, redeploys. Every client against every real-time scenario; `--idle <min>` |
| `bench` | REMOTE, read-only. Times every GET operation |
| `perf` | REMOTE. `deploy`, then the bench of the new isolate |
| `perf:try` | REMOTE. `-- -name <experiment>`: a scratch Worker, built, benched, deleted |
| `perf:clean` | REMOTE. Deletes what a `perf:try` left |
| `cloudflare:token` | Fails unless `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` are set |
| `cloudflare:secrets` | REMOTE. Copies those two from fnox into the repo's GitHub secrets |
| `access:setup -- <email>...` | REMOTE. Cloudflare Access in front of the Worker: these people log in with GitHub; the Worker's `ACCESS_TEAM_DOMAIN` and `ACCESS_AUD`, also in fnox. Again to change who ([Auth](../guides/auth.md#cloudflare-access)) |
| `access:token -- create <machine> <file\|fnox>`, `list`, `revoke <machine>` | REMOTE. A machine's own Access service token: made, listed, revoked |
| `access:delete` | REMOTE. Deletes the Access application and its machines' tokens |
| `upstream:status` | REMOTE, read-only. Every `Upstream:` tag in the code, with its issue's state |
| `sdk:list` | The groups in `fern/generators.yml` |
| `sdk:check-spec` | Fern's validation of the specs and its settings |
| `sdk:gen <group>` | Generates one SDK into `sdk/out/<group>` (Docker) |
| `sdk:check <group>` | Go: build, vet, tests against WireMock. TypeScript: typecheck |
| `sdk:ready [group...]` | Generates what the tests use, if missing or made from specs or settings that have changed since, and builds the CLI if the project has one |
| `sdk:publish` | Generates and checks the Go SDK, copies it into `sdk/go/` |
| `sdk:publish:check` | Fails if `sdk/go/` is not what the specs generate now |
| `sdk:publish:fresh` | Fails if the specs changed since `sdk/go/` was made (a hash, no Docker) |
| `sdk:cli:build [-linux]` | HEAVY. Builds the generated Rust CLI. This and the two below need the `cli` group in `fern/generators.yml` ([Add the CLI](../guides/sdks.md#add-the-cli)); without it they say the project has no CLI |
| `sdk:dist` | Generates, checks and archives every SDK and the specs into `dist/` |
| `sdk:dist:cli [-target ...]` | HEAVY. The CLI into `dist/` for darwin, linux, windows × amd64, arm64 (all six on a Mac, all but darwin elsewhere) |
| `sdk:cli:smoke` | Runs the CLI in `dist/` built for this machine |
| `dist` | HEAVY. Empties `dist/`, then `sdk:dist` and, with a CLI, `sdk:dist:cli`: what a release ships |
| `sdk:docs` | Previews the API's reference site on port 3030 |
| `sdk:clean` | Removes `sdk/out` and stops leftover WireMock containers |
| `release -- vX.Y.Z [-dry-run]` | REMOTE. Cuts a release from this machine: `check`, `dist`, the tag, the GitHub Release, `release:publish`, `release:tags` ([how](../guides/release.md)) |
| `release:publish` | REMOTE on a version tag: makes its GitHub Release if missing, attaches what of `dist/*` it lacks, writes `SHA256SUMS`. Elsewhere a dry run |
| `release:tags` | REMOTE on a version tag: tags `sdk/go/vX.Y.Z`. Elsewhere a dry run |
| `repo`, `repo:check` | REMOTE. `charter repo` at the repo's root: keeps it as `charter.toml` says ([Keep the repo in shape](../guides/deploy.md#keep-the-repo-in-shape)); `repo:check` changes nothing and fails on drift |
| `workflows`, `workflows:check` | Writes `.github/` from the tool's templates; fails if it differs |
| `docs:setup` | Writes the docs site's config, `docs/writing.md`, `docs/llms.txt` and the [generated pages](../guides/deploy.md#generated-pages) |
| `docs:lint`, `docs:check` | Checks `docs/`; `docs:check` also fails if the config or a generated page is stale |
| `docs:review` | Has Claude bring `docs/` into line with `docs/writing.md` |
| `docs:pages` | REMOTE. Turns GitHub Pages on for `docs/` |

## The other projects

| Project | Differences |
|---|---|
| `examples/notes-ts/` | No `run`, `build`, `test:native`, `perf`. `lint` typechecks, `test` runs the feed's tests, `test:workerd` runs the live and auth tests under workerd |
| `examples/start-go/` | No `soak`, no CLI (the CLI tasks say so). `live-test` checks `/api/hello`, the spec and MCP |
| `examples/start-htmx/` | As `examples/start-go/`, with the pages: `check` adds `ui:check`; `live-test` writes test messages and checks the pages and their stream too |
| `examples/start-datastar/` | As `examples/start-htmx/` |
| `ui:gen`, `ui:check` | In `examples/start-htmx/` and `examples/start-datastar/`: write `pages/*.x.go` from `pages/*.gsx` (gsx, pinned in `go.mod`); fail if the committed ones are stale, writing them as they should be |
| `examples/start-ts/` | As `examples/notes-ts/`, but `test:workerd` runs the live test under workerd; no `soak` |
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
| `catalog` | REMOTE, read-only. `charter catalog`: the owner's charter repos and who pins each ([Repos that use each other](../guides/repos.md)) |
| `release -- vX.Y.Z [-dry-run]` | REMOTE. Cuts a release of the repo, as in a project; [how](../contributing.md#cut-a-release) |
| `dist` | HEAVY. `dist` in `examples/notes-go/` and `examples/notes-ts/` |
| `release:publish` | REMOTE on a version tag: `ts:dist`, then the package, the tool (GoReleaser) and the notes examples' `dist/` onto the Release. Elsewhere a dry run |
| `charter:release` | GoReleaser on the tool. REMOTE on a version tag (a build only if the Release has the tool already); elsewhere a snapshot |
| `ts:dist` | Packs the TypeScript library into `dist/charter-ts-X.Y.Z.tgz`, versioned as the tag (none: `0.0.0-dev`) |
| `ts:release` | REMOTE on a version tag: attaches what of `dist/*` its GitHub Release lacks. Elsewhere a dry run |
| `release:tags` | REMOTE on a version tag: tags `go/vX.Y.Z` and `examples/notes-go/sdk/go/vX.Y.Z` |
| `setup`, `doctor`, `sdk:clean`, `repo`, `repo:check`, `workflows`, `upstream:status`, `docs:setup`, `docs:lint`, `docs:review`, `docs:pages`, `cloudflare:secrets` | As in a project, for the whole repo |

## Windows and macOS

| | On Windows (`windows-2025`) and macOS (`macos-15`), on every push and version tag |
|---|---|
| Checked | `charter:check`, `go:check`, `ts:check`; in `examples/notes-go/`, `examples/notes-ts/`, `examples/start-datastar/`, `examples/start-go/`, `examples/start-htmx/` and `examples/start-ts/`, `setup` and `check`. A project made by `charter new` gets the same two jobs |
| Not checked | Fern's tasks and those that need a generated SDK: Fern generates in Linux containers, which GitHub's Windows and macOS runners do not run. The REMOTE tasks |
| The CLI | In a project with one: built for every OS on Linux by the `release` workflow (darwin only by a release cut on a Mac); its `cli-windows` job starts the Windows amd64 one on Windows (`sdk:cli:smoke`) |
| Different on Windows | A stopped server is ended by force with what it started (`taskkill`); git must check out LF line ends (`.gitattributes` says so) |
