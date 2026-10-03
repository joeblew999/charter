---
title: The charter command
nav_order: 2
parent: Reference
---

# The charter command: every command and flag

The tool the tasks run (`cmd/charter/`). `charter help` lists the commands. A command works on the project it is run in: the nearest folder upwards with a `mise.toml` beside a `fern/` folder. Commands marked * also run outside one. Flags come before arguments.

| Where | How to run it |
|---|---|
| Anywhere, with Go | `go run github.com/joeblew999/charter/cmd/charter@latest <command>` |
| In a project | `charter <command>`: `mise.toml` pins it |
| In this repo | `go run ./cmd/charter <command>` |

## Make a project

| Command | Flags | What it does |
|---|---|---|
| `new` * | `-name <name>` (required), `-module <go module>` (default `github.com/<gh login>/<name>`), `-subdomain <workers.dev subdomain>` (default a placeholder), `-into <empty dir>` (default `./<name>`), `-from <checkout>` | Copies `examples/notes-go/` (without `sdk/go/`) under your name, adds docs and workflows, pins the tool and the library to its own release (with `-from`, to the checkout) |
| `version` * | | The release the tool is |

## Build

| Command | Flags | What it does |
|---|---|---|
| `wasm-build` | below | Writes the Worker glue into `build/`, patches a copy of TinyGo's runtime once per TinyGo version, builds `build/app.wasm` and checks its size |
| `size` * | `-max <bytes> <file>` | The size check on its own |

### wasm-build

| Flag | Meaning | Default |
|---|---|---|
| `-heap <MB>` | The starting heap; 0 is TinyGo's own | 8 |
| `-stack <size>` | The stack per goroutine | `128kb` |
| `-opt <level>` | TinyGo's optimisation level: `z`, `s`, `1`, `2` | `z` |
| `-max <bytes>` | Fail if the Wasm, gzipped, is larger | 3000000 |
| `-tinygo <which>` | `pinned` (the one tested, from mise) or `system` (on the path, untested) | `pinned` |
| `-plain` | TinyGo as it is: no patch, no starting heap | off |

| Patch | Upstream |
|---|---|
| `finalizerGCThreshold` 32 becomes 0 (`src/runtime/gc_finalizer.go`) | tinygo-org/tinygo#5800 |
| A finished goroutine's stack is kept for the next (`src/internal/task/task_asyncify.go`); left out with a TinyGo that has the fix | tinygo-org/tinygo#5801 |

## Run and test

| Command | Flags | What it does |
|---|---|---|
| `with-server` * | `-url <url>`, `-env NAME=VALUE`, `-start <cmd>`, `-run <cmd>`, `-show` | Starts a server, waits for the URL, runs the commands, stops the server and what it started. `{port}`, `{port2}`: free ports. Output only on failure, or with `-show` |
| `exec` * | `-env NAME=VALUE`, `-quiet`, then `<program> [args]` | Runs a program with those variables; `-quiet` shows its output only if it fails |
| `lint` * | `-vet <packages>`, `-wasm <packages>`, then `<file or folder>...` | Fails if `gofmt` would change a file, then `go vet` for the host and for Wasm |
| `migrate-local` | `-port <port>`, `-worker <name>` | Applies `migrations/*.sql` to a running dev server's local D1 |
| `migrate` | `-worker <name>` | REMOTE. Applies pending migrations to `<worker>-db` |
| `doctor` | | Says what the project's tasks need and lack |
| `harness-sync`, `harness-test`, `harness-deploy` | `harness-test -remote` | For a Worker that imports its own generated SDK (`conformance/showcase-ts/`): copy it in, test it under `cf dev` or deployed, deploy it twice |

`with-server` and `exec` run a program an npm package of the project installs (`cf`, `fern`) from `node_modules/.bin`, never from the path.

## Measure

| Command | Flags | What it does |
|---|---|---|
| `bench` * | below, then `[<url>]` (default `API_URL`) | Times every GET operation with examples in the spec, plus one missing path |
| `perf` | `-name <experiment>`, `-build '<wasm-build flags>'`, `-prebuilt`, `-keep`, then `-- <bench flags>` | REMOTE. Builds, deploys to `<worker>-perf-<experiment>`, benches it from its first request, deletes it |
| `perf-clean` | `-all` | REMOTE. Deletes scratch Workers and databases; without `-all`, not those of the last 20 minutes |

### bench

| Flag | Meaning | Default |
|---|---|---|
| `-n <count>` | Requests per operation, after 3 warm-up ones | 20 |
| `-write` | Also POST, PUT, PATCH and DELETE | off |
| `-cpu` | Also Cloudflare's CPU time, median and p99 (needs the two `CLOUDFLARE_` variables) | off |
| `-each` | Also every request's CPU and runtime kind. Implies `-cpu` | off |
| `-burst <k>` | First send `k` requests at once | 0 |
| `-warm <duration>` | Send requests for this long first | 0 |
| `-spec <file or url>` | The OpenAPI spec | `<url>/api/openapi.json` |
| `-body 'POST /path={...}'` | A JSON body for an operation without an example. Repeat | |
| `-header 'Name: value'` | A header for every request. Repeat | |
| `-worker <name>` | The Worker's name, for the CPU figures | From the URL |

## SDKs and releases

The task `sdk:gen` runs `sdk-gen`, and so on ([Tasks](tasks.md)).

| Command | Arguments and flags |
|---|---|
| `sdk-list`, `sdk-clean`, `dist-sdk` | none |
| `sdk-gen`, `sdk-check` | `<group>` |
| `sdk-ready` | `[group...]`: default `typescript-dist`, `go`, `cli` |
| `sdk-publish` | `-check` (fail if `sdk/go` is stale), `-quick` (with it: by a hash), `-into <dir>` |
| `cli-build`, `dist-cli` | `-linux`: for Linux, in Docker |
| `release` * | `-tag vX.Y.Z`; without a tag or a workflow's tag, a dry run |
| `dist-ts` * | `-tag vX.Y.Z`: builds `ts/` and packs it as `dist/charter-ts-X.Y.Z.tgz`, in this repo |
| `release-tags` * | `-tag vX.Y.Z`, then `<module dir>...` |
| `release-tool` * | `-tag vX.Y.Z`: GoReleaser on the tool, in this repo |

## The repo around a project

| Command | Flags | What it does |
|---|---|---|
| `workflows` * | `-check`, `-into <repo dir>` | Writes `.github/`: the workflows `check`, `deploy`, `sdk-check`, `release`, the issue forms and `labels.tsv` |
| `issue` * | `<bug\|feature\|upstream>` | Prints an issue body with that form's headings, for `gh issue create --body-file` |
| `labels` * | | REMOTE. Creates or updates the repo's labels from the labels file |
| `docs` * | `-check`, `-into <repo dir>` | Writes the docs site's config, `docs/writing.md`, `docs/llms.txt` |
| `docs-lint` * | `-into <repo dir>` | Fails on missing front matter, an unlinked page, a dead link or anchor, an unknown task, a missing path, a release version |
| `docs-review` * | `-print` | Hands Claude the review prompt with what the lint found |
| `upstream` * | | Every `Upstream:` tag in the code, with its issue's state |
| `each` * | `-only <name,...>`, then `<task> [args]` | Runs a mise task in every project below this folder that has it |
| `need-env` * | `<NAME>...` | Fails unless these variables are set |
| `github-secrets` * | `<NAME>...` | Copies variables into the repo's GitHub secrets, never printing them |
