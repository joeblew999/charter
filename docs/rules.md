---
title: Rules
nav_order: 1
parent: This repository
---
# Rules for working in this repo

The rules for changing this repo, for developers and agents alike. They are binding. The repo is the reference for our other API projects, so it stays clean, pinned and verified. Read [README.md](README.md) first, then [project.md](project.md): "What is what" names the parts, and the two servers, their contracts and their Fern folders are easy to mix up.

## Rules

- **`docs/` is the single source of truth.** Everything written about the repo goes in a page there. `AGENTS.md`, `CLAUDE.md` and the folder READMEs only point to it: don't put content in them. One place can't contradict itself.
- **Pages in `docs/` follow [writing.md](writing.md).** `mise run docs:lint` checks what a program can (links, tasks and paths that don't exist), and `mise run docs:review` has Claude check the rest. Run the lint before committing a docs change.
- **A new page in `docs/` starts with its sidebar lines** (`title`, `nav_order`, and `parent` if it sits under another page; copy them from any page) and gets a row in the start page's table. The lint fails without either.
- **`docs/_config.yml`, `docs/_sass/`, `docs/writing.md` and `docs/llms.txt` are written by `mise run docs:setup`.** Don't edit them: change the templates in `cmd/charter/docs/`, as with the workflows. `mise run charter:check` fails if they differ.
- **Docs are plain Markdown that GitHub Pages renders as it is.** Link pages relatively (`[api-go.md](api-go.md)`), and don't write two opening curly braces together or a curly brace followed by a percent sign: Jekyll reads those as template code.
- **mise drives everything, locally and on GitHub, and every task is one line.** Anything that needs more is a command of the `charter` tool (`cmd/charter/`, Go, standard library only): add a command there, then a one-line task that calls it ([dev.md](dev.md#adding-a-task)). No `scripts/` or tasks folder, no shell blocks in `mise.toml`. Test programs (`examples/notes-go/test/*.mjs`, `examples/notes-go/test/soak-go/`) are code, not glue.
- **GitHub workflows are generated and only call mise.** The templates are `cmd/charter/workflows/*.yml` (`check`, `deploy`, `sdk-check`, `release`). Edit a template, run `mise run workflows`, commit both; `mise run charter:check` fails if they differ. A step that does work is `mise run <task>`: no shell in YAML, so a failing step runs the same locally. Actions and runners are pinned to exact versions.
- **A release is a version tag** (`vX.Y.Z`), built by the `release` workflow. Don't upload release files or push the module tags (`go/vX.Y.Z`, `examples/notes-go/sdk/go/vX.Y.Z`) by hand ([dev.md](dev.md#cutting-a-release)).
- **Exact pins, one source each.** Go, Node, Rust, jq, gh and fnox go in `mise.toml`: a project's `mise.toml` is complete, so each example pins the tools its own tasks need, and a test of the tool (`cmd/charter/sdk_test.go`) holds every `mise.toml` in the repo to the same version of each. TinyGo and binaryen are pinned in the charter tool (`cmd/charter/wasm.go`), beside the patches made for that TinyGo, and `charter wasm-build` asks mise for exactly those; Go modules in `go.mod`; npm packages in each project's one `package.json`; Fern's generators in each `generators.yml`. Lockfiles are committed. No global installs.
- **The contract is the source, one per project.** After changing one, write its specs again, in that example's folder; its check fails if a committed spec is stale. Never edit a generated spec (`fern/openapi.json`, `fern/asyncapi.json` in each example) by hand.

  | Contract | Write the specs, in | The check that catches a stale spec |
  |---|---|---|
  | `examples/notes-ts/src/contract.ts` (the oRPC Worker) | `mise run spec`, in `examples/notes-ts/` | `mise run check` there |
  | `examples/notes-go/api/contract.go` (the Go Worker) | `mise run spec`, in `examples/notes-go/` | `mise run check` there |
  | `examples/showcase-ts/src/contract.ts` (the oRPC showcase) | `mise run spec`, in `examples/showcase-ts/` | `mise run check` there |
  | `examples/showcase-go/api/contract.go` (the Go showcase) | `mise run spec`, in `examples/showcase-go/` | `mise run check` there |

- **The two contracts of an API describe the same API.** Change both together: `TestTheNotesExamplesHaveTheSameSurface` (`examples/surface_test.go`, run by `mise run charter:check`) fails when what Fern sees of the notes API differs, and `TestSameSurfaceAsTheORPCShowcase` (`examples/showcase-go/api/`) does the same for the showcase.
- **Everything that ships to Workers from `examples/notes-go/` and `examples/showcase-go/` builds with TinyGo** (`mise run build` in each). Standard Go is for the native build, `go test` and the spec commands. `go test` can't see TinyGo's gaps, so each one's `check` also runs the Wasm under workerd.
- **Workarounds name their upstream issue.** Tag them in the code as `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, add a row to the table in [upstream.md](upstream.md), and check with `mise run upstream:status`. A workaround without its issue never gets removed.
- **Test locally and on Cloudflare.** `mise run check` is the local half. After a deploy, the live test must pass against the deployed Worker (the `deploy` workflow does it; by hand: `mise run live-test` in the example's folder). Some bugs exist only in production ([testing.md](testing.md)).
- **Only verified results** go into [findings.md](findings.md): what ran, where, when, and what came out.
- **Heavy jobs:**
  - `sdk:gen` needs Docker;
  - `sdk:cli:build` is heavy;
  - `soak` redeploys the example's Worker.
  - Stop the containers and dev servers you start.

## Starting in a fresh checkout or worktree

```sh
mise install && mise run setup     # tools, then every example's npm packages (a new worktree has neither node_modules nor sdk/out)
mise run doctor                    # what is missing, per example: npm packages, Docker, Go, Rust
mise run check                     # every local check (needs Docker, no Cloudflare account)
mise tasks                         # every task is one line; `go run ./cmd/charter help` lists what is behind them
cd examples/notes-go && mise tasks # each example is a project with its own tasks, under the same names
```

The local checks pick free ports themselves, so several can run at once (two worktrees, two agents). Give any server you start by hand a port of your own (`API_PORT`), and stop it when you are done.

## Using cf (the Cloudflare CLI)

- **It is pinned** in the `package.json` of each example, to the same version.
- **Tasks call `./node_modules/.bin/cf`** in the example's folder.
- **`cf` is search-first:** `cf cli search "<what you want>"` (keep queries anonymous), then `<command> --help`, then `cf schema <command>`.
- **Output is JSON;** lists return one page unless you pass `--per-page`; deletes need `--force` in non-interactive shells.
