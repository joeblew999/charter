---
title: The charter command
nav_order: 2
parent: Reference
---

# The charter command: every command and flag

The tool the tasks run (`charter`). Every task in a project's `mise.toml` is one line, and what needs more is a command here. Read it for a command's flags, or to use the tool outside a task. It is Go, standard library only (`cmd/charter/`).

## How to run it

| Where | How |
|---|---|
| Anywhere, with Go | `go run github.com/joeblew999/charter/cmd/charter@latest <command>` |
| In a project made by `charter new` | `charter <command>`: `mise.toml` pins it as `"go:github.com/joeblew999/charter/cmd/charter"`, and `mise install` puts it on the path of every task |
| In this repo | `go run ./cmd/charter <command>`; from an example, `go run ../../cmd/charter <command>` |

```sh
go run ./cmd/charter help       # every command, one line each
go run ./cmd/charter version    # the release it is, or that it was built from a checkout
```

**A command works on the project it is run in:** the nearest folder, from where it is started upwards, that has a `mise.toml` beside a `fern/` folder. Commands marked "anywhere" also run outside one. Flags come before arguments.

## Make a project

| Command | Flags | What it does |
|---|---|---|
| `new` (anywhere) | `-name <name>` (required: lower case, digits, hyphens), `-module <go module>` (default `github.com/<gh login>/<name>`), `-subdomain <workers.dev subdomain>` (default: a placeholder), `-into <dir>` (default `./<name>`, must be empty), `-from <checkout>` | Creates a Go project: a copy of `examples/notes-go/` under your name |
| `version` (anywhere) | | The release the tool is: what `new` pins a project to |

What `new` does:

- **Copies the example's files** as git knows them, without `sdk/go/`. From the tool's own release (its tag, cloned), or from `-from`, or from the checkout it is run in.
- **Renames** the Worker, its URL, the Go module, Fern's organisation, and how the tasks run the tool.
- **Adds** `README.md`, `AGENTS.md`, `CLAUDE.md`, `docs/README.md`, `docs/rules.md`, `docs/writing.md` and the four GitHub workflows.
- **Pins** the tool in `mise.toml` and the library in `go.mod` to the tool's release. From a checkout: the tasks run the tool from there (a `go.work` lets them) and `go.mod` has a `replace` line.
- **Runs** `go mod tidy` and `gofmt`.

## Build

| Command | Flags | What it does |
|---|---|---|
| `wasm-build` | `-heap 8`, `-stack 128kb`, `-opt z`, `-max 3000000`, `-tinygo pinned`, `-plain` | Builds the project's Go Worker into `build/` |
| `size` (anywhere) | `-max <bytes> <file>` | Fails if the file, gzipped, is larger |

### wasm-build

| Flag | Meaning | Default |
|---|---|---|
| `-heap <MB>` | The starting heap. 0: TinyGo's own, a few pages | 8 |
| `-stack <size>` | The stack per goroutine. Huma overflows TinyGo's default | `128kb` |
| `-opt <level>` | TinyGo's optimisation level: `z` and `s` for size, `1` and `2` for speed | `z` |
| `-max <bytes>` | Fail if the Wasm, gzipped, is larger. 3,000,000 is the Workers Free limit | 3000000 |
| `-tinygo pinned\|system` | `pinned`: the TinyGo the tool was tested with, which it asks mise for. `system`: the `tinygo` on the path, untested | `pinned` |
| `-plain` | TinyGo as it is: no runtime patch, no starting heap. To compare | off |

What it does, in order:

1. **Writes workers-go's JavaScript** (`wasm_exec.js`, `runtime.mjs`) into `build/`.
2. **Writes the library's Worker glue** into `build/`: `worker/*.mjs` of the library module as the project's `go.mod` resolves it. So the JavaScript is the one written for the Go the project builds against.
3. **Makes a patched copy of TinyGo's runtime source,** once per TinyGo version, in your cache folder. TinyGo itself is not rebuilt.
4. **Builds** `build/app.wasm` with TinyGo, and checks its size.

| Patch | File in TinyGo | Why | Upstream |
|---|---|---|---|
| No collection at every pause | `src/runtime/gc_finalizer.go`: `finalizerGCThreshold` 32 becomes 0 | TinyGo collected whenever the scheduler went idle after 32 finalizers, and workers-go registers one per JavaScript value. The collector still runs when the heap is full | tinygo-org/tinygo#5800, open |
| Goroutine stacks are reused | `src/internal/task/task_asyncify.go` | TinyGo allocated a stack for every goroutine and every call from JavaScript, and rarely freed one | tinygo-org/tinygo#5801, closed: fixed on TinyGo's dev branch. The patch leaves itself out with a TinyGo that has the fix |

The tool pins TinyGo and binaryen (`wasm-opt`) itself, beside the patches made for them: [Configuration and pins](config.md#pinned-versions). What each step was worth: [Benchmarks](../benchmarks.md).

## Run and test

| Command | Flags | What it does |
|---|---|---|
| `with-server` (anywhere) | `-url <url>`, `-start <cmd>`, `-run <cmd>` (repeat), `-show` | Starts a server, waits up to 90 s for the URL, runs the commands, stops the server. `{port}` in any of them is a free port, `{port2}` another. Output is shown only on failure, or always with `-show` |
| `migrate-local` | `-port <port>` (default `API_PORT`), `-worker <name>` | Applies `migrations/*.sql` to a running dev server's local D1, each once. It remembers them in a table `_local_migrations` |
| `migrate` | `-worker <name>` | REMOTE. Applies pending migrations to the database `<worker>-db` |
| `doctor` | | Says what the project's tasks need and lack |
| `harness-sync` | | Copies the compiled TypeScript SDK into the project's Worker (`src/client`), generating it first if the specs are newer |
| `harness-test` | `-remote` | Runs the SDK test inside the project's Worker: under `cf dev`, or the deployed one (`API_URL`, `API_MOCK_URL`) |
| `harness-deploy` | | REMOTE. Deploys the project's Worker twice: as itself and, with `--mode api`, as `<worker>-api` |

The three `harness-` commands are for a project whose Worker imports its own generated SDK, as `examples/showcase-ts/` does.

## Measure

| Command | Flags | What it does |
|---|---|---|
| `bench` (anywhere) | below, then `[<url>]` (default `API_URL`) | Times every operation of an API from its OpenAPI spec |
| `perf` | `-name <experiment>`, `-build '<wasm-build flags>'`, `-prebuilt`, `-keep`, then `-- <bench flags>` | REMOTE. Builds, deploys to a scratch Worker `<worker>-perf-<experiment>`, benches it from its first request, deletes it |
| `perf-clean` | `-all` | REMOTE. Deletes scratch Workers and their databases. Without `-all` it leaves those made in the last 20 minutes |

### bench

It calls GET operations whose required inputs have examples in the spec, plus one path that does not exist. It skips streams, and bodies that are not JSON.

| Flag | Meaning | Default |
|---|---|---|
| `-n <count>` | Requests per operation, after 3 warm-up ones | 20 |
| `-write` | Also POST, PUT, PATCH and DELETE | off |
| `-cpu` | Also the CPU time Cloudflare measured: median and p99 | off |
| `-each` | Also every request's CPU in the order sent, with the kind of Go runtime it got. Implies `-cpu` | off |
| `-burst <k>` | First send `k` requests at once to the first operation. Use right after a deploy, without `-warm` | 0 |
| `-warm <duration>` | Send requests for this long first | 0 |
| `-spec <file or url>` | The OpenAPI spec | `<url>/api/openapi.json` |
| `-body 'POST /path={...}'` | A JSON body for an operation whose spec has no example. Repeat | |
| `-header 'Name: value'` | A header for every request. Repeat | |
| `-worker <name>` | The Worker's name, for the CPU figures | The first label of the URL's host |

CPU time comes from Workers Logs through Cloudflare's API: it needs `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` and observability enabled on the Worker, and waits up to two minutes. How to read the output: [Measure and improve performance](../guides/performance.md).

## SDKs

| Command | Flags | What it does |
|---|---|---|
| `sdk-list` | | The groups `fern/generators.yml` defines |
| `sdk-gen` | `<group>` | Generates one SDK with Fern (Docker) into `sdk/out/<group>` |
| `sdk-check` | `<group>` | Go: build, vet, tests against WireMock. TypeScript: typecheck. Another language: says it was only generated |
| `sdk-ready` | `[group...]` | Generates what is missing of `typescript-dist`, `go`, `cli` (or the groups named), and builds the CLI |
| `sdk-publish` | `-check`, `-quick` (with `-check`: compare a hash, no Docker), `-into <dir>` (default `sdk/go`) | Generates the Go SDK fresh, checks it, copies its sources into the committed folder. `-check`: fail if that copy is stale |
| `cli-build` | `-linux` | HEAVY. Builds the generated Rust CLI in `sdk/out/cli`, natively or for Linux in Docker |
| `sdk-clean` | | Removes `sdk/out` and stops leftover WireMock containers |

## Release

| Command | Flags | What it does |
|---|---|---|
| `dist-sdk` | | Generates, checks and archives every SDK group and the specs into `dist/`. Not the `cli` group, nor groups ending in `-dist` |
| `dist-cli` | `-linux` | HEAVY. Generates and builds the CLI into `dist/` |
| `release` | `-tag vX.Y.Z` | Attaches `dist/*` to the tag's GitHub Release, creating it if needed. A tag with a hyphen is a pre-release |
| `release-tags` (anywhere) | `-tag vX.Y.Z`, then `<module dir>...` | Tags each Go module in a subdirectory `<path in the repo>/vX.Y.Z` on the tag's commit, through the GitHub API |
| `release-tool` (anywhere) | `-tag vX.Y.Z` | GoReleaser on the tool itself, in this repo |
| `need-env` (anywhere) | `<NAME>...` | Fails, naming them, unless these variables are set |
| `github-secrets` (anywhere) | `<NAME>...` | Copies environment variables into the repo's GitHub Actions secrets. Values are never printed |

Without `-tag`, the tag is the one a GitHub workflow runs on. With neither, `release`, `release-tags` and `release-tool` are dry runs.

## The repo around a project

| Command | Flags | What it does |
|---|---|---|
| `workflows` (anywhere) | `-check`, `-into <repo dir>` | Writes `check`, `deploy`, `sdk-check` and `release` into `.github/workflows/`. A repo with an `examples/` folder gets the variant for several projects. `-check`: fail if they differ |
| `docs` (anywhere) | `-check`, `-into <repo dir>` | Writes `docs/_config.yml`, `docs/_sass/`, `docs/writing.md` and `docs/llms.txt`. It asks GitHub for the repo's name and description |
| `docs-lint` (anywhere) | `-into <repo dir>` | Checks `docs/` (below) |
| `docs-review` (anywhere) | `-print` | Hands Claude the review prompt with what the lint found. `-print`: only show the prompt |
| `upstream` (anywhere) | | Every `Upstream:` tag in the code under this folder, with its issue's state |
| `each` (anywhere) | `-only <name,...>`, then `<task> [args]` | Runs a mise task in every project below this folder that has it, one after the other |

What `docs-lint` faults:

- **A page without front matter** (`title`, `nav_order`), or a start page without `permalink: /`.
- **A page no other page links to.**
- **A link to a file or a heading that is not there.**
- **`mise run <task>`** for a task no `mise.toml` in the repo defines.
- **A path in a code span** that starts with a top-level entry of the repo and does not exist.
- **A release version written into a page.**
- **Two curly braces together,** which the site's renderer reads as template code.

Plans and findings are checked for links only.

## Add a command

1. **Write it** in a file of `cmd/charter/`: a function `func(args []string) error`, registered in that file's `init` with its usage and one line of help.
2. **Add a one-line task** that calls it, in the example's `mise.toml` (`go run ../../cmd/charter <command>`).
3. **Document it** in this page and in [Tasks](tasks.md), in the same commit.
