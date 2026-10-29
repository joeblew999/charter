---
title: What to do
nav_order: 100
has_children: true
---

# What to do, and when

Written by `charter init` from charter's commands: don't edit, it is the same in every repo charter keeps (after another release of charter, `charter init` writes it again).

Each is `charter <command>`: the charter the repo's `mise.toml` pins (`mise use github:joeblew999/charter`), or `mise run charter:<command>`. What it takes follows it (`charter issues plan`). [The rules](rules.md) are binding; [Writing docs](writing.md) says how a page is written; this repo's own pages are in [self/](../self/README.md).

| When | Run | What it does |
|---|---|---|
| First, whenever you start; with a kind, to file a bug or a plan | `charter issues` | REMOTE, read-only: the open issues, newest first; with a kind, the body of a new one and the gh command that files it |
| Before every commit | `charter check` | every check that needs no Cloudflare account: charter's files; in a project, its types, its specs against the contract, its tests, the Worker run and tested, its size; at a repo's top, the jobs charter.toml's [ci] check lists; -go: the folder's Go module (gofmt, go vet for Wasm too, go test) |
| After changing a page in docs/ | `charter docs` | what a program can see wrong in docs/ (front matter, links and anchors, pages nothing links to, commands, scripts and paths that don't exist, pinned release versions); -review has Claude read it for the rest |
| To run a job as GitHub does, here; -remote to start it there | `charter ci` | a job of charter.toml's [ci.jobs] here, as GitHub runs it; with none, every job GitHub runs on a change (an app with no jobs: its [ci], charter ci run); -remote starts the job on GitHub instead |
| Now and then: a CLOSED one means its workaround can go | `charter upstream` | REMOTE, read-only: the upstream issues the code works around (its Upstream: tags), and their state |
| After changing charter.toml's [github] | `charter repo` | REMOTE: the repo on GitHub as charter.toml's [github] says: labels, description, homepage, topics, GitHub Pages (a public repo's); -check changes nothing and fails on drift |
| After installing another release of charter, and after changing charter.toml's [ci] or [github] | `charter init` | write charter's files into the repo (never one of the repo's own), its mise tasks and skill, and a workflow per job of charter.toml; -check writes nothing and fails if one differs |

In a project (`examples/conformance-go`, `examples/conformance-ts`, `examples/notes-go`, `examples/notes-ts`, `examples/start-datastar`, `examples/start-go`, `examples/start-htmx`, `examples/start-ts`), from its folder, where its `charter.toml` says what its steps need:

| When | Run | What it does |
|---|---|---|
| To try the API on your machine | `charter dev` | the Worker on your machine, as on Cloudflare (workerd), its local database migrated, until Ctrl-C; -native, in Go: go run . |
| After every change to the contract, to charter.toml's [sdks], or to migrations/ or api/queries.sql | `charter spec` | write both specs (fern/openapi.json, fern/asyncapi.json) from the contract, Fern's settings (fern/generators.yml) from them and [sdks], and the database's typed code from migrations/ (db: sqlc in Go, drizzle in TypeScript) |
| When a caller needs an SDK; -prove after changing [sdks]; -publish after a contract change, where the repo commits its Go SDK | `charter sdk` | an SDK with Fern (Docker), by default the TypeScript one, into sdk/out; -prove: every client, proven in its language; -publish, in Go: the Go SDK into sdk/go |
| To put the API on Cloudflare | `charter deploy` | REMOTE: the Worker on Cloudflare with its secrets (fnox exec -- charter deploy production), then its migrations |
| After every deploy | `charter live-test` | REMOTE: the standard test and the project's tests against the deployed Worker |

In charter's own repo, charter is this checkout's: `go tool charter <command>`. Its release: `go tool charter release X.Y.Z`.

Every command, with what it takes: `charter help`.
