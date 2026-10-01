# Rules for working in this repo

One notes API on Cloudflare Workers, built twice (oRPC in `api/`, Go with Huma on workers-go in `api-go/`), plus Fern-generated SDKs/CLI/docs. It's the reference for our other API projects, so keep it clean, pinned and verified. These rules are for developers and agents alike. Read [README.md](README.md) first ("What is what": it's easy to mix up the two servers, their contracts and their Fern folders).

## Rules

- **`docs/` is the single source of truth.** Everything written about the repo goes in a page there. `AGENTS.md`, `CLAUDE.md` and the folder READMEs only point to it: don't put content in them.
- **Docs are plain Markdown that GitHub Pages renders as it is.** Link pages relatively (`[api-go.md](api-go.md)`), and don't write two opening curly braces together or a curly brace followed by a percent sign: Jekyll reads those as template code.
- **mise drives everything, locally and on GitHub, and every task is one line.** Anything that needs more is a command of the `dev` tool (`dev/`, Go, standard library only): add a command there, then a one-line task that calls it. No `scripts/` or tasks folder, no shell blocks in `mise.toml`. Test programs (`test/*.mjs`, `test/soak-go`) are code, not glue.
- **GitHub workflows are generated and only call mise.** The templates are `dev/workflows/*.yml` (prefix `api-`, `sdk-` or `dev-`). Edit a template, run `mise run dev:workflows`, commit both; `mise run dev:check` fails if they differ. A step that does work is `mise run <task>`: no shell in YAML. Actions and runners are pinned to exact versions.
- **A release is a version tag** (`vX.Y.Z`), built by the `*-release` workflows. Don't upload release files or push the module tags (`api-go/vX.Y.Z`, `dev/vX.Y.Z`) by hand. See docs/dev.md.
- **Exact pins, one source each.** Node, jq, gh, TinyGo and binaryen go in `mise.toml`; Rust in `rust-toolchain.toml`; Go in the `toolchain` line of `go.work`; Go modules in `go.mod`. Lockfiles are committed. No global installs.
- **The contract is the source, one per server.**
  - oRPC: after changing `api/src/contract.ts`, run `mise run api:spec`. `mise run api:check` fails if a committed spec is stale.
  - Go: after changing `api-go/api/contract.go`, run `mise run api-go:spec`. `mise run api-go:check` fails if a committed spec is stale.
  - Never edit `sdk/fern/apis/api/*.json` or `sdk/fern/apis/api-go/*.json` by hand.
- **The two contracts describe the same API.** Change both together: `api-go`'s `TestSameSurfaceAsTheORPCContract` fails when what Fern sees differs.
- **Everything that ships to Workers from `api-go/` builds with TinyGo** (`mise run api-go:build`). Standard Go is for the native build, `go test` and `cmd/spec`. `go test` can't see TinyGo's gaps, so `api-go:check` also runs the Wasm under workerd.
- **Workarounds name their upstream issue.** Tag them in the code as `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, add a row to the table in `docs/upstream.md`, and check with `mise run upstream:status`.
- **Test locally and on Cloudflare.** `mise run check` is the local half. After a deploy, the live test must pass against the deployed Worker (`api-deploy` does it; by hand: `mise run api:live-test` or `api-go:live-test`). Some bugs exist only in production.
- **Only verified results** go into docs/findings.md.
- **Heavy jobs:**
  - `sdk:gen` needs Docker;
  - `sdk:cli:build` and `sdk:cloudflare` are heavy;
  - `api:soak` and `api-go:soak` redeploy their Worker.
  - Stop the containers and dev servers you start.

## Starting in a fresh checkout or worktree

```sh
mise install && mise run setup     # tools, then npm packages (a new worktree has neither node_modules nor sdk/out)
mise run check                     # every local check; about 30 s once things are cached
mise tasks                         # every task is one line; `go run ./dev help` lists what is behind them
```

The local checks pick free ports themselves, so several can run at once (two worktrees, two agents). Give any server you start by hand a port of your own, and stop it when you are done.

## Using cf (the Cloudflare CLI, pinned in api/package.json)

- Tasks call `./node_modules/.bin/cf` from `api/`, `api-go/` or `sdk/harness/`.
- `cf` is search-first: `cf cli search "<what you want>"` (keep queries anonymous), then `<command> --help`, then `cf schema <command>`.
- Output is JSON; lists return one page unless you pass `--per-page`; deletes need `--force` in non-interactive shells.
