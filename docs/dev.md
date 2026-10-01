---
title: Tasks, workflows, releases
nav_order: 7
---

# dev/: the tool the tasks run

Every task in `../mise.toml` is one line. Anything that needs more than one line is a command of this Go program (standard library only), so the logic is real code that can be read, tested and reused, not shell inside TOML.

```sh
go run ./dev help            # every command
mise run <task>              # what you normally type: each task is one line that calls a tool or a dev command
```

| Command | What it does | Task that calls it |
|---|---|---|
| `with-server` | Starts a server, waits until a URL answers, runs commands against it, stops it | `api-go:test:native`, `api-go:test:workerd` |
| `migrate-local` | Applies `migrations/*.sql` to a running dev server's local D1, each once (cf can't) | `api:migrate:local`, `api-go:migrate:local` |
| `migrate` | Finds the Worker's D1 database and applies pending migrations | `api:migrate`, `api-go:migrate`, the deploy tasks |
| `size` | Fails if a file is over a gzipped size (the Wasm limit) | `api-go:build` |
| `sdk-gen`, `sdk-check`, `sdk-ready`, `sdk-list`, `sdk-clean` | Fern: generate an SDK, prove it works, make what the tests need | `sdk:*` |
| `cli-build` | Builds the Rust CLI that Fern generates, natively or for Linux in Docker | `sdk:cli:build` |
| `harness-sync`, `harness-test`, `harness-deploy` | Fern's TypeScript SDK inside a Worker (`sdk/harness`). `harness-sync` copies the SDK in, generating it again when a showcase spec is newer than it | `sdk:harness:*`, `showcase:typecheck` |
| `bench` | Times the read routes of a notes API as a client sees them ([benchmarks.md](benchmarks.md)) | `api:bench`, `api-go:bench` |
| `new` | Creates a new Go API project from this repo's example ([a new project](#a-new-project-dev-new)) | none: run it with `go run ...dev@latest new` |
| `docs` | Writes the docs site's config into a repo's `docs/` ([the docs site](#the-docs-site)) | `docs:setup` |
| `upstream` | Lists every `Upstream: owner/repo#n` tag in the code with the issue's state | `upstream:status` |
| `doctor` | Checks the tools and installs the tasks need | `doctor` |
| `cloudflare-spec` | Slices Cloudflare products out of Forge's spec as a Fern API | `sdk:cloudflare` |
| `workflows` | Writes the GitHub workflows from the templates in `dev/workflows/`; `-check` fails if a committed one differs | `dev:workflows`, `dev:check` |
| `dist-dev`, `dist-sdk`, `dist-cli` | Build what a release ships into `dist/`: this tool's binaries, the SDK sources and specs, the Fern CLI | `dev:dist`, `sdk:dist`, `sdk:dist:cli` |
| `release`, `release-tags` | Attach `dist/` to the tag's GitHub Release; tag the Go modules. Without a version tag both are a dry run | `release`, `release:tags` |
| `need-env` | Fails, naming them, unless the given environment variables are set | `cloudflare:token` |

## Adding a task

1. If it's one command, write the one-line task in `mise.toml`.
2. If it isn't, add a command here (register it in an `init`, as the others do) and write a one-line task that calls it.

## GitHub workflows

The workflows are templates in `dev/workflows/`, compiled into this tool. `mise run dev:workflows` writes them to `.github/workflows/`, and `mise run dev:check` fails if a committed one differs. Edit the template, never the copy.

Every step that does work is `mise run <task>`, so a failing step is one line you can run locally. A test (`dev/workflows_test.go`) holds the templates to that, and to exact versions of the actions and runners. The prefix says what a workflow is for: `api-`, `sdk-`, or `dev-` (this tool).

| Workflow | When | What it runs | Secrets |
|---|---|---|---|
| `api-check` | push to main, pull requests | `api:check` and `api-go:check`, one job each | none |
| `sdk-check` | push to main, pull requests | `sdk:demo`; `showcase:check` and `sdk:harness:test`; and `sdk:gen` + `sdk:check` for the Go and TypeScript SDKs of `api`, `api-go` and `showcase` | none |
| `dev-check` | push to main, pull requests | `dev:check` | none |
| `api-deploy` | by hand only (pick `api` or `api-go`) | `cloudflare:token`, `<api>:deploy`, then `<api>:live-test` against the Worker it just deployed | `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` |
| `dev-release` | a version tag. A dry run by hand, and on pull requests that touch `dev/` | `dev:dist`, `release`, `release:tags` | none (the workflow's own token) |
| `sdk-release` | a version tag. A dry run by hand, and on pull requests that touch `sdk/` or `dev/` | `sdk:dist`, `sdk:dist:cli <api>`, `release` | none |

`api-deploy` fails at its first step, naming the secrets, when they are missing. Set them once per repo with `mise run cloudflare:secrets`: it copies both values from fnox (the keychain) into the repo's GitHub secrets, without printing them.

### Cutting a release

```sh
git tag v0.1.0 && git push origin v0.1.0        # on the commit to release, once its checks are green
```

The tag must be a semantic version: `v1.2.3`, or `v1.2.3-rc.1` for a pre-release. The two release workflows build everything again from that commit and attach it to the GitHub Release `v0.1.0` (the first job to finish creates it):

| File | What |
|---|---|
| `dev-linux-amd64`, `dev-linux-arm64`, `dev-darwin-arm64`, `dev-darwin-amd64` | This tool |
| `api-sdk-go.tar.gz`, `api-sdk-typescript.tar.gz` | The SDK sources Fern generates from the oRPC specs: generated fresh, then checked (`sdk:check`) |
| `api-go-sdk-go.tar.gz`, `api-go-sdk-typescript.tar.gz` | The same from the Go Worker's specs |
| `api-specs.tar.gz`, `api-go-specs.tar.gz` | `openapi.json` and `asyncapi.json` of each API |
| `api-cli-linux-amd64`, `api-go-cli-linux-amd64` | The Fern CLI of each API (the binary calls itself `orpc-api`). About 3 minutes each on the runner |

**Go module versions.** `api-go/` and `dev/` are Go modules in subdirectories, and Go only finds a version of such a module under a tag with the directory in front. So `release:tags` adds `api-go/v0.1.0` and `dev/v0.1.0` on the same commit as `v0.1.0`. You push one tag; those two follow. Then `go get github.com/joeblew999/orpc-api/api-go@v0.1.0` and `go run github.com/joeblew999/orpc-api/dev@v0.1.0 help` work.

**A dry run** is the same build without a version tag: `release` lists what is in `dist/` and publishes nothing, `release:tags` says which tags it would add, and the files are kept as workflow artifacts. Pull requests do that, and so does `gh workflow run dev-release.yml`. Locally: `mise run dev:dist && mise run sdk:dist && mise run release`.

Not released yet: the SDKs as packages (npm, a Go module of their own) and the CLI for macOS and Windows. See `docs/plans/next.md`.

### The workflows in another repo

```sh
go run github.com/joeblew999/orpc-api/dev@latest workflows -into .          # writes .github/workflows/api-*.yml and sdk-*.yml
go run github.com/joeblew999/orpc-api/dev@latest workflows -into . -check   # fails if they differ from the templates
```

It writes the `api-` and `sdk-` workflows. `-only api` or `-only sdk` writes one set; the `dev-` ones are only written where there is a `dev/` module, as here. The templates adapt to the repo: one without an `api/` folder (a project made by `dev new`) gets the Go API's jobs only.

They only call mise tasks, so the repo needs a `mise.toml` with the tasks they name: `setup`, `api:check`, `api-go:check`, `api:deploy`, `api-go:deploy`, `cloudflare:token`, `sdk:demo`, `showcase:check`, `sdk:harness:test`, `sdk:gen`, `sdk:check`, `sdk:dist`, `sdk:dist:cli` and `release`. Copy them from this repo's `mise.toml`. A project with one API deletes the other API's job from the copy.

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

- **It pulls this repo from GitHub at the tool's own version** and copies the example: `api-go/` (contract, handlers, Worker entry, hub, platform files, spec command), `migrations/`, `test/`, the Fern folder `sdk/fern/apis/api-go/`. There is no separate template, so a new project starts from code that passed this repo's checks.
- **It renames:** the Worker and its D1 database (`-name`), the Go module (`-module`, default `github.com/<your GitHub login>/<name>`), and the SDK's names (`billing-api` gives `BillingApiClient`).
- **It keeps as imports** the reusable packages (`humaworkers`, `asyncapi`, `follow`, `humamcp`), pinned to the same version, so fixes arrive with `go get -u`.
- **It writes** a `mise.toml` with the Go and SDK tasks (the dev tool pinned by version, not copied), `go.work`, a README, and a `docs/` folder with a start page and rules.
- **Then, with a GitHub repo:** `mise run dev:workflows` (the workflows come out for one Go API), `mise run docs:setup` and `mise run docs:pages`.

The project starts as the notes API. Change `api-go/api/contract.go`, run `mise run api-go:spec`, and go from there.

Before a release, the scaffold is proven by hand: `go run ./dev new -name trial -into /tmp/trial -from .`, then that project's `mise run setup` and `mise run check` (`go test ./dev` covers the copy, the renaming and a build).

## The docs site

`docs/` is plain Markdown, and GitHub Pages renders it as it is: its built-in Jekyll with the Just the Docs theme (sidebar, search, diagrams from ```` ```mermaid ```` blocks). There is no build step and no workflow.

```sh
mise run docs:setup      # writes docs/_config.yml and docs/_sass/custom/custom.scss (go run ./dev docs)
mise run docs:pages      # once per repo: turns GitHub Pages on for docs/ on the default branch
```

- **For agents:** the site also serves `llms.txt` (https://joeblew999.github.io/orpc-api/llms.txt): every page in sidebar order, each linked as raw Markdown. The site's renderer builds it from the pages; nothing to maintain.
- **The config is the same for every repo.** `dev docs` fills in the repo's name, description and URLs from GitHub, so there is nothing to edit. `mise run dev:check` fails if the committed files differ.
- **The sidebar comes from the pages.** Each page starts with its short title, its order, and its parent if it has one:

  ```
  ---
  title: Go Worker (api-go/)
  nav_order: 4
  ---
  ```

- **In another repo** (any repo whose docs are in `docs/`):

  ```sh
  go run github.com/joeblew999/orpc-api/dev@latest docs -into .
  gh api -X POST 'repos/{owner}/{repo}/pages' -f 'source[branch]=main' -f 'source[path]=/docs'
  ```

## Using it from another repo

The module path is real, so another repo can run it without copying it:

```sh
go run github.com/joeblew999/orpc-api/dev@latest help
```

Every command but `workflows` expects this repo's layout (`api/`, `api-go/`, `sdk/`, `migrations/`, a `go.work` at the root).

Each release also has the tool as a binary (`dev-<os>-<arch>`), for a repo that doesn't want Go only to run it.
