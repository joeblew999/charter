# Working in this repo (for agents)

One notes API on Cloudflare Workers, built twice (oRPC in `api/`, Go with Huma on workers-go in `api-go/`), plus Fern-generated SDKs/CLI/docs. It's the reference for our other API projects, so keep it clean, pinned and verified. Read README.md ("What is what" first: it's easy to mix up the two servers, their contracts and their Fern folders), then api/README.md, api-go/README.md and .plans/realtime.md.

## Rules

- **mise drives everything, locally and on GitHub, and every task is one line.** Anything that needs more is a command of the `dev` tool (`dev/`, Go, standard library only): add a command there, then a one-line task that calls it. No `scripts/` or tasks folder, no shell blocks in `mise.toml`. Test programs (`test/*.mjs`, `test/soak-go`) are code, not glue.
- **GitHub workflows are generated and only call mise.** The templates are `dev/workflows/*.yml` (prefix `api-`, `sdk-` or `dev-`). Edit a template, run `mise run dev:workflows`, commit both; `mise run dev:check` fails if they differ. A step that does work is `mise run <task>`: no shell in YAML. Actions and runners are pinned to exact versions.
- **A release is a version tag** (`vX.Y.Z`), built by the `*-release` workflows. Don't upload release files or push the module tags (`api-go/vX.Y.Z`, `dev/vX.Y.Z`) by hand. See dev/README.md.
- **Exact pins, one source each.** Node, jq, gh, TinyGo and binaryen go in `mise.toml`; Rust in `rust-toolchain.toml`; Go in the `toolchain` line of `go.work`; Go modules in `go.mod`. Lockfiles are committed. No global installs.
- **The contract is the source, one per server.**
  - oRPC: after changing `api/src/contract.ts`, run `mise run api:spec`. `mise run api:check` fails if a committed spec is stale.
  - Go: after changing `api-go/api/contract.go`, run `mise run api-go:spec`. `mise run api-go:check` fails if a committed spec is stale.
  - Never edit `sdk/fern/apis/api/*.json` or `sdk/fern/apis/api-go/*.json` by hand.
- **The two contracts describe the same API.** Change both together: `api-go`'s `TestSameSurfaceAsTheORPCContract` fails when what Fern sees differs.
- **Everything that ships to Workers from `api-go/` builds with TinyGo** (`mise run api-go:build`). Standard Go is for the native build, `go test` and `cmd/spec`. `go test` can't see TinyGo's gaps, so `api-go:check` also runs the Wasm under workerd.
- **Workarounds name their upstream issue.** Tag them in the code as `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, add a row to the table in `api/README.md`, and check with `mise run upstream:status`.
- **Only verified results** go into FINDINGS.md.
- **Heavy jobs:**
  - `sdk:gen` needs Docker;
  - `sdk:cli:build` and `sdk:cloudflare` are heavy;
  - `api:soak` and `api-go:soak` redeploy their Worker.
  - Stop the containers and dev servers you start.

## Using cf (the Cloudflare CLI, pinned in api/package.json)

- Tasks call `./node_modules/.bin/cf` from `api/`, `api-go/` or `sdk/harness/`.
- `cf` is search-first: `cf cli search "<what you want>"` (keep queries anonymous), then `<command> --help`, then `cf schema <command>`.
- Output is JSON; lists return one page unless you pass `--per-page`; deletes need `--force` in non-interactive shells.
