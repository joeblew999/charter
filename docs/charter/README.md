---
title: What to do
nav_order: 100
has_children: true
---

# What to do, and when

Written by `charter-ts adopt` from the repo's scripts (package.json): don't edit, change the scripts, then `npm run adopt`.

Each is `npm run <name>`; what it takes goes after `--` (`npm run issues -- plan`). [The rules](rules.md) are binding; [Writing docs](writing.md) says how a page is written; this repo's own pages are in [self/](../self/README.md).

| When | Run | What it does |
|---|---|---|
| First, whenever you start; with a kind, to file a bug or a plan | `npm run issues` | REMOTE, read-only: the open issues, newest first; with a kind (bug, feature, upstream, plan), the body of a new one and the gh command that files it |
| Before every commit | `npm run check` | every check that needs no Cloudflare account: in a project, the types, the specs against the contract, the tests, the API run and tested, its size; in a Go module, gofmt, go vet and go test; at a repo's top, the jobs ci.config.ts lists in check (gh signed in, for the repo job) |
| After changing a page in docs/ | `npm run docs` | what a program can see wrong in docs/ (front matter, links and anchors, pages nothing links to, scripts and paths that don't exist, pinned release versions); --review has Claude read it for the rest |
| To run a job as GitHub does, here; --remote to start it there | `npm run ci` | a job of ci.config.ts here, as GitHub runs it; with no job, every job GitHub runs on a change; --remote starts the job on GitHub instead |
| Now and then: a CLOSED one means its workaround can go | `npm run upstream` | REMOTE, read-only: the upstream issues the code works around (its `Upstream:` tags), and their state |
| After changing package.json's description, keywords or homepage | `npm run repo` | REMOTE: the repo on GitHub as package.json says: labels, description, homepage, topics, GitHub Pages; --check changes nothing and fails on drift |
| After installing another release of charter-ts, and after changing ci.config.ts | `npm run adopt` | write charter's files into the repo (never one of the repo's own), the scripts it lacks, and a workflow per job of ci.config.ts; --check writes nothing and fails if one differs |

In a project (`ts/examples-start`, `ts/examples-notes`, `ts/conformance`, `go/examples-notes`, `go/examples-start`, `go/examples-start-htmx`, `go/examples-start-datastar`, `go/conformance`), from its folder, or from here with `-w <folder>`:

| When | Run | What it does |
|---|---|---|
| To try the API on your machine | `npm run dev` | the API on your machine, as on Cloudflare (workerd), its local database migrated, until Ctrl-C; --native, in Go: go run . |
| After every change to the contract | `npm run spec` | write both specs (fern/openapi.json, fern/asyncapi.json) from the contract |
| When a caller needs an SDK; --prove after changing fern/generators.yml; --publish after a contract change, where the repo commits its Go SDK | `npm run sdk` | an SDK with Fern (Docker), by default the TypeScript one, into sdk/out; --prove: every client, proven in its language; --publish, in Go: the Go SDK into sdk/go |
| To put the API on Cloudflare | `npm run deploy` | REMOTE: the Worker on Cloudflare with its secrets (fnox exec -- npm run deploy), then its migrations |
| After every deploy | `npm run live-test` | REMOTE: the standard test and the project's tests against the deployed Worker |

The repo's own: `npm run release` (`node release.ts`).

Another release of charter-ts: `npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz`, then `npm run adopt` (Renovate opens that pull request). Every command, with what it takes: `npx charter-ts help`.
