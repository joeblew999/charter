---
title: How to help
nav_order: 14
---

# How to help

Report something, make it faster, or pick up an issue. [The rules](rules.md) are short and binding; what to run, and when, is [What to do](../charter/README.md).

## Set up

```sh
git clone https://github.com/joeblew999/charter && cd charter
mise install    # the tools: node, go, gh, fnox
gh auth login   # the repo job reads GitHub
npm ci          # every package; builds charter-ts and charter-lib
npm run check   # the packages, repo and projects jobs (Docker for the conformance projects)
npm run ci      # every job GitHub runs, the clients in every language among them (Docker)
```

## What is where

| Folder | What it is |
|---|---|
| `charter-ts/` | The tool: its commands are in `charter-ts/src/commands.ts`. The files it writes into every repo are this repo's own, listed in `charter-ts/src/charter-files.ts` |
| `ts/lib/` | `charter-lib`, what a TypeScript Worker imports |
| `go/lib/` | The Go library, with its Worker glue in `go/lib/worker/` |
| `ts/examples-start/`, `ts/examples-notes/` | The TypeScript examples, which `charter-ts new` copies: one route, and the notes API (sign-in, streams, rate limits) |
| `go/examples-start/`, `go/examples-notes/` | The same two APIs in Go, held to the TypeScript ones by `twins` |
| `go/examples-start-htmx/`, `go/examples-start-datastar/` | Go with server-rendered pages ([Server-rendered pages](pages.md)) |
| `ts/conformance/`, `go/conformance/` | Every Fern feature, its SDKs tested against the API, inside a Worker too; the TypeScript one is the reference |

The root `package.json` is the npm workspace of `charter-ts/`, `ts/` and `go/`'s projects; mise only installs the tools (`mise.toml`).

## What is generated

Never edit these: change the source, then run the command.

| Path | Written by |
|---|---|
| `fern/openapi.json`, `fern/asyncapi.json`, `fern/generators.yml` | `npm run spec`, from the contract; Fern's settings from the spec and `charter.config.ts` (`sdks`) |
| `api/queries.go`, `api/tables.ts` | `npm run spec`, from `migrations/*.sql` (and in Go `api/queries.sql`), as `db` in `charter.config.ts` says |
| `pages/*.x.go` (committed) | gsx, from the components; `npm run check` writes them and fails while they were stale |
| `sdk/go/` (committed) | `npm run sdk -- --publish` |
| `sdk/out/`, `build/`, `charter-ts/dist/`, `ts/lib/dist/` | `npm run sdk`; the Go Worker's build (`dev`, `check`, `deploy`) and the program that writes a Go project's specs (`build/_spec`, `spec`); `npm ci` |
| `ts/conformance/client/` | The SDK the conformance Worker runs, copied in by `check`, `dev` and `deploy` |
| `charter-ts/charter-files/`, `dist/` | `npm pack`, and `npm run release` |
| `.github/workflows/self-*.yml`, `charter-repo.yml` | `npm run adopt`, from `ci.config.ts` and charter's own job |
| `docs/charter/README.md` | `npm run adopt`, from `package.json`'s scripts |
| `package-lock.json` | `npm install` |

## Report a bug

Use the GitHub forms; for a bug in Fern, TinyGo, workers-go, Huma or oRPC, check [Upstream issues](upstream.md) first. An agent prints the same form:

```sh
npm run issues -- bug     # or feature, upstream, plan: the body, and the gh command that files it
```

## Make it faster

The target: a Go Worker costs no more CPU per request than its TypeScript twin. What was tried and measured is in [Benchmarks](benchmarks.md). Issues: [`perf`](https://github.com/joeblew999/charter/labels/perf).

## Pick up an issue

The plan is the [open issues](https://github.com/joeblew999/charter/issues). Before a pull request: `npm run check` passes, and `docs/` says what changed.

## A change to charter-ts

`npm run check -w charter-ts` runs its tests, beside the code (`charter-ts/src/*.test.ts`): `adopt`, `new` and `docs` among them, in throwaway folders. Try a change on this repo too (`npm run adopt`, `npm run docs`), and on a throwaway repo with the package you pack:

```sh
npm pack -w charter-ts --pack-destination /tmp/try
cd <a throwaway repo>
npx --package /tmp/try/charter-ts-X.Y.Z.tgz charter-ts adopt
```

## The CI

`ci.config.ts` at the root has this repo's own jobs, typed; `npm run adopt` writes a workflow for each (`.github/workflows/self-<job>.yml`), and `charter-repo.yml` is charter's `repo` job, the same in every repo. Every workflow calls the job runner (`charter-ci.yml`): mise installs the tools, npm the packages, then charter-ts runs the job as `npm run ci -- <job>` runs it here. One job in one project: `npm run ci -- clients go/conformance`.

Every command says what it made and how long it took: each artifact its size, gzipped, against its limit, and how that changed since it was last made here; each program of a second or more, each server's start, each job. `check` ends with both.

| Job | When | What |
|---|---|---|
| `repo` | Push, pull request | charter's job: charter's files and the workflows (`adopt --check`), the docs, the repo on GitHub, the upstream issues |
| `packages` | Push, pull request, tag; Linux and macOS | charter-ts and charter-lib with their tests, the Go library, the projects kept alike (`projects.test.ts`), the workflows linted |
| `projects` | Push, pull request, tag; one runner per project | Every project's check |
| `twins` | Push, pull request, tag | Each API that is here in TypeScript and in Go, both under workerd, the same requests to both: no answer may differ, nor their saved specs and migrations ([The standard API](standard.md#held-the-same)) |
| `new` | Push, pull request, tag | A project made by `charter-ts new` from the packed package, in both languages, installed, and its check passing (`charter-ts/test/new-from-package.ts`) |
| `clients` | Push, pull request, tag; one runner per conformance project | Every client its `fern/generators.yml` declares, generated and proven in its language |
| `deploy` | By hand: `npm run ci -- deploy <project> --remote` | A project's deploy, then its live tests |

## Where the tests are

Each test checks one thing, at one level, and the same level is in the same place in Go and in TypeScript:

| # | Level | Who needs it | Where | When it runs |
|---|---|---|---|---|
| 1 | **The standard:** a running API serves its specs, lists its MCP tools, leads a client to sign in, refuses with problem details | Every API | Built into charter-ts (`charter-ts/src/standard.ts`): it reads the API's own spec, so nothing is written or copied | Every project's `check` and `live-test`, first |
| 2 | **The API's own:** it does what it is for | Every API | The project's `test/` (`test/live.ts`, `test/sdk.ts`…); a Go twin runs its TypeScript reference's, as its `charter.config.ts` says | The project's `check` and `live-test` |
| 3 | **Unit tests:** a handler on its own | Where wanted | Beside the code: `api/*_test.go`, `api/*.test.ts` | The project's `check` |
| 4 | **Twins:** the Go project answers as its TypeScript reference | An API in both languages | Built into charter-ts (`charter-ts twins`) | The `twins` job |
| 5 | **The libraries** | charter's developers | Beside the code: `go/lib/*/*_test.go`, `ts/lib/src/*.test.ts`, `charter-ts/src/*.test.ts` | The `packages` job |
| 6 | **This repo only:** the projects are alike; a project made from the release works | charter's developers | `projects.test.ts`, `charter-ts/test/new-from-package.ts` | The `packages` and `new` jobs |

## Cut a release

```sh
npm run release -- X.Y.Z --dry-run   # every check, and the packages into dist/
npm run release -- X.Y.Z             # the version, the tag ts-vX.Y.Z, the GitHub Release with the packages
```

It runs `npm run check` before it tags, and never waits for GitHub. It tags `ts-vX.Y.Z` and `go/lib/vX.Y.Z` (the Go library's, which `go get` and `new --go` pin), and the Release carries `charter-ts-X.Y.Z.tgz` and `charter-lib-X.Y.Z.tgz`, which a repo pins. It puts that version into every install command the docs show (npx keeps what it fetched for a URL, so a URL without a version would serve an old one).
