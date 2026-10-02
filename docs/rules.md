---
title: Rules
nav_order: 1
parent: This repository
---
# Rules for working in this repo

The rules for changing this repo, for developers and agents alike. They are binding. The repo is the reference for our other API projects, so it stays clean, pinned and verified. Read [README.md](README.md) first: "What is what" names the parts, and the two servers, their contracts and their Fern folders are easy to mix up.

## Rules

- **`docs/` is the single source of truth.** Everything written about the repo goes in a page there. `AGENTS.md`, `CLAUDE.md` and the folder READMEs only point to it: don't put content in them. One place can't contradict itself.
- **Pages in `docs/` follow [writing.md](writing.md).** `mise run docs:lint` checks what a program can (links, tasks and paths that don't exist), and `mise run docs:review` has Claude check the rest. Run the lint before committing a docs change.
- **A new page in `docs/` starts with its sidebar lines** (`title`, `nav_order`, and `parent` if it sits under another page; copy them from any page) and gets a row in the start page's table. The lint fails without either.
- **`docs/_config.yml`, `docs/_sass/`, `docs/writing.md` and `docs/llms.txt` are written by `mise run docs:setup`.** Don't edit them: change the templates in `dev/docs/`, as with the workflows. `mise run dev:check` fails if they differ.
- **Docs are plain Markdown that GitHub Pages renders as it is.** Link pages relatively (`[api-go.md](api-go.md)`), and don't write two opening curly braces together or a curly brace followed by a percent sign: Jekyll reads those as template code.
- **mise drives everything, locally and on GitHub, and every task is one line.** Anything that needs more is a command of the `dev` tool (`dev/`, Go, standard library only): add a command there, then a one-line task that calls it ([dev.md](dev.md#adding-a-task)). No `scripts/` or tasks folder, no shell blocks in `mise.toml`. Test programs (`test/*.mjs`, `test/soak-go/`) are code, not glue.
- **GitHub workflows are generated and only call mise.** The templates are `dev/workflows/*.yml` (prefix `api-`, `sdk-` or `dev-`). Edit a template, run `mise run dev:workflows`, commit both; `mise run dev:check` fails if they differ. A step that does work is `mise run <task>`: no shell in YAML, so a failing step runs the same locally. Actions and runners are pinned to exact versions.
- **A release is a version tag** (`vX.Y.Z`), built by the `*-release` workflows. Don't upload release files or push the module tags (`api/go/vX.Y.Z`, `dev/vX.Y.Z`) by hand ([dev.md](dev.md#cutting-a-release)).
- **Exact pins, one source each.** Node, jq, gh, TinyGo, binaryen and fnox go in `mise.toml`; Rust in `rust-toolchain.toml`; Go in the `toolchain` line of `go.work`; Go modules in `go.mod`; Fern's generators in each `generators.yml`. Lockfiles are committed. No global installs.
- **The contract is the source, one per server.** After changing one, write its specs again; the check fails if a committed spec is stale. Never edit a generated spec (`sdk/fern/apis/<api>/openapi.json`, `asyncapi.json`) by hand.

  | Contract | Write the specs | The check that catches a stale spec |
  |---|---|---|
  | `api/ts/src/contract.ts` (the oRPC Worker) | `mise run api:ts:spec` | `mise run api:ts:check` |
  | `api/go/api/contract.go` (the Go Worker) | `mise run api:go:spec` | `mise run api:go:check` |
  | `sdk/harness/src/contract.ts` (the oRPC showcase) | `mise run showcase:ts:spec` | `mise run showcase:ts:check` |
  | `api/go/showcase/contract.go` (the Go showcase) | `mise run showcase:go:spec` | `mise run showcase:go:check` |

- **The two contracts of an API describe the same API.** Change both together: `TestSameSurfaceAsTheORPCContract` (`api/go/api/`) fails when what Fern sees of the notes API differs, and `TestSameSurfaceAsTheORPCShowcase` (`api/go/showcase/`) does the same for the showcase.
- **Everything that ships to Workers from `api/go/` builds with TinyGo** (`mise run api:go:build`, `mise run showcase:go:build`). Standard Go is for the native build, `go test` and the spec commands. `go test` can't see TinyGo's gaps, so `api:go:check` and `showcase:go:check` also run the Wasm under workerd.
- **Workarounds name their upstream issue.** Tag them in the code as `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, add a row to the table in [upstream.md](upstream.md), and check with `mise run upstream:status`. A workaround without its issue never gets removed.
- **Test locally and on Cloudflare.** `mise run check` is the local half. After a deploy, the live test must pass against the deployed Worker (the `api-deploy` workflow does it; by hand: `mise run api:ts:live-test` or `mise run api:go:live-test`). Some bugs exist only in production ([testing.md](testing.md)).
- **Only verified results** go into [findings.md](findings.md): what ran, where, when, and what came out.
- **Heavy jobs:**
  - `sdk:gen` needs Docker;
  - `sdk:cli:build` and `sdk:cloudflare` are heavy;
  - `api:ts:soak` and `api:go:soak` redeploy their Worker.
  - Stop the containers and dev servers you start.

## Starting in a fresh checkout or worktree

```sh
mise install && mise run setup     # tools, then npm packages (a new worktree has neither node_modules nor sdk/out)
mise run doctor                    # what is missing: npm installs, Docker, Go, TinyGo, Rust
mise run check                     # every local check (needs Docker, no Cloudflare account)
mise tasks                         # every task is one line; `go run ./dev help` lists what is behind them
```

The local checks pick free ports themselves, so several can run at once (two worktrees, two agents). Give any server you start by hand a port of your own (`PORT`, `API_GO_PORT`, `SHOWCASE_GO_PORT`), and stop it when you are done.

## Using cf (the Cloudflare CLI)

- **It is pinned** in the `package.json` of `api/ts/`, `api/go/` and `sdk/harness/`, to the same version.
- **Tasks call `./node_modules/.bin/cf`** from one of those folders. The Go showcase uses `api/go/`'s.
- **`cf` is search-first:** `cf cli search "<what you want>"` (keep queries anonymous), then `<command> --help`, then `cf schema <command>`.
- **Output is JSON;** lists return one page unless you pass `--per-page`; deletes need `--force` in non-interactive shells.
