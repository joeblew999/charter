---
title: The charter tool, workflows, releases
nav_order: 8
parent: This repository
---
# cmd/charter/: the tool the tasks run

Every task in `mise.toml` is one line. Anything that needs more than one line is a command of the `charter` tool: a Go program in `cmd/charter/` (standard library only), so the logic is real code that can be read, tested and reused, not shell inside TOML. Read this page to add a task, to change a GitHub workflow, to cut a release, to start a new project, or to set up the docs site.

```sh
mise tasks                   # every task, with what it does
mise run <task>              # what you normally type: each task is one line that calls a tool or a charter command
go run ./cmd/charter help            # every charter command, with its flags
mise run charter:check           # the tool's own checks: gofmt, vet, tests, and the workflows and docs config match their templates
```

`mise run charter:check` asks GitHub for the repo's name (`gh repo view`), so it needs `gh` logged in.

## The commands

| Command | What it does | Task that calls it |
|---|---|---|
| `with-server` | Starts a server, waits up to 90 s until a URL answers, runs commands against it, stops it. `{port}` in its arguments is a free port, `{port2}` another. A command's output is shown only if it fails; `-show` streams it | `test:native`, `test:workerd` (the Go examples) |
| `migrate-local` | Applies `examples/notes-go/migrations/*.sql` to a running dev server's local D1, each once (cf can't) | `migrate:local`, `test:workerd` (the notes examples) |
| `migrate` | Finds the Worker's D1 database (`<worker>-db`) and applies pending migrations | `migrate`, `deploy` (the notes examples) |
| `wasm-build` | Builds a Go program into the Wasm a Worker deploys, tuned for Workers (a patched copy of TinyGo's runtime and an 8 MB starting heap: [benchmarks.md](benchmarks.md)), and checks its size. It also writes the Go library's Worker glue (`go/worker/*.mjs`) into the program's `build/`, from the library module as the program's `go.mod` resolves it. `-plain` builds with TinyGo as it is | `build` (the Go examples) |
| `size` | Fails if a file is over a gzipped size | none: `wasm-build` does it |
| `sdk-gen`, `sdk-check`, `sdk-ready`, `sdk-list`, `sdk-clean` | Fern: generate an SDK, prove it works, make what the tests need | `sdk:gen`, `sdk:check`, `sdk:ready`, `sdk:list`, `sdk:clean` |
| `sdk-publish` | Copies the Go API's Go SDK, generated fresh and checked, into `examples/notes-go/sdk/go`: the committed Go module another repo fetches with `go get`. `-check` fails when that copy is stale; `-check -quick` says so from a hash of the specs, with no Docker | `sdk:publish`, `sdk:publish:check`, `sdk:publish:fresh` |
| `cli-build` | Builds the Fern CLI (Rust), natively or with `-linux` for Linux in Docker | `sdk:cli:build` |
| `harness-sync`, `harness-test`, `harness-deploy` | Fern's TypeScript SDK inside the harness Worker (`examples/showcase-ts/`). `harness-sync` copies the SDK in, generating it again when a showcase spec is newer than it | `lint`, `test:workerd`, `deploy` |
| `bench` | Calls every operation of an API that its OpenAPI spec has examples for and prints wall time; `-cpu` adds the CPU time Cloudflare recorded, `-write` the operations that change data ([benchmarks.md](benchmarks.md)) | `bench`, `perf`, and the repo's `compare` |
| `new` | Creates a new Go API project from this repo's example ([below](#a-new-project-charter-new)) | none: run it with `go run ...charter@latest new` |
| `version` | Prints the release the tool is: what `new` pins a project to | none |
| `docs` | Writes the docs site's config, `docs/writing.md` and `docs/llms.txt` into a repo; `-check` fails if one differs ([below](#the-docs-site)) | `docs:setup`, `charter:check` |
| `docs-lint` | Checks `docs/` for what a program can check | `docs:lint` |
| `docs-review` | Hands Claude the review prompt with what `docs-lint` found; `-print` only shows the prompt | `docs:review` |
| `upstream` | Lists every `Upstream: owner/repo#n` tag in the code with the issue's state | `upstream:status` |
| `doctor` | Checks the tools and installs the project's tasks need, and lists its SDK groups | `doctor` |
| `each` | Runs one mise task in every project below the folder (the examples) that defines it, one after the other; `-only notes-ts,notes-go` picks and orders them | the repo's `setup`, `examples:check`, `compare`, `doctor`, `sdk:clean` |
| `workflows` | Writes the GitHub workflows from the templates in `cmd/charter/workflows/`; `-check` fails if a committed one differs | `workflows`, `charter:check` |
| `release-tool` | Builds this tool for Linux and macOS and publishes it to the tag's GitHub release, with GoReleaser (`.goreleaser.yaml`) | `charter:release` |
| `dist-sdk`, `dist-cli` | Build what an SDK release ships into `dist/`: the SDK sources and specs, the Fern CLI | `sdk:dist`, `sdk:dist:cli` |
| `release`, `release-tags` | Attach `dist/` to the tag's GitHub Release; tag the Go modules. Without a version tag both are a dry run | `release`, `release:tags` |
| `need-env` | Fails, naming them, unless the given environment variables are set | `cloudflare:token` |
| `github-secrets` | Copies the given environment variables into the repo's GitHub Actions secrets, without printing them | `cloudflare:secrets` |

A command works on the project it is run in: the nearest folder, from where it was started upwards, that has a `mise.toml` beside a `fern/` folder. Each example is one, and so is what `charter new` makes. Some also run outside a project, in the folder they are started in (`charter help` marks them): `new`, `version`, `each`, `workflows`, `docs`, `docs-lint`, `docs-review`, `upstream`, `bench`, `with-server`, `size`, `release-tool`, `release-tags`, `need-env` and `github-secrets`. The repo's own tasks call the tool as `go run ./cmd/charter`, an example's as `go run ../../cmd/charter` (the root `go.work` lets Go run it from there), and a project made by `charter new` as `charter`, which mise installs at the pinned release.

## Adding a task

1. If it's one command, write the one-line task in `mise.toml`. A task is plain `sh`: on Linux mise runs dash, so no bashisms.
2. If it isn't, add a command in `cmd/charter/` (register it in an `init`, as the others do) and write a one-line task that calls it.

## GitHub workflows

The workflows are templates in `cmd/charter/workflows/`, compiled into the tool. `mise run workflows` writes them to `.github/workflows/`, and `mise run charter:check` fails if a committed one differs. Edit the template, never the copy.

Every step that does work is `mise run <task>`, so a failing step is one line you can run locally. A test (`cmd/charter/workflows_test.go`) holds the templates to that, to exact versions of the actions and runners, and to tasks that exist. One set of templates serves two kinds of repo: lines between `# if-dir examples` and `# end` are written only into a repo that has an `examples/` folder (this one), the lines after `# else` only into one that doesn't (a project).

In this repo:

| Workflow | When | What it runs | Secrets |
|---|---|---|---|
| `check` | push to main, pull requests, by hand | `charter:check`, `go:check`, and `setup` then `check` in each example, one job each | none |
| `sdk-check` | push to main, pull requests, by hand | `sdk:gen` + `sdk:check` for the Go and TypeScript SDKs of each example; and `sdk:publish:check` in `examples/notes-go` | none |
| `deploy` | by hand only (pick `notes-go` or `notes-ts`) | in that example: `cloudflare:token`, `setup`, `deploy`, then `live-test` against the Worker it just deployed | `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` |
| `release` | a version tag. A dry run by hand, and on pull requests that touch `cmd/`, `go/`, `examples/`, `.goreleaser.yaml`, `mise.toml` or `go.mod` | `charter:release` (GoReleaser), `release:tags`; and in `examples/notes-go` and `examples/notes-ts`: `sdk:dist`, `sdk:dist:cli`, `release` | none (the workflow's own token) |

`deploy` fails at its first step, naming the secrets, when they are missing. Set them once per repo with `mise run cloudflare:secrets`: it copies both values from fnox (the keychain) into the repo's GitHub secrets, without printing them. Whether this repo's secrets are set, and whether `deploy` has run, was not checked for this page; the recorded deploys were made from a machine ([findings.md](findings.md)). The showcases' Workers are not among the workflow's choices: deploy them with `mise run deploy` in their folders.

### Getting the latest release

Nothing in these docs names a version, so nothing here goes stale: every link below follows the newest release by itself. The current one: [![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

```sh
go run github.com/joeblew999/charter/cmd/charter@latest help                 # the tool, no install (needs Go)
go get github.com/joeblew999/charter/go@latest                   # the Go packages
curl -fsSL https://github.com/joeblew999/charter/releases/latest/download/charter_darwin_arm64.tar.gz | tar xz dev   # the tool as a binary
```

As a mise tool in any project (`charter` is then on the path of that project's tasks):

```toml
[tools]
"go:github.com/joeblew999/charter/cmd/charter" = "latest"                                   # built with Go
# or a prebuilt binary, no Go needed:
"ubi:joeblew999/charter" = { version = "latest", exe = "dev", matching = "dev_" }
```

A project made by `charter new` has the first line already, with the release it was made from; `mise up` moves it on.

- **The release page:** https://github.com/joeblew999/charter/releases/latest
- **Any file of the latest release:** `https://github.com/joeblew999/charter/releases/latest/download/<file>`, with the file names from the table below (they carry no version for this reason).
- **`mise run docs:lint` fails on a hard-coded release version** in a page (`@v1.2.3`, a `releases/tag/` or `releases/download/v...` link). Use `@latest`, `releases/latest`, or the placeholder `vX.Y.Z`. Findings and plans may name versions: they record what was.

### Cutting a release

```sh
git tag vX.Y.Z && git push origin vX.Y.Z        # on the commit to release, once its checks are green
```

The tag must be a semantic version: `v1.2.3`, or `v1.2.3-rc.1` for a pre-release. The release workflow builds everything again from that commit and put it on the tag's GitHub Release (the first job to finish creates it). [GoReleaser](https://goreleaser.com) builds and publishes this tool (`.goreleaser.yaml`, `mise run charter:release`); the SDK jobs add their files with `gh`:

| File | What |
|---|---|
| `charter_linux_amd64.tar.gz`, `charter_linux_arm64.tar.gz`, `charter_darwin_arm64.tar.gz`, `charter_darwin_amd64.tar.gz`, `checksums.txt` | This tool, and the SHA-256 of each archive. No Windows build: the tool manages process groups, which is Unix-only |
| `notes-ts-sdk-go.tar.gz`, `notes-ts-sdk-typescript.tar.gz` | The SDK sources Fern generates from the oRPC specs: generated fresh, then checked (`sdk:check`) |
| `notes-go-sdk-go.tar.gz`, `notes-go-sdk-typescript.tar.gz` | The same from the Go Worker's specs |
| `notes-ts-specs.tar.gz`, `notes-go-specs.tar.gz` | `openapi.json` and `asyncapi.json` of each API |
| `notes-ts-cli-linux-amd64`, `notes-go-cli-linux-amd64` | The Fern CLI of each API (the binary calls itself `notes`) |

**Go module versions.** The tool is in the repo's root module, so `vX.Y.Z` is its version. `go/` and `examples/notes-go/sdk/go/` are Go modules in subdirectories that other repos use, and Go only finds a version of such a module under a tag with the directory in front. So `release:tags` adds `go/vX.Y.Z` and `examples/notes-go/sdk/go/vX.Y.Z` on the same commit as `vX.Y.Z`. You push one tag; those two follow. Then `go get github.com/joeblew999/charter/go@latest`, `go run github.com/joeblew999/charter/cmd/charter@latest help` and `go get github.com/joeblew999/charter/examples/notes-go/sdk/go@latest` pick it up. Don't push the module tags or upload release files by hand.

**A dry run** is the same build without a version tag: GoReleaser makes a snapshot in `dist/`, `release` lists what is in `dist/` and publishes nothing, `release:tags` says which tags it would add, and the files are kept as workflow artifacts. Pull requests do that, and so does `gh workflow run release.yml`. Locally: `mise run charter:release` at the root, and `mise run sdk:dist && mise run release` in `examples/notes-go`.

**The Go SDK as a module.** `examples/notes-go/sdk/go/` is the Go SDK of the Go API, committed so that another repo can `go get github.com/joeblew999/charter/examples/notes-go/sdk/go`. `mise run sdk:publish` writes it from a fresh generation (after `mise run spec`, when the contract changed); never edit it. `mise run sdk:publish:check` fails when it is stale, and the `sdk-check` workflow runs that. How a project made by `charter new` does the same for its API: [Giving the Go SDK to another repo](guides/sdks.md#giving-the-go-sdk-to-another-repo).

Not released: the TypeScript SDK as an npm package, the Go SDKs of the oRPC API and the showcases as modules, the Fern CLI for macOS and Windows, and anything else of the two showcases ([plans/next.md](plans/next.md)).

### The workflows in another repo

```sh
go run github.com/joeblew999/charter/cmd/charter@latest workflows -into .          # writes .github/workflows/: check, deploy, sdk-check, release
go run github.com/joeblew999/charter/cmd/charter@latest workflows -into . -check   # fails if they differ from the templates
```

The templates adapt to the repo: one with an `examples/` folder gets this repo's jobs (the tool, the library, each example in its folder), any other gets a project's (its tasks at the root). `charter new` writes them into a new project.

They only call mise tasks, so the repo needs a `mise.toml` with the tasks they name: `setup`, `check`, `deploy`, `live-test`, `cloudflare:token`, `sdk:gen`, `sdk:check`, `sdk:publish:check`, `sdk:dist`, `sdk:dist:cli`, `release` and `release:tags`. A project made by `charter new` has them.

## A new project: `charter new`

One command makes a working Go API project: `examples/notes-go` under your project's name.

```sh
go run github.com/joeblew999/charter/cmd/charter@latest new -name billing-api      # into ./billing-api
cd billing-api && git init
mise install && mise run setup
mise run check                 # lint, tests, spec drift, the TinyGo build, the live and MCP tests natively and under workerd
mise run run            # natively: http://localhost:5174/api/hello
mise run deploy         # to Cloudflare, then: mise run live-test
```

| Flag | What | Default |
|---|---|---|
| `-name` | The project and its Worker: lower-case letters, digits and hyphens | required |
| `-module` | The Go module path | `github.com/<your GitHub login>/<name>`, from `gh` |
| `-subdomain` | Your Cloudflare account's workers.dev subdomain: the word before `.workers.dev` in a deployed Worker's URL | the placeholder `your-subdomain` |
| `-into` | Where to create it; the folder must be empty | `./<name>` |
| `-from` | A checkout of this repo to copy from | the tool's own version, cloned from GitHub |

- **Its first line is the tool's version and what it pinned to it:** the charter tool in the project's `mise.toml`, the Go library in its `go.mod`. `charter version` prints the version alone. Minutes after a release, `charter@latest` can still be the release before (Go's module proxy caches it); the release binary and `charter@vX.Y.Z` are exact.
- **It copies the example** from this repo at the tool's own version: every file of `examples/notes-go/` that is tracked or would be (the contract and handlers, the Worker entry, the platform files, the spec command, `mise.toml`, `package.json`, the Fern folder `fern/`, `migrations/`, `test/`), as it is. There is no separate template, no list of files and no filtering of tasks, so a new project starts from code that passed this repo's checks. One folder is left out: the committed Go SDK (`sdk/go/`), which is generated from specs that now name another Worker; `mise run sdk:publish` writes the project's own.
- **It renames:** the Worker and its D1 database (`-name`), the Go module (`-module`, and with it the Go SDK's module path `<module>/sdk/go`), and Fern's organisation (`fern/fern.config.json`). The SDK's names (`NotesClient`, the Go package `notes`, the CLI `notes`) stay the example's until you change them in `fern/generators.yml`.
- **It sets the Worker's URL** to `https://<name>.<subdomain>.workers.dev`, as the default of `API_URL` in the project's `mise.toml` and in the two copied specs, which must agree for `mise run check` to pass. Without `-subdomain` that is a placeholder, and the closing message says what to do after the first deploy: put the URL the deploy prints in `mise.local.toml` as `API_URL` (or make it the default in `mise.toml`, which CI reads too), then `mise run spec`. Nothing in a project names this repo's own subdomain.
- **Its closing message also says what the Go Worker costs to run** ([api-go.md](api-go.md#cost)), and links the guide to putting your own API in the example's place ([guides/replace-the-example.md](guides/replace-the-example.md)).
- **It makes the tool the project's own.** The example's tasks run the tool from this checkout (`go run ../../cmd/charter`). In the project they run `charter`, and `mise.toml` pins the release under `[tools]` (`"go:github.com/joeblew999/charter/cmd/charter"`), so `mise install` builds it. The Go library is a requirement in `go.mod` at the same release, so fixes arrive with `go get -u`.
- **Made with `-from <checkout>`, nothing is pinned:** the newest release can be older than the checkout. The tasks run `go run $CHARTER/cmd/charter`, where `CHARTER` in the project's `mise.toml` is the checkout; a `go.work` in the project lets Go run it from there; and `go.mod` builds against the checkout's library through a `replace` line. Remove all three once you depend on a release.
- **It writes** a README, `AGENTS.md`, a `docs/` folder with a start page, rules and the writing rules, and the GitHub workflows (`.github/workflows/`).
- **Then, with a GitHub repo:** `mise run docs:setup` and `mise run docs:pages`.

The project starts as the notes API. Change `api/contract.go` in it, run `mise run spec`, and go from there ([api-go.md](api-go.md#starting-a-go-workers-go-project-from-it)).

Before a release, the scaffold is proven by hand: `go run ./cmd/charter new -name trial -into /tmp/trial -from "$PWD"`, then that project's `mise run setup` and `mise run check`. `go test ./cmd/...` covers the copy, the renaming, the Worker's URL with and without `-subdomain`, the first line and a build.

## The docs site

`docs/` is plain Markdown, and GitHub Pages renders it as it is: its built-in Jekyll with the Just the Docs theme (sidebar, search, diagrams from ```` ```mermaid ```` blocks). There is no build step and no workflow.

```sh
mise run docs:setup      # writes docs/_config.yml, docs/_sass/custom/custom.scss, docs/writing.md and docs/llms.txt
mise run docs:pages      # REMOTE, once per repo: turns GitHub Pages on for docs/ on main
mise run docs:lint       # checks docs/ for what a program can check
mise run docs:review     # has Claude bring docs/ into line with docs/writing.md
```

- **What `docs:setup` writes is the same for every repo.** The tool fills in the repo's name, description and URLs from GitHub, so there is nothing to edit. The templates are in `cmd/charter/docs/`; `mise run charter:check` fails if a committed file differs.
- **The sidebar comes from the pages.** Each page starts with its short title, its order, and its parent if it has one:

  ```
  ---
  title: Go Worker (examples/notes-go/)
  nav_order: 4
  ---
  ```

- **`docs:lint` checks,** for every page: the front matter (and `permalink: /` on the start page, without which the site has no home page); no template braces; a link from the start page; every relative link's file and anchor; every `mise run` task against `mise.toml`; and every path in a code span that starts with a top-level folder. A link written inside code is an example and is not checked, and a path that git ignores (`sdk/out/`, a build folder) passes. Tasks and paths are not checked in `plans/`, [findings.md](findings.md) and [writing.md](writing.md).
- **`docs:review` needs the `claude` command** (Claude Code). It gives Claude the prompt in `cmd/charter/docs/review.md` with the lint's output, allowed to edit files and to run the lint and `charter:check`. Read its edits with `git diff` before committing.
- **For agents:** the site also serves `llms.txt` (https://joeblew999.github.io/charter/llms.txt): every page in sidebar order, each linked as raw Markdown. The site's renderer builds it from the pages; nothing to maintain.
- **In another repo** (any repo whose docs are in `docs/`):

  ```sh
  go run github.com/joeblew999/charter/cmd/charter@latest docs -into .
  go run github.com/joeblew999/charter/cmd/charter@latest docs-lint -into .
  gh api -X POST 'repos/{owner}/{repo}/pages' -f 'source[branch]=main' -f 'source[path]=/docs'
  ```

## Using the tool from another repo

The module path is real, so another repo can run it without copying it:

```sh
go run github.com/joeblew999/charter/cmd/charter@latest help
```

`new`, `workflows`, `docs`, `docs-lint` and `docs-review` run anywhere. Every other command expects this repo's layout, or that of a project made by `charter new` (a `go.work` at the root, `examples/notes-go/`, `sdk/`, `examples/notes-go/migrations/`).

Each release also has the tool as a binary (`charter_<os>_<arch>.tar.gz`, with checksums), for a repo that doesn't want Go only to run it.
