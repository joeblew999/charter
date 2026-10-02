---
title: The charter tool
nav_order: 1
parent: Reference
---

# The charter tool: every command and flag

The program the tasks run (`charter`) is one Go binary with a command for everything that needs more than one line of shell. This page lists every command: its flags and their defaults, what it does, what it needs, and where it runs. Read it when a task fails and you want to run its command by hand, or when you write a task of your own. The tasks that call these commands are in [Tasks](tasks.md).

## How to run it

```sh
go run github.com/joeblew999/charter/cmd/charter@latest help   # no install, needs Go: lists every command
```

As a shell function, for a session:

```sh
charter() { go run github.com/joeblew999/charter/cmd/charter@latest "$@"; }   # then: dev help
```

As a mise tool of a project, so that its tasks can call `charter <command>` and the version is pinned in one place. Put one of these two lines in `mise.toml`:

```toml
[tools]
"go:github.com/joeblew999/charter/cmd/charter" = "latest"                                   # built with Go
# or a prebuilt binary, no Go needed:
"ubi:joeblew999/charter" = { version = "latest", exe = "dev", matching = "dev_" }
```

A project made by `charter new` has the first line already, and every task in it calls `charter <command>`. In such a project:

```sh
mise exec -- charter help        # the pinned tool, from any shell
mise run doctor              # what you normally type: a task that calls the tool
```

The tool as a binary, without Go or mise, is in [Releases](releases.md#the-stable-links).

The examples on this page write `charter <command>`. Use whichever of the forms above you have.

## Where a command runs

A command works on the project it is run in. A project is a folder with a `mise.toml` beside a `fern/` folder: the command looks for one in the directory it was started in, then in each directory above it, and runs from there. Every path on this page is from that folder.

| Commands | Where they run |
|---|---|
| `new`, `version`, `each`, `workflows`, `docs`, `docs-lint`, `docs-review`, `upstream`, `bench`, `with-server`, `size`, `release-tool`, `release-tags`, `need-env`, `github-secrets` | In a project, or outside one in the directory they are started in (`charter help` marks them) |
| Every other command | Only in a project. Without one it fails: `not in a project: a project is a folder with a mise.toml beside a fern/ folder (charter new makes one)` |

"Project layout" below means what `charter new` creates ([Configuration](config.md#the-files-of-a-project)). "charter only" means the command is for the charter repo itself, or for a project shaped like one of its other examples.

A flag is written `-name value`. Flags come before arguments.

## Project

### new

```sh
charter new -name <name> [-module <go module>] [-into <dir>] [-from <checkout>]   # create a new Go API project
```

Creates a project: the notes API of charter under your project's name, with its tasks, Fern folder, tests and a `docs/` folder. It prints the next commands to run.

| Flag | Default | What it is |
|---|---|---|
| `-name` | required | The project, its Worker and its database (`<name>-db`). 3 to 42 characters: lower-case letters, digits and hyphens, starting with a letter and not ending in a hyphen. The SDK's names stay the example's (`NotesClient`) until changed in `fern/generators.yml` |
| `-module` | `github.com/<your GitHub login>/<name>` | The project's Go module path |
| `-into` | `./<name>` | Where to create it. The directory must be empty or absent |
| `-from` | the tool's own release, cloned from GitHub | A checkout of charter to copy from |

- **Needs:** Go (it runs `go mod tidy` and `gofmt`), git and network access (it clones charter at the tool's version). `gh`, logged in, only when `-module` is left out.
- **Runs:** anywhere.
- **Which version the project pins:** run as a release (`@latest`, or a downloaded binary), the project's `mise.toml` and `go.mod` name that release. Run with `-from`, or from a checkout of charter, nothing is pinned: the tasks run the tool from the checkout (`go run $CHARTER/cmd/charter`, with `CHARTER` set in `mise.toml` and a `go.work` that lets Go run it), and `go.mod` gets a `replace` line that points at the checkout's library. Remove all three once you depend on a release.

### doctor

```sh
charter doctor     # check what the tasks need
```

Prints one line per check: npm packages installed in the project's folder, Docker running, `go`, `cargo` and `gh` on the path, `FERN_TOKEN` set, no leftover WireMock containers. Then it lists the SDK groups, as `sdk-list` does. It fails only for missing npm packages or Docker not running. The others are warnings.

- **Needs:** nothing.
- **Runs:** project layout.

### upstream

```sh
charter upstream   # every workaround in the code, with the state of its upstream issue
```

Finds every comment `Upstream: <owner>/<repo>#<n>` in the files git tracks (Markdown, `mise.toml` and a `cmd/charter/` folder are left out) and asks GitHub for each issue's state. `CLOSED` means that workaround can go.

- **Needs:** a git repository, and `gh` (an issue it cannot read is shown as `UNREACHABLE`).
- **Runs:** in a project, or anywhere.

## API and database

### with-server

```sh
charter with-server -url <url> -start <cmd> [-show] -run <cmd> [-run <cmd>]...   # start a server, run commands against it, stop it
```

Starts a server, waits up to 90 seconds until the URL answers, runs each `-run` command in order, then stops the server and its children. It stops at the first command that fails. Output of the server and of the commands is shown only on failure.

| Flag | Default | What it is |
|---|---|---|
| `-url` | required | What must answer before the commands run |
| `-start` | required | The server's command, one shell line (run with `bash -c`) |
| `-show` | off | Stream the commands' output instead of showing it only when one fails |
| `-run` | at least one | A command to run while the server is up, one shell line. Repeat the flag for more |

In `-url`, `-start` and every `-run`, the text `{port}` is replaced by a free port and `{port2}` by a second one. It fails if the URL already answers before the server starts.

- **Needs:** `bash`.
- **Runs:** in a project, or anywhere.

### migrate-local

```sh
charter migrate-local [-worker <name>] [-port <port>]   # apply migrations to a running local dev server
```

Applies each file in `migrations/*.sql`, in name order, to the local D1 database of a running `cf dev`, each file once. It records what it applied in a table `_local_migrations` in that database.

| Flag | Default | What it is |
|---|---|---|
| `-worker` | the project's Worker: the first label of the host in `API_URL` | The Worker's name, as in `cloudflare.config.ts` |
| `-port` | the value of `API_PORT` | The dev server's port |

- **Needs:** the dev server running (`mise run dev`).
- **Runs:** project layout.

### migrate

```sh
charter migrate [-worker <name>]   # REMOTE: apply pending migrations to the Worker's D1 database
```

Finds the D1 database named `<worker>-db` on your Cloudflare account and applies the pending files of `migrations/` with `cf d1 migrations apply`. It fails if the database does not exist: deploy the Worker first.

| Flag | Default | What it is |
|---|---|---|
| `-worker` | the project's Worker: the first label of the host in `API_URL` | The Worker's name. The database is `<worker>-db` |

- **Needs:** npm packages installed (`mise run setup`) and a Cloudflare login (`cf auth login`, or `CLOUDFLARE_API_TOKEN`).
- **Runs:** project layout, where the project has `migrations/`.

### size

```sh
charter size -max <bytes> <file>   # fail if the file, gzipped, is larger than <bytes>
```

Prints the file's size and its size gzipped (best compression), and fails over the limit.

| Flag | Default | What it is |
|---|---|---|
| `-max` | required | The limit in bytes, gzipped |

- **Needs:** nothing.
- **Runs:** in a project, or anywhere. The file's path is from the root.

### wasm-build

```sh
charter wasm-build   # build the project's Go program into build/app.wasm for workers-go
```

Builds the Wasm a Go Worker deploys, tuned for Cloudflare Workers, and fails if it is too large. It also writes the JavaScript that runs the Wasm into `build/`: workers-go's `wasm_exec.js` and `runtime.mjs`, and the Go library's glue (`go.mjs`, `websocket.mjs`, `hub.mjs`, `tinygo-clock.mjs`), taken from the module `github.com/joeblew999/charter/go` at the version the project's `go.mod` requires (`go list -m`), so the JavaScript always matches the Go it talks to. For the Wasm it does five things TinyGo's own `tinygo build` does not:

- **Turns off one collector run per pause.** TinyGo runs a full garbage collection whenever the program waits and 32 objects with finalizers were made since the last one. workers-go makes one such object for every JavaScript value, so a request collected many times over. The build sets that one constant to 0.
- **Reuses goroutine stacks.** TinyGo allocates a stack for every goroutine and for every call from JavaScript into Go, and its collector rarely frees one. The build makes TinyGo's scheduler keep the stack of a finished goroutine, which it has just cleared, for the next goroutine: about 20 lines in one file.
- **Starts with a heap of 8 MB.** TinyGo starts with a few pages and collects each time it must grow. With 8 MB an ordinary request never fills the heap, so the collector does not run in it. A stream that lives long does fill it, and is collected then.
- **Gives each goroutine a 128 KB stack.** Huma overflows TinyGo's default of 64 KB. 96 KB ran the examples; 128 KB leaves room. If a deep contract fails with `stack overflow` or `memory access out of bounds`, raise it.
- **Checks the size:** the Wasm, gzipped, against `-max`.

The first two are patches to TinyGo's runtime, which is Go source that TinyGo compiles into every program. The build compiles against a copy of that source with the two changes, made once per TinyGo version in your cache folder. TinyGo itself, the compiler, is not rebuilt or changed. If a newer TinyGo no longer has the text a patch replaces, the build stops and says so; `-plain` builds without them.

| Flag | Default | What it is |
|---|---|---|
| `-heap` | `8` | Starting heap in MB. `0` keeps TinyGo's own |
| `-stack` | `128kb` | Stack per goroutine |
| `-opt` | `z` | TinyGo's optimisation level. Measured on Cloudflare, `2` was no faster than `z` and is larger |
| `-max` | `3000000` | Fail if the Wasm, gzipped, is larger than this many bytes |
| `-plain` | off | Build with TinyGo as it is, with no patches and no starting heap: to compare |

- **Needs:** mise. The tool pins the TinyGo and the binaryen it was tested with and asks mise for exactly those, which installs them the first time; a project does not pin TinyGo itself. `-tinygo system` builds with the `tinygo` on the path instead, untested.
- **Runs:** project layout.

What the changes save is in [Benchmarks](../benchmarks.md). The patches carry the tags `Upstream: tinygo-org/tinygo#5800` and `#5801`; when TinyGo has them, they go and the flags stay.

### bench

```sh
charter bench [-each] [-burst 8] [-warm 30s] <url>   # what each operation of an API costs, and each request
```

Works on any API with an OpenAPI spec. It reads the spec, calls every operation it can build a request for, and prints one row each: the HTTP status, the median and the slowest wall time as a client sees it, and with `-cpu` the CPU time Cloudflare recorded.

- **Which operations:** every GET whose required parameters have an example in the spec. A stream (`text/event-stream`) is skipped, and so is a body that is not JSON. `-write` adds POST, PUT, PATCH and DELETE, with the request body's example. It also requests one path that does not exist, which shows what a request costs before any handler runs. Operations it skipped are listed with the reason.
- **CPU time** is what Workers bills and limits. It comes from Workers Logs through Cloudflare's API: the median and the 99th percentile per operation, over the requests this run made.

| Flag | Default | What it is |
|---|---|---|
| `-n` | `20` | Requests per operation, after 3 that are not counted |
| `-spec` | `<url>/api/openapi.json` | The OpenAPI spec, a file or a URL |
| `-write` | off | Also call operations that change data. They do change it |
| `-cpu` | off | Also report CPU time from Cloudflare |
| `-each` | off | Also print the CPU time of every request, in the order sent. For a Go Worker run by the library's `go.mjs` each figure is marked `w` when a Go runtime started at module load served it and `n` when the request had to start one. Implies `-cpu` |
| `-burst` | `0` | First send this many requests at once to the first operation. Right after a deploy and without `-warm`, it shows what a new isolate does with them |
| `-warm` | `0` | Send requests for this long first, for example `30s`. A Worker just deployed or idle is slower at first |
| `-worker` | the first label of the URL's host | The Worker's name, for `-cpu` |
| `-header` | none | A header for every request, for example `'Authorization: Bearer <token>'`. Repeat it for more |

- **Needs:** a server at the URL. For `-cpu`: a deployed Worker with observability enabled (the example's `cloudflare.config.ts` enables it), and `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment or in fnox. The token needs permission to read Workers Observability.
- **Runs:** in a project, or anywhere.

With `-cpu` a run waits for the logs to arrive, which takes up to two minutes.

## SDKs

A project has one API and one Fern folder: `fern/`, with `fern.config.json`, the two specs and a `generators.yml`. A group is one SDK that file defines. A new project has the groups `go`, `typescript`, `typescript-dist` and `cli`.

### sdk-list

```sh
charter sdk-list   # the groups fern/generators.yml defines
```

- **Needs:** nothing.
- **Runs:** project layout.

### sdk-gen

```sh
charter sdk-gen <group>   # generate one SDK with Fern into sdk/out/<group>
```

Runs `fern generate --local` for one group. It overwrites what is there.

- **Needs:** Docker running, and npm packages installed (`mise run setup`).
- **Runs:** project layout.

### sdk-check

```sh
charter sdk-check <group>   # prove the generated SDK in sdk/out/<group> works
```

For a Go SDK (a folder with `go.mod`): build, vet and test, outside the workspace. When the SDK has a `wiremock/docker-compose.test.yml`, it starts that WireMock container first, tells the tests where it is (`WIREMOCK_URL`) and stops it after. For a TypeScript SDK (a folder with an `index.ts`, at its top or in a folder `src` or `sdk`): a typecheck with the project's TypeScript (the 5.9 installed as `typescript-sdk` where the project has one), in bundler mode with the default libs. It writes a `tsconfig.check.json` beside the entry file.

- **Needs:** Go, and Docker for the WireMock tests; or npm packages installed for TypeScript.
- **Runs:** project layout.

### sdk-publish

```sh
charter sdk-publish [-check [-quick]] [-into <dir>]   # copy the Go SDK into the committed folder another repo can go get
```

Removes `sdk/out/go`, generates it again, runs `sdk-check` on it, and replaces the committed folder with its sources: everything except Fern's run record (`.fern/`), the WireMock fixtures and the tests that need them (`wiremock/`, folders ending in `_test`), and `CONTRIBUTING.md`.  It fails when the module path in the SDK's `go.mod` does not end in the folder: Go finds a module in a subfolder by that path. How to use the result: [Giving the Go SDK to another repo](../guides/sdks.md#giving-the-go-sdk-to-another-repo).

| Flag | Default | What it is |
|---|---|---|
| `-check` | off | Write nothing: generate, and fail if the committed folder differs, naming the files. Passes when the folder does not exist |
| `-quick` | off | With `-check`: generate nothing, so no Docker. The committed folder holds a file `.made-from` with a hash of the specs and `generators.yml` it was generated from; this fails if they have changed since. It fails too for a folder published before that file existed: publish it again |
| `-into` | `sdk/go` | The committed folder |

- **Needs:** Docker running, npm packages installed, Go.
- **Runs:** project layout.

### sdk-ready

```sh
charter sdk-ready [group...]   # generate and build what the tests use, if it is missing
```

With no group: generates `typescript-dist`, `go` and `cli` when their output is missing, and builds the CLI when it has no binary. With groups: only those. A group other than those three counts as present when its folder exists. It never regenerates something that is there.

- **Needs:** Docker when something is missing; `cargo` for the CLI.
- **Runs:** project layout.

### sdk-clean

```sh
charter sdk-clean   # remove generated SDKs and stop leftover WireMock containers
```

Removes `sdk/out/` and `sdk/.work/`, and removes every running container of the image `wiremock/wiremock:3.9.1` on this machine, whichever project started it.

- **Needs:** nothing (without Docker it only removes the folders).
- **Runs:** project layout.

### cli-build

```sh
charter cli-build [-linux]   # HEAVY: build the generated Rust CLI in sdk/out/cli
```

Builds the first binary named in the folder's `Cargo.toml`, in release mode with `rustls`. The first build compiles every dependency: minutes at full CPU. The binary is `sdk/out/cli/target/release/<name>`, or with `-linux` `sdk/out/cli/target-linux/release/<name>`.

| Flag | Default | What it is |
|---|---|---|
| `-linux` | off | Build for Linux inside Docker, for the container's architecture, with the Rust version that `mise.toml` pins |

- **Needs:** `cargo`; or Docker with `-linux`.
- **Runs:** project layout.

### dist-sdk

```sh
charter dist-sdk   # generate, check and archive the SDK sources and specs into dist/
```

Removes and regenerates the `go` and `typescript` groups, checks each as `sdk-check` does, and writes `dist/<project>-sdk-go.tar.gz`, `dist/<project>-sdk-typescript.tar.gz` and `dist/<project>-specs.tar.gz` (the two spec files), where `<project>` is the name of the project's folder. `dist/` ignores itself: it needs no line in `.gitignore`.

- **Needs:** Docker.
- **Runs:** project layout.

### dist-cli

```sh
charter dist-cli [-linux]   # HEAVY: generate and build the Fern CLI into dist/
```

Generates the `cli` group, builds it as `cli-build` does, and copies the binary to `dist/<project>-cli-<os>-<arch>`.

| Flag | Default | What it is |
|---|---|---|
| `-linux` | off | Build for Linux inside Docker instead of for this machine |

- **Needs:** Docker, and `cargo` without `-linux`.
- **Runs:** project layout.

### harness-sync, harness-test, harness-deploy

```sh
charter harness-sync            # copy the showcase TypeScript SDK into the SDK test Worker
charter harness-test [-remote]  # run the SDK test inside that Worker
charter harness-deploy          # REMOTE: deploy that Worker, twice
```

The SDK test Worker of charter (`examples/showcase-ts/`) runs Fern's TypeScript SDK inside workerd.

| Flag | Default | What it is |
|---|---|---|
| `-remote` (of `harness-test`) | off | Test the deployed Worker (`API_URL`, `API_MOCK_URL`) instead of a local `cf dev` |

- **Needs:** Docker when the SDK is missing or stale; a Cloudflare login for `harness-deploy`.
- **Runs:** charter only.

## Docs

### docs

```sh
charter docs [-into <repo dir>] [-check]   # write the docs site's config into a repo's docs/ folder
```

Writes four files, the same for every repo apart from its name, description and URLs, which it asks GitHub for: `docs/_config.yml`, `docs/_sass/custom/custom.scss`, `docs/writing.md` and `docs/llms.txt`. GitHub Pages renders `docs/` with them. It prints the site's URL.

| Flag | Default | What it is |
|---|---|---|
| `-into` | `.` | The repo to write into |
| `-check` | off | Write nothing, and fail if a file differs. Where there is no `docs/_config.yml` yet it says so and passes |

- **Needs:** `gh`, logged in, and a GitHub repository for the directory (`gh repo view` must work there).
- **Runs:** anywhere.

### docs-lint

```sh
charter docs-lint [-into <repo dir>]   # check docs/ for what a program can check
```

Checks every Markdown page under `docs/`: front matter with `title` and `nav_order` (and `permalink: /` on `docs/README.md`); no template braces; a link to it from another page; the file and the anchor of every relative link; every `mise run <task>` against `mise.toml`; every path in a code span that starts with a top-level entry of the repo (a path git ignores passes); and no hard-coded release version of charter. Tasks, paths and versions are not checked in pages under `plans/`, in `findings.md` or in `writing.md`. It prints one line per problem and fails if there is one.

| Flag | Default | What it is |
|---|---|---|
| `-into` | `.` | The repo whose `docs/` to check |

- **Needs:** nothing.
- **Runs:** anywhere.

### docs-review

```sh
charter docs-review [-print]   # have Claude bring docs/ up to date
```

Runs `docs-lint`, puts its output into a review prompt, and hands that to the `claude` command, which may edit files and run `mise run docs:lint`, `mise run charter:check` and `mise tasks`. Read its edits with `git diff` before committing.

| Flag | Default | What it is |
|---|---|---|
| `-print` | off | Print the prompt and stop |

- **Needs:** the `claude` command (Claude Code) on the path, except with `-print`.
- **Runs:** anywhere.

## Workflows and releases

### workflows

```sh
charter workflows [-check] [-into <repo dir>]   # write the GitHub workflows: check, deploy, sdk-check, release
```

Writes GitHub workflows into `.github/workflows/`. Each step that does work is `mise run <task>`. In a project made by `charter new` it writes four: `check.yml`, `deploy.yml`, `sdk-check.yml` and `release.yml`, with the jobs of one Go API. It only writes files that differ.

| Flag | Default | What it is |
|---|---|---|
| `-check` | off | Write nothing, and fail if a workflow differs from its template. Where there is no `.github/workflows/` yet it says so and passes |
| `-into` | the root | The repo to write into |

- **Needs:** nothing. The repo needs a `mise.toml` with the tasks the workflows name.
- **Runs:** anywhere.

### release

```sh
charter release [-tag vX.Y.Z]   # attach dist/* to the tag's GitHub Release
```

Lists the files in `dist/` with their sizes. With a tag it creates the tag's GitHub Release when there is none (marked as a pre-release when the tag has a hyphen) and uploads the files, replacing files of the same name. Without a tag it is a dry run: nothing is published. It fails if `dist/` is empty.

| Flag | Default | What it is |
|---|---|---|
| `-tag` | the tag the GitHub workflow runs on (`GITHUB_REF_NAME` when `GITHUB_REF_TYPE` is `tag`); otherwise none | The version tag: `v1.2.3`, or `v1.2.3-rc.1` for a pre-release |

- **Needs:** `gh` with write access to the repo, for a real run. The tag must exist on GitHub.
- **Runs:** in a project, or anywhere.

### release-tags

```sh
charter release-tags [-tag vX.Y.Z] <module dir>...   # tag each Go module in a subdirectory
```

For each directory (it must have a `go.mod`; one that does not exist is skipped, as `sdk/go` is before the first `sdk-publish`), adds the tag `<the directory's path in the repo>/vX.Y.Z` on the commit of `vX.Y.Z`, through the GitHub API. A tag that exists on that commit is left; one that exists on another commit is an error. Without a tag it is a dry run that prints the tags it would add. Why these tags exist: [Releases](releases.md#the-tags-of-a-release).

| Flag | Default | What it is |
|---|---|---|
| `-tag` | as for `release` | The version tag |

- **Needs:** `gh` with write access, and the tag in the local checkout, for a real run.
- **Runs:** in a project, or anywhere.

### release-tool

```sh
charter release-tool [-tag vX.Y.Z]   # GoReleaser on the charter tool itself
```

Runs GoReleaser with `.goreleaser.yaml`. On a version tag it publishes the tool's archives and checksums to the tag's GitHub Release. Without one it builds a snapshot into `dist/`.

| Flag | Default | What it is |
|---|---|---|
| `-tag` | as for `release` | The version tag |

- **Needs:** `goreleaser`, and `GH_TOKEN` or `GITHUB_TOKEN` to publish.
- **Runs:** charter only.

### need-env

```sh
charter need-env <NAME>...   # fail unless these environment variables are set
```

Fails, naming the ones that are missing or empty. A workflow runs it first, so a missing secret is one clear line.

- **Needs:** nothing.
- **Runs:** in a project, or anywhere.

### github-secrets

```sh
charter github-secrets <NAME>...   # REMOTE: copy environment variables into the repo's GitHub Actions secrets
```

Sets each named variable as a repository secret with `gh secret set`. The value goes on standard input, so it is in no command line and no output. It fails if a variable is not set where it runs.

- **Needs:** `gh`, logged in as someone who may set the repo's secrets, and the variables in the environment (`mise run cloudflare:secrets` runs it under `fnox exec`).
- **Runs:** in a project, or anywhere.

## Every command

The same list as `charter help` prints, with where each one runs.

| Command | Group | Runs |
|---|---|---|
| `bench` | API and database | Anywhere |
| `cli-build` | SDKs | Project layout |
| `dist-cli` | SDKs | Project layout |
| `dist-sdk` | SDKs | Project layout |
| `docs` | Docs | Anywhere |
| `docs-lint` | Docs | Anywhere |
| `docs-review` | Docs | Anywhere |
| `doctor` | Project | Project layout |
| `github-secrets` | Workflows and releases | Anywhere |
| `harness-deploy` | SDKs | charter only |
| `harness-sync` | SDKs | charter only |
| `harness-test` | SDKs | charter only |
| `migrate` | API and database | Project layout |
| `migrate-local` | API and database | Project layout |
| `need-env` | Workflows and releases | Anywhere |
| `new` | Project | Anywhere |
| `release` | Workflows and releases | Project layout |
| `release-tool` | Workflows and releases | charter only |
| `release-tags` | Workflows and releases | Anywhere |
| `sdk-check` | SDKs | Project layout |
| `sdk-clean` | SDKs | Project layout |
| `sdk-gen` | SDKs | Project layout |
| `sdk-list` | SDKs | Project layout |
| `sdk-publish` | SDKs | Project layout |
| `sdk-ready` | SDKs | Project layout |
| `size` | API and database | Anywhere |
| `upstream` | Project | Anywhere |
| `wasm-build` | API and database | Project layout |
| `with-server` | API and database | Anywhere |
| `workflows` | Workflows and releases | Anywhere |

## Limits

- **No Windows build.** The tool starts and stops process groups, which is Unix-only. It runs on Linux and macOS.
- **`charter help` is the only help.** A command that takes flags prints them when given `-h`. There is no longer help per command.
- **`docs-lint` and `upstream` need a git repository.** In a project that has not had `git init`, `docs-lint` reports ignored paths (`sdk/out/`) as missing, and `upstream` finds no tags.
- **What was run for this page:** `new`, `help`, `workflows`, `workflows -check`, `docs -check`, `docs-lint`, `sdk-list`, `doctor`, `upstream`, `release` and `release-tags` (dry runs) and `migrate` were run on 2026-10-01 in a new project, and so were `sdk-publish` and `sdk-publish -check`, with the tool built from a checkout. The other commands are described from their source.
