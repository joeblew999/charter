---
title: How to help
nav_order: 14
---

# How to help

Report something, make it faster, or pick up an issue. [The rules](rules.md) are short and binding; what to run, and when, is [What to do](../charter/README.md). In this repo charter is this checkout's: `go tool charter <command>`.

## Set up

```sh
git clone https://github.com/joeblew999/charter && cd charter
mise install              # the tools: go, node, gh, fnox, TinyGo, workerd…
gh auth login             # the repo job reads GitHub
npm ci                    # the packages; builds charter-lib (ts/)
go tool charter check     # the packages, repo and projects jobs (Docker for the conformance projects)
go tool charter ci        # every job GitHub runs, the clients in every language among them (Docker)
```

Docker: charter starts it where it is installed but not running and may be started (a Claude cloud session), and otherwise says to start it. Where the internet is a proxy on the machine itself with its own CA (a Claude cloud session), Fern's generators run their containers through `internal/fern/cloud/docker`, which gives them the host's network, the proxy and its CA (`internal/fern/docker.go`); on a Mac or a GitHub runner the containers are as Docker makes them.

## What is where

| Folder | What it is |
|---|---|
| The top (`go.mod`) | The Go library, one package per folder (`auth/`, `serve/`…), with its Worker glue in `worker/`; and the command, `charter`, in `cmd/charter/`, its steps in `internal/cli/`: a project's (`internal/cli/projectsteps`) and a repo's (`internal/cli/repo`, with the files charter writes into every repo in `internal/cli/repo/files/`) |
| `ts/` | `charter-lib`, what a TypeScript Worker imports |
| `examples/start-ts/`, `examples/notes-ts/` | The TypeScript examples, which `charter new` copies: one route, and the notes API (sign-in, streams, rate limits) |
| `examples/start-go/`, `examples/notes-go/` | The same two APIs in Go, held to the TypeScript ones by `twins` |
| `examples/start-htmx/`, `examples/start-datastar/` | Go with server-rendered pages ([Server-rendered pages](pages.md)) |
| `examples/conformance-ts/`, `examples/conformance-go/` | Every Fern feature, its SDKs tested against the API, inside a Worker too; the TypeScript one is the reference |

The top's `charter.toml` says how the repo is kept: its GitHub settings and its jobs. The root `package.json` is the npm workspace of `ts/` and the projects in `examples/`; the root `go.work` is the Go workspace of the library and the Go projects; mise installs the tools (`mise.toml`).

## What is generated

Never edit these: change the source, then run the command.

| Path | Written by |
|---|---|
| `fern/openapi.json`, `fern/asyncapi.json`, `fern/generators.yml` | `charter spec`, from the contract; Fern's settings from the spec and `charter.toml` (`[sdks]`) |
| `api/queries.go`, `api/tables.ts` | `charter spec`, from `migrations/*.sql` (and in Go `api/queries.sql`), as `db` in `charter.toml` says |
| `pages/*.x.go` (committed) | gsx, from the components; `charter check` writes them and fails while they were stale |
| `sdk/go/` (committed) | `charter sdk -publish` |
| `sdk/out/`, `build/`, `ts/dist/` | `charter sdk`; the Worker's build (`charter build`: TinyGo's, or the TypeScript bundle) and the program that writes a project's specs (`build/_spec`, `build/_spec.mjs`: `charter spec`); `npm ci` |
| `examples/conformance-ts/client/` | The SDK the conformance Worker runs, copied in by `check`, `dev` and `deploy` |
| `dist/` | `charter release`: charter-lib packed in `dist/lib/`, the binaries in `dist/charter/` (goreleaser) |
| `docs/charter/`, `AGENTS.md`, `CLAUDE.md`, `.github/` (forms, labels, workflows) | `charter init`, from `internal/cli/repo/files/`, charter's commands and the top's `charter.toml` |
| A project's `.mise/tasks/`, `.claude/skills/charter/`, `.config/mise/conf.d/charter.toml` | `charter tasks` and `charter skill` |
| `package-lock.json` | `npm install` |

## Report a bug

Use the GitHub forms; for a bug in Fern, TinyGo, workers-go, Huma or oRPC, check [Upstream issues](upstream.md) first. An agent prints the same form:

```sh
go tool charter issues bug   # or feature, upstream, plan: the body, and the gh command that files it
```

## Make it faster

The target: a Go Worker costs no more CPU per request than its TypeScript twin. What was tried and measured is in [Benchmarks](benchmarks.md). Issues: [`perf`](https://github.com/joeblew999/charter/labels/perf).

## Pick up an issue

The plan is the [open issues](https://github.com/joeblew999/charter/issues). Before a pull request: `go tool charter check` passes, and `docs/` says what changed.

## A change to charter's repo files

What `charter init` writes into every repo is in `internal/cli/repo/files/`, the same in every repo; the tests beside it (`internal/cli/repo/*_test.go`) run `init`, `docs` and the rest in throwaway repos. After changing one, `go tool charter init` writes it into this repo too, and `charter check` fails until it does.

## The CI

The top's `charter.toml` has this repo's own jobs, `[ci.jobs.<name>]`; `charter init` writes a workflow for each (`.github/workflows/self-<job>.yml`), and `charter-repo.yml` is charter's `repo` job, the same in every repo. Every workflow calls the job runner (`charter-ci.yml`): mise installs the tools, npm the packages, then charter (this checkout's, built) runs the job as `go tool charter ci <job>` runs it here. One job in one project: `go tool charter ci clients examples/conformance-go`.

Every command says what it made and how long it took: each Worker its size, gzipped, against its limit; each program of a second or more, each server's start, each job. `check` ends with both.

| Job | When | What |
|---|---|---|
| `repo` | Push, pull request | charter's job: charter's files and the workflows (`init -check`), the docs, the repo on GitHub, the upstream issues |
| `packages` | Push, pull request, tag; Linux and macOS | charter-lib with its tests, the Go library and charter with theirs (the projects kept alike among them: `examples/examples_test.go`), the release's config, the workflows linted |
| `projects` | Push, pull request, tag; one runner per project | Every project's check |
| `twins` | Push, pull request, tag | Each API that is here in TypeScript and in Go, both under workerd, the same requests to both: no answer may differ, nor their saved specs and migrations ([The standard API](standard.md#held-the-same)) |
| `new` | Push, pull request, tag | A project made by `charter new` from each example, in both languages, with charter from mise and charter-lib packed, installed, and its check passing (`examples/new-projects.sh`) |
| `go-app` | Push, pull request, tag | A Go app made by `charter init` gets charter's TinyGo through mise, every way it builds (`examples/new-app.sh`) |
| `clients` | Push, pull request, tag; one runner per conformance project | Every client its `fern/generators.yml` declares, generated and proven in its language |
| `deploy` | By hand: `go tool charter ci deploy <project> -remote` | A project's deploy, then its live tests |

## Where the tests are

Each test checks one thing, at one level, and the same level is in the same place in Go and in TypeScript:

| # | Level | Who needs it | Where | When it runs |
|---|---|---|---|---|
| 1 | **The standard:** a running API serves its specs, lists its MCP tools, leads a client to sign in, refuses with problem details | Every API | Built into charter (`internal/standard`): it reads the API's own spec, so nothing is written or copied | Every project's `check` and `live-test`, first |
| 2 | **The API's own:** it does what it is for | Every API | The project's `test/` (`test/live.ts`, `test/sdk.ts`…); a Go twin runs its TypeScript reference's, as its `charter.toml`'s `[test]` says | The project's `check` and `live-test` |
| 3 | **Unit tests:** a handler on its own | Where wanted | Beside the code: `api/*_test.go`, `api/*.test.ts` | The project's `check` |
| 4 | **Twins:** the Go project answers as its TypeScript reference | An API in both languages | Built into charter (`charter twins`, `internal/twins`) | The `twins` job |
| 5 | **The libraries and charter** | charter's developers | Beside the code: `*/*_test.go` at the top, `ts/src/*.test.ts` | The `packages` job |
| 6 | **This repo only:** the projects are alike; a project made from the release works | charter's developers | `examples/examples_test.go`, `examples/new-projects.sh` | The `packages` and `new` jobs |

## Cut a release

```sh
go tool charter release X.Y.Z -dry-run   # every check, charter-lib into dist/lib, the binaries into dist/charter
go tool charter release X.Y.Z            # the version, the tag, the GitHub Release with charter-lib and the binaries
```

It runs `charter check` before it tags, and never waits for GitHub. It tags `vX.Y.Z` (the Go library's, the root module, which `go get` pins; above `v0.19.1`, the last tag of the Go tool the root held before charter-ts), and the Release `vX.Y.Z`, marked latest, carries the `charter` binary for every machine and `charter-lib-X.Y.Z.tgz` (goreleaser, `.goreleaser.yaml`): a repo takes charter with `mise use github:joeblew999/charter`, and charter-lib by its URL in `package.json`. It puts that version into every install command the docs show.
