---
title: Rules
nav_order: 14
---
# Rules for working in this repo

Binding, for developers and agents alike: other repos build on this one. These are this repo's own; the rules every repo shares are in [Rules for every repo](../charter/rules.md).

| Rule | Why |
|---|---|
| **`docs/` is the single source of truth.** `AGENTS.md`, `CLAUDE.md` and folder READMEs only point to it. Pages follow [Writing docs](../charter/writing.md); run `npm run docs` | One place cannot contradict itself |
| **Never edit what is generated** ([the list](contributing.md#what-is-generated)) | A hand edit is lost, and `npm run check` fails |
| **Every npm script is the charter-ts command of its own name** (`"check": "charter-ts check"`), the same in every project; more is a command of charter-ts (`charter-ts/src/commands.ts`), typed. `projects.test.ts` fails otherwise | One way to run each thing, the same everywhere, Windows included |
| **A project is self-contained,** set up by its `charter.config.ts` | `charter-ts new` copies an example as it is |
| **After changing a contract, `npm run spec`** (`npm run spec` in Go). The Go and the oRPC contract of an API change together | `npx charter-ts twins` fails otherwise |
| **What ships to Workers builds with TinyGo,** checked under workerd. `go/` and its glue `go/lib/worker/` change together | `go test` cannot see TinyGo's gaps; the two ship as one module |
| **Exact pins, one place each:** `mise.toml`, `go.mod`, `package.json`, `fern/generators.yml`; and the toolchains charter-ts asks mise for when it needs them: TinyGo in `charter-ts/src/go.ts`, Rust, uv and Python's checkers in `charter-ts/src/clients.ts`. Lockfiles are committed | A build is the same everywhere |
| **Every file the tool writes into a repo says so in its first lines:** ``Written by `charter-ts <command>` `` and `don't edit`, or ``Started by `charter-ts <command>` `` and `yours to edit` ([Any repo](any-repo.md#tell-what-is-charters)) | A reader of that repo can tell what is charter's; a test fails on a template without it |
| **Workarounds name their issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, and a row in [Upstream issues](upstream.md) | Otherwise it is never removed |
| **After a deploy, the live test passes against it** | Some bugs exist only on Cloudflare |
| **Only verified results go into the docs:** what ran, where, when. Performance numbers only in [Benchmarks](benchmarks.md) | A reader acts on it; a copied number goes stale |
| **A release of charter-ts is a tag `ts-vX.Y.Z`, cut with `npm run release -- X.Y.Z`** ([how](contributing.md#cut-a-release)) | It runs every local check before the tag, and never waits for GitHub |

## Working in a checkout

- **Local checks pick free ports,** so worktrees and agents can run at once. Give a server you start by hand its own `API_PORT`, and stop it.
- **Heavy:** `npm run sdk` (Docker), the Go builds (TinyGo, reused from charter's cache when the sources are the same), `test/soak.ts` (redeploys).
- **Cloudflare's CLI** is `npx cf`, pinned per example. `cf cli search "<what>"` finds a command.
