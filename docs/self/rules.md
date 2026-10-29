---
title: Rules
nav_order: 15
---
# Rules for working in this repo

Binding, for developers and agents alike: other repos build on this one. These are this repo's own; the rules every repo shares are in [Rules for every repo](../charter/rules.md).

| Rule | Why |
|---|---|
| **Every npm script is the charter-ts command of its own name** (`"check": "charter-ts check"`), the same in every project; npm's `prepare` and this repo's `release` are the only others. More is a command of charter-ts (`charter-ts/src/commands.ts`), typed. `projects.test.ts` fails otherwise | One way to run each thing, the same everywhere, Windows included |
| **A project is self-contained,** set up by its `charter.config.ts` (TypeScript) or `charter.toml` (Go): in this repo only a Go twin reaches out, to run its TypeScript twin's `test/`, which `new --go` copies in | `charter-ts new` copies an example and it works on its own |
| **After changing a contract, `npm run spec`.** The Go and the TypeScript contract of an API change together | `npm run ci -- twins` fails otherwise |
| **What ships to Workers builds with TinyGo,** checked under workerd. The Go library and its Worker glue `worker/` change together | `go test` cannot see TinyGo's gaps; the two ship as one module |
| **Pins in one place each:** the tools in `mise.toml`, TinyGo, wasm-opt and sqlc among them (charter-ts reads them there, `charter-ts/src/pins.ts`, so mise installs them, Renovate moves them, and a new repo's `mise.toml` starts from them); packages in `go.mod` and `package.json` (ranges; the lockfiles, committed, are exact); and in charter-ts, what it writes or fetches when it needs it: Fern's generators, Rust, uv and Python's checkers in `internal/fern` (Go projects) and `charter-ts/src/sdk.ts` (TypeScript ones, until they move to charter too, #121; a test holds the two equal), the Workers compatibility date in `charter-ts/src/cloudflare.ts`, actionlint in `ci.config.ts`, the job runner's actions in `charter-ci.yml`. TinyGo's runtime patches are in `internal/cli/build`, beside the build that applies them, and fail it, saying so, when the pinned TinyGo no longer has what they change. An app gets TinyGo and wasm-opt from charter, never its own pin: `internal/pins/mise.toml`, equal to charter's `mise.toml` (a test), which `charter tasks` writes into it as `.config/mise/conf.d/charter.toml` | A build is the same everywhere |
| **Every file charter-ts writes into a repo says so in its first lines:** ``Written by `charter-ts <command>` `` and `don't edit`, or ``Started by `charter-ts <command>` `` and `yours to edit` ([Any repo](any-repo.md#tell-what-is-charters)) | A reader of that repo can tell what is charter's |
| **A workaround also gets a row** in [Upstream issues](upstream.md) | What it waits for, and what to do then, in one table |
| **After a deploy, the live test passes against it** | Some bugs exist only on Cloudflare |
| **Only verified results go into the docs:** what ran, where, when. Performance numbers only in [Benchmarks](benchmarks.md) | A reader acts on it; a copied number goes stale |
| **A release is `npm run release -- X.Y.Z`** ([how](contributing.md#cut-a-release)) | It runs the check before it tags, and never waits for GitHub |

## Working in a checkout

- **Local checks pick free ports,** so worktrees and agents can run at once. Give a server you start by hand its own `API_PORT`, and stop it.
- **Heavy:** `npm run sdk` (Docker), the Go builds (TinyGo, reused from charter's cache when the sources are the same), `test/soak.ts` (redeploys).
- **Cloudflare's CLI** is `npx cf`, pinned per project. `cf cli search "<what>"` finds a command.
