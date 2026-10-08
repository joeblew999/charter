---
title: Rules
nav_order: 1
parent: How to help
---
# Rules for working in this repo

Binding, for developers and agents alike: other repos build on this one. These are this repo's own; the rules every repo shares are in [Rules for every repo](repo/rules.md).

| Rule | Why |
|---|---|
| **`docs/` is the single source of truth.** `AGENTS.md`, `CLAUDE.md` and folder READMEs only point to it. Pages follow [Writing docs](writing.md); run `mise run docs:lint` | One place cannot contradict itself |
| **Never edit what is generated** ([the list](README.md#what-is-generated)) | A hand edit is lost, and `mise run check` fails |
| **Every task is one line** of programs, double quotes and `&&`, which sh and cmd.exe both take. More is a command of the tool in `cmd/charter/`, added to [its table](reference/charter.md) | A task runs the same everywhere, Windows included |
| **A project is self-contained; task names are the same in every project.** Workflows only call mise, one task a job, so what GitHub runs you can run | `charter new` copies an example as it is, and CI runs what you run |
| **After changing a contract, `mise run spec`.** The Go and the oRPC contract of an API change together | The surface tests fail otherwise |
| **What ships to Workers builds with TinyGo,** checked under workerd. `go/` and its glue `go/worker/` change together | `go test` cannot see TinyGo's gaps; the two ship as one module |
| **Exact pins, one place each:** `mise.toml`, `go.mod`, `package.json`, `fern/generators.yml`; TinyGo in `cmd/charter/wasm.go`. Lockfiles are committed | A build is the same everywhere |
| **Every file the tool writes into a repo says so in its first lines:** ``Written by `charter <command>` `` and `don't edit`, or ``Started by `charter <command>` `` and `yours to edit` ([Any repo](guides/any-repo.md#tell-what-is-charters)) | A reader of that repo can tell what is charter's; a test fails on a template without it |
| **Workarounds name their issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)`, and a row in [Upstream issues](upstream.md) | Otherwise it is never removed |
| **After a deploy, the live test passes against it** | Some bugs exist only on Cloudflare |
| **Only verified results go into the docs:** what ran, where, when. Performance numbers only in [Benchmarks](benchmarks.md) | A reader acts on it; a copied number goes stale |
| **A release is a version tag, cut with `mise run release`** ([how](contributing.md#cut-a-release)); when only the tool or `tasks/` changed, [the fast track](contributing.md#fast-track-the-tool-only) | It checks and builds before the tag, in minutes; the workflows check the tag on every OS after |

## Working in a checkout

- **Local checks pick free ports,** so worktrees and agents can run at once. Give a server you start by hand its own `API_PORT`, and stop it.
- **Heavy:** `sdk:gen` (Docker), `sdk:cli:build` (Rust), `soak` (redeploys). `mise run sdk:clean` stops the containers.
- **Cloudflare's CLI** is `npx cf`, pinned per example. `cf cli search "<what>"` finds a command.
