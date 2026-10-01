---
title: The dev tool, workflows, releases
nav_order: 8
parent: This repository
---
# dev/: the tool the tasks run

Every task in `mise.toml` is one line. Anything that needs more than one line is a command of the `dev` tool: a Go program in `dev/` (standard library only), so the logic is real code that can be read, tested and reused, not shell inside TOML. Read this page to add a task, to change a GitHub workflow, to cut a release, to start a new project, or to set up the docs site.

```sh
mise tasks                   # every task, with what it does
mise run <task>              # what you normally type: each task is one line that calls a tool or a dev command
go run ./dev help            # every dev command, with its flags
mise run dev:check           # the tool's own checks: gofmt, vet, tests, and the workflows and docs config match their templates
```

`mise run dev:check` asks GitHub for the repo's name (`gh repo view`), so it needs `gh` logged in.

## The commands

| Command | What it does | Task that calls it |
|---|---|---|
| `with-server` | Starts a server, waits up to 90 s until a URL answers, runs commands against it, stops it. `{port}` in its arguments is a free port, `{port2}` another. A command's output is shown only if it fails; `-show` streams it | `api-go:test:native`, `api-go:test:workerd`, `showcase-go:test:native`, `showcase-go:test:workerd` |
| `migrate-local` | Applies `migrations/*.sql` to a running dev server's local D1, each once (cf can't) | `api:migrate:local`, `api-go:migrate:local`, `api-go:test:workerd` |
| `migrate` | Finds the Worker's D1 database (`<worker>-db`) and applies pending migrations | `api:migrate`, `api-go:migrate`, the two notes `deploy` tasks |
| `size` | Fails if a file is over a gzipped size (the Wasm limit) | `api-go:build`, `showcase-go:build` |
| `sdk-gen`, `sdk-check`, `sdk-ready`, `sdk-list`, `sdk-clean` | Fern: generate an SDK, prove it works, make what the tests need | `sdk:gen`, `sdk:check`, `sdk:ready`, `sdk:list`, `sdk:clean`, `sdk:demo` |
| `cli-build` | Builds the Fern CLI (Rust), natively or with `-linux` for Linux in Docker | `sdk:cli:build` |
| `harness-sync`, `harness-test`, `harness-deploy` | Fern's TypeScript SDK inside the harness Worker (`sdk/harness/`). `harness-sync` copies the SDK in, generating it again when a showcase spec is newer than it | `showcase:typecheck`, `sdk:harness:test`, `sdk:harness:deploy` |
| `bench` | Times the read routes of a notes API as a client sees them; `-n` requests per route ([benchmarks.md](benchmarks.md)) | `api:bench`, `api-go:bench` |
| `new` | Creates a new Go API project from this repo's example ([below](#a-new-project-dev-new)) | none: run it with `go run ...dev@latest new` |
| `version` | Prints the release the tool is: what `new` pins a project to | none |
| `docs` | Writes the docs site's config, `docs/writing.md` and `docs/llms.txt` into a repo; `-check` fails if one differs ([below](#the-docs-site)) | `docs:setup`, `dev:check` |
| `docs-lint` | Checks `docs/` for what a program can check | `docs:lint` |
| `docs-review` | Hands Claude the review prompt with what `docs-lint` found; `-print` only shows the prompt | `docs:review` |
| `upstream` | Lists every `Upstream: owner/repo#n` tag in the code with the issue's state | `upstream:status` |
| `doctor` | Checks the tools and installs the tasks need, and lists the APIs | `doctor` |
| `cloudflare-spec` | Slices Cloudflare products out of Forge's spec as a Fern API; `-products`, `-release` | `sdk:cloudflare` |
| `workflows` | Writes the GitHub workflows from the templates in `dev/workflows/`; `-check` fails if a committed one differs | `dev:workflows`, `dev:check` |
| `dist-dev`, `dist-sdk`, `dist-cli` | Build what a release ships into `dist/`: this tool's binaries, the SDK sources and specs, the Fern CLI | `dev:dist`, `sdk:dist`, `sdk:dist:cli` |
| `release`, `release-tags` | Attach `dist/` to the tag's GitHub Release; tag the Go modules. Without a version tag both are a dry run | `release`, `release:tags` |
| `need-env` | Fails, naming them, unless the given environment variables are set | `cloudflare:token` |
| `github-secrets` | Copies the given environment variables into the repo's GitHub Actions secrets, without printing them | `cloudflare:secrets` |

A command runs from the repo's root, which it finds by the `go.work` above where it was started. Six also run outside this repo's layout, in any repo: `new`, `version`, `workflows`, `docs`, `docs-lint` and `docs-review`.

## Adding a task

1. If it's one command, write the one-line task in `mise.toml`. A task is plain `sh`: on Linux mise runs dash, so no bashisms.
2. If it isn't, add a command in `dev/` (register it in an `init`, as the others do) and write a one-line task that calls it.

## GitHub workflows

The workflows are templates in `dev/workflows/`, compiled into the tool. `mise run dev:workflows` writes them to `.github/workflows/`, and `mise run dev:check` fails if a committed one differs. Edit the template, never the copy.

Every step that does work is `mise run <task>`, so a failing step is one line you can run locally. A test (`dev/workflows_test.go`) holds the templates to that, and to exact versions of the actions and runners. The prefix says what a workflow is for: `api-`, `sdk-`, or `dev-` (this tool).

| Workflow | When | What it runs | Secrets |
|---|---|---|---|
| `api-check` | push to main, pull requests, by hand | `api:check`, `api-go:check` and `showcase-go:check`, one job each | none |
| `sdk-check` | push to main, pull requests, by hand | `sdk:demo`; `showcase:check` and `sdk:harness:test`; and `sdk:gen` + `sdk:check` for the Go and TypeScript SDKs of `api`, `api-go`, `showcase` and `showcase-go` | none |
| `dev-check` | push to main, pull requests, by hand | `dev:check` | none |
| `api-deploy` | by hand only (pick `api` or `api-go`) | `cloudflare:token`, `<api>:deploy`, then `<api>:live-test` against the Worker it just deployed | `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` |
| `dev-release` | a version tag. A dry run by hand, and on pull requests that touch `dev/`, `.goreleaser.yaml`, `mise.toml` or `go.work` | `dev:release` (GoReleaser), `release:tags` | none (the workflow's own token) |
| `sdk-release` | a version tag. A dry run by hand, and on pull requests that touch `sdk/`, `dev/`, `mise.toml`, `go.work` or `rust-toolchain.toml` | `sdk:dist`, `sdk:dist:cli <api>`, `release` | none |

`api-deploy` fails at its first step, naming the secrets, when they are missing. Set them once per repo with `mise run cloudflare:secrets`: it copies both values from fnox (the keychain) into the repo's GitHub secrets, without printing them. Whether this repo's secrets are set, and whether `api-deploy` has run, was not checked for this page; the recorded deploys were made from a machine ([findings.md](findings.md)). The Go showcase's Worker is not among the workflow's choices: deploy it with `mise run showcase-go:deploy`.

### Getting the latest release

Nothing in these docs names a version, so nothing here goes stale: every link below follows the newest release by itself. The current one: [![latest release](https://img.shields.io/github/v/release/joeblew999/orpc-api)](https://github.com/joeblew999/orpc-api/releases/latest)

```sh
go run github.com/joeblew999/orpc-api/dev@latest help                 # the tool, no install (needs Go)
go get github.com/joeblew999/orpc-api/api-go@latest                   # the Go packages
curl -fsSL https://github.com/joeblew999/orpc-api/releases/latest/download/dev_darwin_arm64.tar.gz | tar xz dev   # the tool as a binary
```

As a mise tool in any project (`dev` is then on the path of that project's tasks):

```toml
[tools]
"go:github.com/joeblew999/orpc-api/dev" = "latest"                                   # built with Go
# or a prebuilt binary, no Go needed:
"ubi:joeblew999/orpc-api" = { version = "latest", exe = "dev", matching = "dev_" }
```

A project made by `dev new` has the first line already, with the release it was made from; `mise up` moves it on.

- **The release page:** https://github.com/joeblew999/orpc-api/releases/latest
- **Any file of the latest release:** `https://github.com/joeblew999/orpc-api/releases/latest/download/<file>`, with the file names from the table below (they carry no version for this reason).
- **`mise run docs:lint` fails on a hard-coded release version** in a page (`@v1.2.3`, a `releases/tag/` or `releases/download/v...` link). Use `@latest`, `releases/latest`, or the placeholder `vX.Y.Z`. Findings and plans may name versions: they record what was.

### Cutting a release

```sh
git tag vX.Y.Z && git push origin vX.Y.Z        # on the commit to release, once its checks are green
```

The tag must be a semantic version: `v1.2.3`, or `v1.2.3-rc.1` for a pre-release. The two release workflows build everything again from that commit and put it on the tag's GitHub Release (the first job to finish creates it). [GoReleaser](https://goreleaser.com) builds and publishes this tool (`.goreleaser.yaml`, `mise run dev:release`); the SDK jobs add their files with `gh`:

| File | What |
|---|---|
| `dev_linux_amd64.tar.gz`, `dev_linux_arm64.tar.gz`, `dev_darwin_arm64.tar.gz`, `dev_darwin_amd64.tar.gz`, `checksums.txt` | This tool, and the SHA-256 of each archive. No Windows build: the tool manages process groups, which is Unix-only |
| `api-sdk-go.tar.gz`, `api-sdk-typescript.tar.gz` | The SDK sources Fern generates from the oRPC specs: generated fresh, then checked (`sdk:check`) |
| `api-go-sdk-go.tar.gz`, `api-go-sdk-typescript.tar.gz` | The same from the Go Worker's specs |
| `api-specs.tar.gz`, `api-go-specs.tar.gz` | `openapi.json` and `asyncapi.json` of each API |
| `api-cli-linux-amd64`, `api-go-cli-linux-amd64` | The Fern CLI of each API (the binary calls itself `orpc-api`) |

**Go module versions.** `api-go/` and `dev/` are Go modules in subdirectories, and Go only finds a version of such a module under a tag with the directory in front. So `release:tags` adds `api-go/vX.Y.Z` and `dev/vX.Y.Z` on the same commit as `vX.Y.Z`. You push one tag; those two follow. Then `go get github.com/joeblew999/orpc-api/api-go@latest` and `go run github.com/joeblew999/orpc-api/dev@latest help` pick it up. Don't push the module tags or upload release files by hand.

**A dry run** is the same build without a version tag: GoReleaser makes a snapshot in `dist/`, `release` lists what is in `dist/` and publishes nothing, `release:tags` says which tags it would add, and the files are kept as workflow artifacts. Pull requests do that, and so does `gh workflow run dev-release.yml`. Locally: `mise run dev:release && mise run sdk:dist && mise run release`.

Not released: the SDKs as packages (npm, a Go module of their own), the Fern CLI for macOS and Windows, and anything of the two showcases ([plans/next.md](plans/next.md)).

### The workflows in another repo

```sh
go run github.com/joeblew999/orpc-api/dev@latest workflows -into .          # writes .github/workflows/api-*.yml and sdk-*.yml
go run github.com/joeblew999/orpc-api/dev@latest workflows -into . -check   # fails if they differ from the templates
```

It writes the `api-` and `sdk-` workflows. `-only api` or `-only sdk` writes one set; the `dev-` ones are only written where there is a `dev/` module, as here. The templates adapt to the repo: one without an `api/` folder (a project made by `dev new`) gets the Go API's jobs only.

They only call mise tasks, so the repo needs a `mise.toml` with the tasks they name: `setup`, `api:check`, `api-go:check`, `showcase-go:check`, `api:deploy`, `api-go:deploy`, `api:live-test`, `api-go:live-test`, `cloudflare:token`, `sdk:demo`, `showcase:check`, `sdk:harness:test`, `sdk:gen`, `sdk:check`, `sdk:dist`, `sdk:dist:cli` and `release`. Copy them from this repo's `mise.toml`. A project with one API deletes the other API's job from the copy.

## A new project: `dev new`

One command makes a working Go API project: the Go half of this repo under your project's name.

```sh
go run github.com/joeblew999/orpc-api/dev@latest new -name billing-api      # into ./billing-api
cd billing-api && git init
mise install && mise run setup
mise run check                 # lint, tests, spec drift, the TinyGo build, the live and MCP tests natively and under workerd
mise run api-go:run            # natively: http://localhost:5174/api/hello
mise run api-go:deploy         # to Cloudflare, then: mise run api-go:live-test
```

| Flag | What | Default |
|---|---|---|
| `-name` | The project and its Worker: lower-case letters, digits and hyphens | required |
| `-module` | The Go module path | `github.com/<your GitHub login>/<name>`, from `gh` |
| `-subdomain` | Your Cloudflare account's workers.dev subdomain: the word before `.workers.dev` in a deployed Worker's URL | the placeholder `your-subdomain` |
| `-into` | Where to create it; the folder must be empty | `./<name>` |
| `-from` | A checkout of this repo to copy from | the tool's own version, cloned from GitHub |

- **Its first line is the tool's version and what it pinned to it:** the dev tool in the project's `mise.toml`, the Go packages in `api-go/go.mod`. `dev version` prints the version alone. Minutes after a release, `dev@latest` can still be the release before (Go's module proxy caches it); the release binary and `dev@vX.Y.Z` are exact.
- **It copies the example** from this repo at the tool's own version: the notes API in `api-go/` (contract, handlers, Worker entry, hub, platform files, spec command), `migrations/`, the notes test programs from `test/`, and the Fern folder `sdk/fern/apis/api-go/`. There is no separate template, so a new project starts from code that passed this repo's checks. The Go showcase and the test that compares with the oRPC contract are left out.
- **It renames:** the Worker and its D1 database (`-name`), the Go module (`-module`), and the SDK's names (`billing-api` gives `BillingApiClient`).
- **It sets the Worker's URL** to `https://<name>.<subdomain>.workers.dev`, as the default of `API_GO_URL` in the project's `mise.toml` and in the two copied specs, which must agree for `mise run check` to pass. Without `-subdomain` that is a placeholder, and the closing message says what to do after the first deploy: put the URL the deploy prints in `mise.local.toml` as `API_GO_URL` (or make it the default in `mise.toml`, which CI reads too), then `mise run api-go:spec`. Nothing in a project names this repo's own subdomain.
- **Its closing message also says what the Go Worker costs to run** ([api-go.md](api-go.md#cost)), and links the guide to putting your own API in the example's place ([guides/replace-the-example.md](guides/replace-the-example.md)).
- **It keeps as imports** the reusable packages (`humaworkers`, `asyncapi`, `follow`, `humamcp`, `transport`, `specfile`), pinned to the same version, so fixes arrive with `go get -u`. Made with `-from`, the project builds against that checkout through a `replace` line in its `go.mod`; remove it once you depend on a release.
- **It writes** a `mise.toml` with the Go and SDK tasks (the dev tool is one of its mise tools, pinned to the release, so tasks call `dev <command>`), `go.work`, a README, `AGENTS.md`, and a `docs/` folder with a start page, rules and the writing rules.
- **Then, with a GitHub repo:** `mise run dev:workflows` (the workflows come out for one Go API), `mise run docs:setup` and `mise run docs:pages`.

The project starts as the notes API. Change `api-go/api/contract.go`, run `mise run api-go:spec`, and go from there ([api-go.md](api-go.md#starting-a-go-workers-go-project-from-it)).

Before a release, the scaffold is proven by hand: `go run ./dev new -name trial -into /tmp/trial -from .`, then that project's `mise run setup` and `mise run check`. `go test ./dev` covers the copy, the renaming, the Worker's URL with and without `-subdomain`, the first line and a build.

## The docs site

`docs/` is plain Markdown, and GitHub Pages renders it as it is: its built-in Jekyll with the Just the Docs theme (sidebar, search, diagrams from ```` ```mermaid ```` blocks). There is no build step and no workflow.

```sh
mise run docs:setup      # writes docs/_config.yml, docs/_sass/custom/custom.scss, docs/writing.md and docs/llms.txt
mise run docs:pages      # REMOTE, once per repo: turns GitHub Pages on for docs/ on main
mise run docs:lint       # checks docs/ for what a program can check
mise run docs:review     # has Claude bring docs/ into line with docs/writing.md
```

- **What `docs:setup` writes is the same for every repo.** The tool fills in the repo's name, description and URLs from GitHub, so there is nothing to edit. The templates are in `dev/docs/`; `mise run dev:check` fails if a committed file differs.
- **The sidebar comes from the pages.** Each page starts with its short title, its order, and its parent if it has one:

  ```
  ---
  title: Go Worker (api-go/)
  nav_order: 4
  ---
  ```

- **`docs:lint` checks,** for every page: the front matter (and `permalink: /` on the start page, without which the site has no home page); no template braces; a link from the start page; every relative link's file and anchor; every `mise run` task against `mise.toml`; and every path in a code span that starts with a top-level folder. A link written inside code is an example and is not checked, and a path that git ignores (`sdk/out/`, a build folder) passes. Tasks and paths are not checked in `plans/`, [findings.md](findings.md) and [writing.md](writing.md).
- **`docs:review` needs the `claude` command** (Claude Code). It gives Claude the prompt in `dev/docs/review.md` with the lint's output, allowed to edit files and to run the lint and `dev:check`. Read its edits with `git diff` before committing.
- **For agents:** the site also serves `llms.txt` (https://joeblew999.github.io/orpc-api/llms.txt): every page in sidebar order, each linked as raw Markdown. The site's renderer builds it from the pages; nothing to maintain.
- **In another repo** (any repo whose docs are in `docs/`):

  ```sh
  go run github.com/joeblew999/orpc-api/dev@latest docs -into .
  go run github.com/joeblew999/orpc-api/dev@latest docs-lint -into .
  gh api -X POST 'repos/{owner}/{repo}/pages' -f 'source[branch]=main' -f 'source[path]=/docs'
  ```

## Using the tool from another repo

The module path is real, so another repo can run it without copying it:

```sh
go run github.com/joeblew999/orpc-api/dev@latest help
```

`new`, `workflows`, `docs`, `docs-lint` and `docs-review` run anywhere. Every other command expects this repo's layout, or that of a project made by `dev new` (a `go.work` at the root, `api-go/`, `sdk/`, `migrations/`).

Each release also has the tool as a binary (`dev-<os>-<arch>`), for a repo that doesn't want Go only to run it.
