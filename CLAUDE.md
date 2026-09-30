# Working in this repo (for agents)

oRPC API on Cloudflare Workers plus Fern-generated SDKs/CLI/docs. It's the reference for our other API projects, so keep it clean, pinned and verified. Read README.md, then api/README.md and .plans/realtime.md.

## Rules

- **mise drives everything.** Tasks live inline in `mise.toml`, and there's no `scripts/` folder. Add or change a task rather than a helper script. Test programs (`api/*.mjs`, `api/soak-go`) are code, not glue.
- **Exact pins, one source each.** Node, jq and gh go in `mise.toml`; Rust in `rust-toolchain.toml`; Go in the `toolchain` line of `go.work`. Lockfiles are committed. No global installs.
- **The contract is the source.** After changing `api/src/contract.ts`, run `mise run api:spec`. `mise run api:check` fails if a committed spec is stale. Never edit `sdk/fern/apis/api/*.json` by hand.
- **Workarounds name their upstream issue.** Tag them in the code as `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, add a row to the table in `api/README.md`, and check with `mise run upstream:status`.
- **Only verified results** go into FINDINGS.md.
- **Heavy jobs:**
  - `sdk:gen` needs Docker;
  - `sdk:cli:build:*` and `sdk:cloudflare` are heavy;
  - `api:soak` redeploys the Worker.
  - Stop the containers and dev servers you start.

## Using cf (the Cloudflare CLI, pinned in api/package.json)

- Tasks call `./node_modules/.bin/cf` from `api/` or `sdk/harness/`.
- `cf` is search-first: `cf cli search "<what you want>"` (keep queries anonymous), then `<command> --help`, then `cf schema <command>`.
- Output is JSON; lists return one page unless you pass `--per-page`; deletes need `--force` in non-interactive shells.
