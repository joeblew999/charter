---
title: Using charter-ts
nav_order: 2
parent: How this repo is kept
---

# Using charter-ts

Written by `charter-ts adopt` from charter-ts itself (the same page in every repo that uses it, at the version its package.json installs): don't edit it here, change the commands in `charter-ts/src/steps.ts` in [charter](https://github.com/joeblew999/charter).

charter-ts is the tool this repo is kept with: package.json installs it, and its npm scripts run it.

| To | Run |
|---|---|
| See what there is to run | `npm run`; in order, with what each does: [Tasks](tasks.md) |
| Run a step | `npm run <script>` |
| Run charter-ts | `npx charter-ts <command>` |
| Move to another release | `npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz`, then `npm run adopt` |
| Read more | [charter's docs](https://joeblew999.github.io/charter/); for machines, [llms.txt](https://joeblew999.github.io/charter/llms.txt) |

## The commands

Those marked * work in any repo; the others in a project, a folder with a `charter.config.ts`.

| Command | What it does |
|---|---|
| `adopt [--description <text>] [--check]` * | write charter's kit into the repo you are in, charter's own included: docs/charter/, the docs site's files, the issue forms, the job runner, AGENTS.md, CLAUDE.md, renovate.json, never a file of the repo's own; what it lacks of package.json (charter-ts, the scripts), docs/README.md, docs/rules.md and ci.config.ts (with one job, repo); then the workflows from ci.config.ts. Run again, it brings the kit to this version; --check writes nothing and fails if the kit or a workflow differs |
| `repo [--check]` * | keep the repo on GitHub in shape from package.json: labels, description, homepage, topics (its keywords), GitHub Pages; --check changes nothing and fails on drift |
| `issues` * | REMOTE, read-only: the open issues, newest first: what is reported and what is planned (a plan is an issue) |
| `issue <bug\|feature\|upstream\|plan>` * | print an issue body with the headings of that issue form, and the gh command that files it |
| `docs:lint` * | check docs/ for what a program can check: front matter, links, the index, scripts, tasks and paths that don't exist |
| `upstream:status` * | REMOTE, read-only: the upstream issues the code works around (every `Upstream:` tag) and their state: CLOSED means that workaround can go |
| `docs:check` * | fail if a file of charter's kit or a workflow is not what this version writes, or docs/ fails the lint |
| `repo:ci` * | REMOTE, read-only: what the repo job runs (ci.config.ts), here: docs:check, repo --check, upstream:status. A pass here is a pass on GitHub |
| `docs:review [--print]` * | have Claude (the claude command) bring docs/ up to date and into line with docs/charter/writing.md; --print only shows the prompt |
| `new <name> [--from start\|notes] [--subdomain <workers.dev subdomain>] [--into <dir>]` * | a new project: a copy of a tested example under your name, with its GitHub workflow |
| `build` | Go: build the Worker into build/ with TinyGo, tuned for Workers, with the Go library's Worker glue beside it; fails over the size Workers Free allows |
| `run` | Go: the API natively (go run .) on the config's port: no Cloudflare, an in-memory store |
| `dev` | the API under workerd (cf dev) on the config's port, with the throwaway secrets (a Go Worker built first), its local database migrated once it answers; until Ctrl-C |
| `migrate:local` | apply migrations/*.sql, each once, to the local database of a running dev |
| `lint` | Go: gofmt, go vet for the host and for Wasm, gsx's output and the Go SDK fresh. TypeScript: typecheck the Worker (tsconfig.json, with cf's types) and what runs in Node: the config and the tests (tsconfig.node.json) |
| `check [--fresh]` | every local check, in order, with no Cloudflare account, and how long each took. Go: lint, go test, the specs, Fern's check, the tests natively and under workerd. TypeScript: lint, the specs, Fern's check, test/*.test.ts, the tests under workerd. A TinyGo build and Fern's check done before for the same sources come from charter's cache; --fresh does them again |
| `spec` | write both specs from the contract into fern/ |
| `spec:check` | fail if a committed spec is stale against the contract |
| `test:native` | Go: the API natively (go run .) on a free port, and the config's native tests against it |
| `bundle` | the Worker as a deploy would upload it (cf build, a Go Worker built first), and how big it is, gzipped, against its limit (charter.config.ts: bundleLimit; Workers Free's 3 MB by default); fails over it |
| `test:workerd` | the Worker under workerd (cf dev) on a free port (a Go Worker built first), its local database migrated, and the config's tests against it |
| `sdk [group]` | generate a group of fern/generators.yml with Fern (Docker) into sdk/out/<group>; by default the TypeScript SDK |
| `go:check <folder>` * | a Go module's check, for one that is not a project (go/lib): gofmt, go vet for the host and for Wasm, go test |
| `clients [group...]` | every client fern/generators.yml declares (or the groups named), generated with Fern (Docker) and proven in its language: Go built, vetted and tested against WireMock, TypeScript typechecked, the Rust CLI built; then a committed Go SDK checked against the specs |
| `ci [<job> [<project>]]` * | a job of ci.config.ts, as GitHub runs it: in one project, or in each; with no job, every job GitHub runs on a push or a pull request, one after another |
| `ci:remote <job> [<project>]` * | REMOTE: start a job's workflow on GitHub, on this branch (pushed), and say where the run is; it does not wait for it |
| `ci:write [--check]` * | write a workflow per job (.github/workflows/self-<job>.yml) from ci.config.ts: each calls the job runner (charter-ci.yml, from the kit), which installs the tools with mise and runs npx charter-ts ci <job>; --check writes nothing and fails if one differs |
| `sdk:publish [--check]` | Go: generate the Go SDK (Docker), prove it (build, vet, tests against WireMock) and copy its sources into sdk/go, the committed module another repo can go get; --check fails if sdk/go is not what the specs generate now |
| `deploy` | REMOTE: deploy the Worker (a Go Worker built first) with its secrets from the environment (fnox exec -- npm run deploy), then apply its migrations |
| `live-test` | REMOTE: the config's live tests against the deployed Worker (with the secrets: fnox exec -- npm run live-test) |
| `version` * | the version of charter-ts this is |
