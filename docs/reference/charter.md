---
title: The charter command
nav_order: 2
parent: Reference
---

# The charter command: every command and flag

The tool the tasks run (`charter`, in `cmd/charter/`). Every task in a project's `mise.toml` is one line, and what needs more is a command here. Read it for a command's flags, or to use the tool outside a task.

| Where | How to run it |
|---|---|
| Anywhere, with Go | `go run github.com/joeblew999/charter/cmd/charter@latest <command>` |
| In a project made by `charter new` | `charter <command>`: `mise.toml` pins it under `[tools]` |
| In this repo | `go run ./cmd/charter <command>` |

`charter help` lists the commands. **A command works on the project it is run in:** the nearest folder, from where it is started upwards, with a `mise.toml` beside a `fern/` folder. Commands marked * also run outside one. Flags come before arguments.

## Make a project

| Command | Flags | What it does |
|---|---|---|
| `new` * | `-name <name>` (required), `-module <go module>`, `-subdomain <workers.dev subdomain>`, `-into <dir>`, `-from <checkout>` | Creates a Go project: a copy of `examples/notes-go/` under your name |
| `version` * | | The release the tool is, which is what `new` pins a project to |

It copies the example's files (without `sdk/go/`) from the tool's own release, or from `-from`, or from the checkout it is run in. It renames the Worker, its URL, the Go module and Fern's organisation; adds `README.md`, `AGENTS.md`, `CLAUDE.md`, three pages in `docs/` and the four GitHub workflows; and pins the tool in `mise.toml` and the library in `go.mod` to the tool's release (from a checkout, both point at the checkout).

Defaults: `-module` is `github.com/<gh login>/<name>`, `-subdomain` a placeholder, `-into` is `./<name>` and must be empty.

## Build

### wasm-build

Builds the project's Go Worker into `build/`.

| Flag | Meaning | Default |
|---|---|---|
| `-heap <MB>` | The starting heap. 0: TinyGo's own | 8 |
| `-stack <size>` | The stack per goroutine. Huma overflows TinyGo's default | `128kb` |
| `-opt <level>` | TinyGo's optimisation level: `z`, `s`, `1`, `2` | `z` |
| `-max <bytes>` | Fail if the Wasm, gzipped, is larger: the Workers Free limit | 3000000 |
| `-tinygo <which>` | `pinned`: the TinyGo the tool was tested with, which it asks mise for. `system`: the one on the path, untested | `pinned` |
| `-plain` | TinyGo as it is: no patch, no starting heap. To compare | off |

In order, it:

1. **Writes workers-go's JavaScript and the library's Worker glue** into `build/`. The glue is `worker/*.mjs` of the library as the project's `go.mod` resolves it, so it matches the Go the project builds against.
2. **Makes a patched copy of TinyGo's runtime source,** once per TinyGo version, in your cache folder. TinyGo itself is not rebuilt.
3. **Builds** `build/app.wasm` and checks its size.

| Patch | Why | Upstream |
|---|---|---|
| `finalizerGCThreshold` 32 becomes 0 (`src/runtime/gc_finalizer.go`) | TinyGo collected whenever the scheduler went idle after 32 finalizers, and workers-go registers one per JavaScript value. The collector still runs when the heap is full | tinygo-org/tinygo#5800 |
| A finished goroutine's stack is kept for the next (`src/internal/task/task_asyncify.go`) | TinyGo allocated a stack per goroutine and per call from JavaScript, and rarely freed one. The patch leaves itself out with a TinyGo that has the fix | tinygo-org/tinygo#5801 |

What each was worth: [Benchmarks](../benchmarks.md). `size -max <bytes> <file>` * is the size check on its own.

## Run and test

| Command | Flags | What it does |
|---|---|---|
| `with-server` * | `-url <url>`, `-start <cmd>`, `-run <cmd>` (repeat), `-show` | Starts a server, waits for the URL, runs the commands, stops it. `{port}` in any of them is a free port, `{port2}` another. Output shows only on failure, or always with `-show` |
| `migrate-local` | `-port <port>`, `-worker <name>` | Applies `migrations/*.sql` to a running dev server's local D1, each once |
| `migrate` | `-worker <name>` | REMOTE. Applies pending migrations to the database `<worker>-db` |
| `doctor` | | Says what the project's tasks need and lack |
| `harness-sync` | | Copies the compiled TypeScript SDK into the project's Worker (`src/client`) |
| `harness-test` | `-remote` | Runs the SDK test inside the project's Worker: under `cf dev`, or deployed |
| `harness-deploy` | | REMOTE. Deploys the project's Worker twice: as itself and as `<worker>-api` |

The `harness-` commands are for a project whose Worker imports its own generated SDK: `examples/showcase-ts/`.

## Measure

| Command | Flags | What it does |
|---|---|---|
| `bench` * | below, then `[<url>]` (default `API_URL`) | Times every operation of an API from its OpenAPI spec |
| `perf` | `-name <experiment>`, `-build '<wasm-build flags>'`, `-prebuilt`, `-keep`, then `-- <bench flags>` | REMOTE. Builds, deploys to a scratch Worker `<worker>-perf-<experiment>`, benches it from its first request, deletes it |
| `perf-clean` | `-all` | REMOTE. Deletes scratch Workers and their databases; without `-all`, not those of the last 20 minutes |

### bench

It calls GET operations whose required inputs have examples in the spec, plus one path that does not exist. It skips streams and bodies that are not JSON.

| Flag | Meaning | Default |
|---|---|---|
| `-n <count>` | Requests per operation, after 3 warm-up ones | 20 |
| `-write` | Also POST, PUT, PATCH and DELETE | off |
| `-cpu` | Also the CPU time Cloudflare measured: median and p99 | off |
| `-each` | Also every request's CPU in the order sent, with the kind of Go runtime it got. Implies `-cpu` | off |
| `-burst <k>` | First send `k` requests at once. Use right after a deploy, without `-warm` | 0 |
| `-warm <duration>` | Send requests for this long first | 0 |
| `-spec <file or url>` | The OpenAPI spec | `<url>/api/openapi.json` |
| `-body 'POST /path={...}'` | A JSON body for an operation without an example. Repeat | |
| `-header 'Name: value'` | A header for every request. Repeat | |
| `-worker <name>` | The Worker's name, for the CPU figures | From the URL |

CPU time comes from Workers Logs: it needs `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` and observability enabled on the Worker, and waits up to two minutes.

## SDKs and releases

What each does is in [Tasks](tasks.md): the task `sdk:gen` runs `sdk-gen`, and so on.

| Command | Arguments and flags |
|---|---|
| `sdk-list`, `sdk-clean`, `dist-sdk` | none |
| `sdk-gen`, `sdk-check` | `<group>` |
| `sdk-ready` | `[group...]`: default `typescript-dist`, `go`, `cli` |
| `sdk-publish` | `-check`: fail if `sdk/go` is stale. `-quick`, with it: by a hash, no Docker. `-into <dir>`: default `sdk/go` |
| `cli-build`, `dist-cli` | `-linux`: for Linux, in Docker |
| `release` | `-tag vX.Y.Z` |
| `release-tags` * | `-tag vX.Y.Z`, then `<module dir>...` |
| `release-tool` * | `-tag vX.Y.Z`: GoReleaser on the tool itself, in this repo |

Without `-tag`, the tag is the one a GitHub workflow runs on. With neither, the three `release` commands are dry runs.

## The repo around a project

| Command | Flags | What it does |
|---|---|---|
| `workflows` * | `-check`, `-into <repo dir>` | Writes a repo's `.github/`: the workflows `check`, `deploy`, `sdk-check` and `release`, the issue forms (a bug, a feature, an upstream bug; blank issues off) and `labels.tsv`. A repo with an `examples/` folder gets the workflows for several projects |
| `issue` * | `<bug\|feature\|upstream>` | Prints an issue body with that form's headings, for `gh issue create --body-file`: gh and the API ignore forms |
| `labels` * | | Remote: creates or updates the repo's GitHub labels from the labels file |
| `docs` * | `-check`, `-into <repo dir>` | Writes the docs site's config, `docs/writing.md` and `docs/llms.txt` |
| `docs-lint` * | `-into <repo dir>` | Checks `docs/` (below) |
| `docs-review` * | `-print` | Hands Claude the review prompt with what the lint found |
| `upstream` * | | Every `Upstream:` tag in the code, with its issue's state |
| `each` * | `-only <name,...>`, then `<task> [args]` | Runs a mise task in every project below this folder that has it |
| `need-env` * | `<NAME>...` | Fails unless these variables are set |
| `github-secrets` * | `<NAME>...` | Copies variables into the repo's GitHub secrets, never printing them |

`docs-lint` faults: a page without `title` and `nav_order`; a page nothing links to; a dead link or anchor; `mise run <task>` for a task no `mise.toml` defines; a path in a code span that does not exist; a release version written into a page. Plans and findings are checked for links only.

## Add a command

1. **Write it** in `cmd/charter/`: a function `func(args []string) error`, registered in its file's `init` with its usage and one line of help.
2. **Add a one-line task** that calls it, in the example's `mise.toml`.
3. **Document it** here and in [Tasks](tasks.md), in the same commit.
