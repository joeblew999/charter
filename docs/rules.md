---
title: Rules
nav_order: 1
parent: How to help
---
# Rules for working in this repo

The rules for changing charter, for developers and agents alike. They are binding. Other repos build on this one, so it stays clean, pinned and verified. Read [what is what](README.md#what-is-what) first.

## The rules

| Rule | Why |
|---|---|
| **`docs/` is the single source of truth.** `AGENTS.md`, `CLAUDE.md` and folder READMEs only point to it | One place cannot contradict itself |
| **Pages follow [Writing docs](writing.md).** Run `mise run docs:lint` before committing a docs change | The lint finds dead links, tasks and paths |
| **Never edit what is generated** ([the list](README.md#what-is-generated)). Change the source or the template, run the task | A hand edit is lost at the next run, and `mise run check` fails until then |
| **mise drives everything, and every task is one line.** What needs more is a command of the tool ([how to add one](reference/charter.md#add-a-command)). No scripts folder, no shell blocks in `mise.toml` | A task then runs the same locally, on GitHub and in every project |
| **A project is self-contained, and task names are the same in every project.** The root `mise.toml` only runs each example's tasks | `charter new` copies an example as it is; nothing is filtered |
| **Workflows only call mise.** A step that does work is `mise run <task>`; actions and runners are pinned | A failing step runs the same on your machine |
| **The contract is the source.** After changing one, `mise run spec` in that example | A stale spec fails the check |
| **The Go and the oRPC contract of an API describe the same API.** Change both together | `TestTheNotesExamplesHaveTheSameSurface` and `TestSameSurfaceAsTheORPCShowcase` fail otherwise |
| **Everything that ships to Workers builds with TinyGo.** Standard Go is for the native build, `go test` and the spec commands | `go test` cannot see TinyGo's gaps, so each check also runs the Wasm under workerd |
| **The library's Go and its Worker glue change together** (`go/` and `go/worker/`) | They ship as one module, and a project gets both from one version |
| **Exact pins, one place each** ([where](reference/config.md#pinned-versions)). Lockfiles are committed. No global installs | A build is the same everywhere |
| **Workarounds name their upstream issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)` in the code, and a row in [Upstream issues](upstream.md) | A workaround without its issue is never removed |
| **Test locally and on Cloudflare.** After a deploy the live test must pass against the deployed Worker | Some bugs exist only in production |
| **Only verified results go into [Findings](findings.md):** what ran, where, when, what came out | A reader acts on it |
| **Performance numbers live in [Benchmarks](benchmarks.md) only,** with where and when they were measured. Prove a claim with `mise run compare` | A repeated number goes stale in one of its copies |
| **A release is a version tag.** Never upload release files or push module tags by hand ([how](reference/releases.md#how-a-release-is-cut)) | The workflow builds from the tagged commit |

## A fresh checkout or worktree

Set up as in [How to help](contributing.md#set-up).

- **The local checks pick free ports,** so several can run at once: two worktrees, two agents.
- **Give a server you start by hand a port of your own** (`API_PORT`), and stop it when you are done.
- **Heavy jobs:** `sdk:gen` needs Docker, `sdk:cli:build` compiles Rust, `soak` redeploys a Worker. Stop the containers you start (`mise run sdk:clean`).

## Using cf, Cloudflare's CLI

- **It is pinned** in each example's `package.json`. Tasks call `./node_modules/.bin/cf` from the example's folder.
- **It is search-first:** `cf cli search "<what you want>"`, then `<command> --help`, then `cf schema <command>`.
- **Output is JSON.** Lists return one page unless you pass `--per-page`; deletes need `--force` in a shell without a terminal.
