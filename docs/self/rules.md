---
title: Rules
nav_order: 15
---
# Rules for working in this repo

Binding, for developers and agents alike: other repos build on this one. These are this repo's own; the rules every repo shares are in [Rules for every repo](../charter/rules.md).

| Rule | Why |
|---|---|
| **Every command is charter's** (`cmd/charter`; in this repo `go tool charter <command>`), a project's from its `charter.toml` and the repo's from the top's. The only npm script is npm's `prepare` (charter-lib built), and a project has none: `examples/examples_test.go` fails otherwise. A command is listed once, in `internal/cli/repo` (`Commands`), which `charter help` and `docs/charter/README.md` are written from, and a test holds it equal to what `cmd/charter` runs | One way to run each thing, the same everywhere, Windows included |
| **A project is self-contained,** set up by its `charter.toml`: in this repo only a Go twin reaches out, to run its TypeScript twin's `test/`, which `new -go` copies in | `charter new` copies an example and it works on its own |
| **After changing a contract, `charter spec`.** The Go and the TypeScript contract of an API change together | `charter ci twins` fails otherwise |
| **What ships to Workers builds with TinyGo,** checked under workerd. The Go library and its Worker glue `worker/` change together | `go test` cannot see TinyGo's gaps; the two ship as one module |
| **Pins in one place each:** the tools in `mise.toml`, TinyGo, wasm-opt, sqlc and workerd among them (charter runs them through mise, so mise installs them and Renovate moves them, and a new repo's `mise.toml` starts from them: `internal/pins/repo.toml`, equal to `mise.toml`, a test); packages in `go.mod` (esbuild among them) and `package.json` (ranges; the lockfiles, committed, are exact); and in charter, what it writes or fetches when it needs it: Fern's generators, Rust, uv and Python's checkers in `internal/fern`; actionlint in the top's `charter.toml`, the job runner's actions in `charter-ci.yml`. TinyGo's runtime patches are in `internal/cli/build`, beside the build that applies them, and fail it, saying so, when the pinned TinyGo no longer has what they change. An app gets TinyGo and wasm-opt from charter, never its own pin: `internal/pins/mise.toml`, equal to charter's `mise.toml` (a test), which `charter tasks` writes into it as `.config/mise/conf.d/charter.toml` | A build is the same everywhere |
| **Every file charter writes into a repo says so in its first lines:** ``Written by `charter <command>` `` and `don't edit`, or ``Started by `charter <command>` `` and `yours to edit` ([Any repo](any-repo.md#tell-what-is-charters)) | A reader of that repo can tell what is charter's |
| **A workaround also gets a row** in [Upstream issues](upstream.md) | What it waits for, and what to do then, in one table |
| **After a deploy, the live test passes against it** | Some bugs exist only on Cloudflare |
| **Only verified results go into the docs:** what ran, where, when. Performance numbers only in [Benchmarks](benchmarks.md) | A reader acts on it; a copied number goes stale |
| **A release is `go tool charter release X.Y.Z`** ([how](contributing.md#cut-a-release)) | It runs the check before it tags, and never waits for GitHub |

## Working in a checkout

- **Local checks pick free ports,** so worktrees and agents can run at once. Give a server you start by hand its own `PORT`, and stop it.
- **Heavy:** `charter sdk` (Docker), the Go builds (TinyGo, reused from charter's cache when the sources are the same), `test/soak.ts` (redeploys).
