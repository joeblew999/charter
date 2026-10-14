---
title: Rules
nav_order: 1
parent: How to help
---
# Rules for working in this repo

Binding, for developers and agents alike: other repos build on this one. These are this repo's own; the rules every repo shares are in [Rules for every repo](charter/rules.md).

| Rule | Why |
|---|---|
| **`docs/` is the single source of truth.** `AGENTS.md`, `CLAUDE.md` and folder READMEs only point to it. Pages follow [Writing docs](charter/writing.md); run `npm run docs:lint` | One place cannot contradict itself |
| **Never edit what is generated** ([the list](README.md#what-is-generated)) | A hand edit is lost, and `npm run check` fails |
| **Every npm script is one line** of programs, double quotes and `&&`, which sh and cmd.exe both take. More is a command of charter-ts (`charter-ts/src/steps.ts`), typed, which [its table](charter/using.md) lists | A script runs the same everywhere, Windows included |
| **A project is self-contained; its steps are the same in every project:** npm scripts that run `charter-ts <step>`, set up by its `charter.config.ts`. A workflow runs one script, so what GitHub runs you can run | `charter-ts new` copies an example as it is, and CI runs what you run |
| **After changing a contract, `npm run spec`** (`npm run spec` in Go). The Go and the oRPC contract of an API change together | The surface tests fail otherwise |
| **What ships to Workers builds with TinyGo,** checked under workerd. `go/` and its glue `go/lib/worker/` change together | `go test` cannot see TinyGo's gaps; the two ship as one module |
| **Exact pins, one place each:** `mise.toml`, `go.mod`, `package.json`, `fern/generators.yml`; TinyGo in `charter-ts/src/go.ts`. Lockfiles are committed | A build is the same everywhere |
| **Every file the tool writes into a repo says so in its first lines:** ``Written by `charter-ts <command>` `` and `don't edit`, or ``Started by `charter-ts <command>` `` and `yours to edit` ([Any repo](guides/any-repo.md#tell-what-is-charters)) | A reader of that repo can tell what is charter's; a test fails on a template without it |
| **Workarounds name their issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, and a row in [Upstream issues](upstream.md) | Otherwise it is never removed |
| **After a deploy, the live test passes against it** | Some bugs exist only on Cloudflare |
| **Only verified results go into the docs:** what ran, where, when. Performance numbers only in [Benchmarks](benchmarks.md) | A reader acts on it; a copied number goes stale |
| **A release of charter-ts is a tag `ts-vX.Y.Z`, cut with `npm run release -- X.Y.Z`** ([how](contributing.md#cut-a-release)) | It runs every local check before the tag, and never waits for GitHub |

## Working in a checkout

- **Local checks pick free ports,** so worktrees and agents can run at once. Give a server you start by hand its own `API_PORT`, and stop it.
- **Heavy:** `npm run sdk` (Docker), `npm run sdk:publish` (Docker), the Go builds (TinyGo), `test/soak.mjs` (redeploys).
- **Cloudflare's CLI** is `npx cf`, pinned per example. `cf cli search "<what>"` finds a command.
