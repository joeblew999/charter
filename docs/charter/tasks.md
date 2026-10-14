---
title: Tasks
nav_order: 4
parent: How this repo is kept
---

# Tasks

Written by `charter docs` from the repo's mise tasks (`charter docs-tasks`): don't edit, change the tasks, then `mise run docs:setup`.

Run one with `mise run <task>`; its arguments and flags go after `--`. `mise tasks` lists them, and `mise run <task> --help` shows one.

## From `mise.toml`

| Task | What it does |
|---|---|
| [`setup`](#setup) | Install every example's npm packages (its own mise run setup) |
| [`check`](#check) | Every LOCAL check (no Cloudflare account; needs Docker): the tool, the Go library, the TypeScript library, then each project's own check |
| [`projects:check`](#projectscheck) | LOCAL: mise run check in every project (examples/ and conformance/), one after the other, against the TypeScript library as built now |
| [`compare`](#compare) | REMOTE, writes test notes: the same bench against the TypeScript notes Worker and then the Go one, with writes and the CPU of each request: are they level? About two minutes |
| [`doctor`](#doctor) | Check what each example's tasks need: npm packages, Docker, Go, Rust (with a CLI) |
| [`sdk:clean`](#sdkclean) | Remove every example's generated SDKs and stop leftover WireMock containers |
| [`ci:scope`](#ciscope) | In the check workflow: which Windows and macOS jobs to run (all on a push; on a pull request, those for what it changes) |
| [`charter:check`](#chartercheck) | The tool's own checks (gofmt, vet, tests, with the test that holds the two notes examples to one surface), the generated workflows and docs config match their templates, and docs/ passes the lint |
| [`catalog`](#catalog) | REMOTE, read-only: the owner's charter repos, what each publishes, and the repos that pin it, as Markdown (\-\- -json for JSON, \-\- -page for a docs page) |
| [`workflows`](#workflows) | Write .github/workflows/ from the templates in cmd/charter/workflows (edit those, never the copies) |
| [`release`](#release) | REMOTE: cut a release of the repo from this machine: mise run release \-\- vX.Y.Z [-dry-run]. Runs check and dist, tags, pushes, makes the GitHub Release, runs release:publish and release:tags |
| [`dist`](#dist) | HEAVY: what the notes examples ship (their SDKs, specs and CLI for every OS), each into its own dist/: what mise run release builds before the tag |
| [`release:publish`](#releasepublish) | REMOTE on a version tag (vX.Y.Z): the TypeScript library's package, then the tool (GoReleaser), then the notes examples' dist/, onto its GitHub Release. Anywhere else: the same as a dry run |
| [`charter:release`](#charterrelease) | GoReleaser on the tool (.goreleaser.yaml). REMOTE on a version tag (vX.Y.Z): archives and checksums go to its GitHub Release. Anywhere else: a snapshot into dist/ |
| [`release:tags`](#releasetags) | REMOTE on a version tag (vX.Y.Z): tag the Go modules in subdirectories that others import (go/vX.Y.Z, examples/notes-go/sdk/go/vX.Y.Z). Anywhere else: a dry run |
| [`go:lint`](#golint) | gofmt and go vet of the Go library, for the host and for Wasm |
| [`go:test`](#gotest) | The Go library's tests: humaworkers, transport, hub, follow, humamcp, asyncapi, specfile, auth, ratelimit |
| [`go:check`](#gocheck) | LOCAL: the Go library's lint and tests |
| [`ts:setup`](#tssetup) | Install the TypeScript library's npm packages and build it (each TypeScript example's setup does it too) |
| [`ts:build`](#tsbuild) | Compile the TypeScript library into ts/dist/ (JavaScript and types), which its package exports: after changing ts/src, before an example uses it |
| [`ts:lint`](#tslint) | Typecheck the TypeScript library and its tests |
| [`ts:test`](#tstest) | The TypeScript library's tests: follow(), auth, ratelimit |
| [`ts:check`](#tscheck) | LOCAL: the TypeScript library's typecheck and tests |
| [`ts:dist`](#tsdist) | Build the TypeScript library and pack it into dist/charter-ts-X.Y.Z.tgz, X.Y.Z from the version tag (none: 0.0.0-dev): what a project installs from the release's URL |
| [`ts:release`](#tsrelease) | REMOTE on a version tag (vX.Y.Z): attach dist/* (what ts:dist wrote) to its GitHub Release, if it lacks them. Anywhere else: a dry run that lists dist/ |
| [`cloudflare:secrets`](#cloudflaresecrets) | REMOTE, once per repo: copy CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID from fnox into this repo's GitHub secrets, for the deploy workflow |

## From `tasks/repo`

| Task | What it does |
|---|---|
| [`docs:setup`](#docssetup) | Write charter's pages (docs/charter/: the rules every repo shares, using charter, writing docs, the tasks) and the docs site's files, where they are charter's (GitHub Pages renders the folder), and the pages docs/_generated.toml lists |
| [`docs:lint`](#docslint) | Check docs/ for what a program can check: front matter, links, the index, tasks and paths that don't exist |
| [`docs:check`](#docscheck) | Fail if the docs site's config or a generated page differs from what docs:setup writes, or docs/ fails the lint |
| [`docs:review`](#docsreview) | Have Claude (the claude command) bring docs/ up to date and into line with docs/charter/writing.md |
| [`docs:pages`](#docspages) | REMOTE, once per repo: turn on GitHub Pages for docs/ on main (GitHub renders the markdown itself) |
| [`repo`](#repo) | REMOTE: keep the repo in shape from charter.toml at its root: docs site and charter's pages, issue forms, labels, the workflow, description, homepage, topics, GitHub Pages; never a file of the repo's own |
| [`repo:check`](#repocheck) | REMOTE, read-only: fail if the repo differs from charter.toml and the tool's templates (charter repo -check) |
| [`issues`](#issues) | REMOTE, read-only: the open issues, newest first: what is reported and what is planned (a plan is an issue) |
| [`repo:ci`](#repoci) | Everything the charter workflow runs (charter.yml, which calls charter's repo-check), here: docs:check (the docs site and generated pages are fresh, docs/ passes the lint), repo:check (the repo is as charter.toml says), upstream:status. The workflow runs this one task and nothing else, so a pass here is a pass on GitHub. REMOTE, read-only |
| [`issue`](#issue) | Print the body to fill in for an issue of one kind, with that form's headings, and the gh command that files it with the form's labels: mise run issue \-\- bug (or feature, upstream, plan) |
| [`upstream:status`](#upstreamstatus) | REMOTE, read-only: the upstream issues the code works around (every `Upstream:` tag): CLOSED means that workaround can go |

## Each task

### `setup`

- **Usage:** `setup`

Install every example's npm packages (its own mise run setup)

### `check`

- **Usage:** `check`

Every LOCAL check (no Cloudflare account; needs Docker): the tool, the Go library, the TypeScript library, then each project's own check

### `projects:check`

- **Usage:** `projects:check`

LOCAL: mise run check in every project (examples/ and conformance/), one after the other, against the TypeScript library as built now

### `compare`

- **Usage:** `compare`

REMOTE, writes test notes: the same bench against the TypeScript notes Worker and then the Go one, with writes and the CPU of each request: are they level? About two minutes

### `doctor`

- **Usage:** `doctor`

Check what each example's tasks need: npm packages, Docker, Go, Rust (with a CLI)

### `sdk:clean`

- **Usage:** `sdk:clean`

Remove every example's generated SDKs and stop leftover WireMock containers

### `ci:scope`

- **Usage:** `ci:scope`

In the check workflow: which Windows and macOS jobs to run (all on a push; on a pull request, those for what it changes)

### `charter:check`

- **Usage:** `charter:check`

The tool's own checks (gofmt, vet, tests, with the test that holds the two notes examples to one surface), the generated workflows and docs config match their templates, and docs/ passes the lint

### `catalog`

- **Usage:** `catalog`

REMOTE, read-only: the owner's charter repos, what each publishes, and the repos that pin it, as Markdown (-- -json for JSON, -- -page for a docs page)

### `workflows`

- **Usage:** `workflows`

Write .github/workflows/ from the templates in cmd/charter/workflows (edit those, never the copies)

### `release`

- **Usage:** `release`

REMOTE: cut a release of the repo from this machine: mise run release -- vX.Y.Z [-dry-run]. Runs check and dist, tags, pushes, makes the GitHub Release, runs release:publish and release:tags

### `dist`

- **Usage:** `dist`

HEAVY: what the notes examples ship (their SDKs, specs and CLI for every OS), each into its own dist/: what mise run release builds before the tag

### `release:publish`

- **Usage:** `release:publish`

REMOTE on a version tag (vX.Y.Z): the TypeScript library's package, then the tool (GoReleaser), then the notes examples' dist/, onto its GitHub Release. Anywhere else: the same as a dry run

### `charter:release`

- **Usage:** `charter:release`

GoReleaser on the tool (.goreleaser.yaml). REMOTE on a version tag (vX.Y.Z): archives and checksums go to its GitHub Release. Anywhere else: a snapshot into dist/

### `release:tags`

- **Usage:** `release:tags`

REMOTE on a version tag (vX.Y.Z): tag the Go modules in subdirectories that others import (go/vX.Y.Z, examples/notes-go/sdk/go/vX.Y.Z). Anywhere else: a dry run

### `go:lint`

- **Usage:** `go:lint`

gofmt and go vet of the Go library, for the host and for Wasm

### `go:test`

- **Usage:** `go:test`

The Go library's tests: humaworkers, transport, hub, follow, humamcp, asyncapi, specfile, auth, ratelimit

### `go:check`

- **Usage:** `go:check`

LOCAL: the Go library's lint and tests

### `ts:setup`

- **Usage:** `ts:setup`

Install the TypeScript library's npm packages and build it (each TypeScript example's setup does it too)

### `ts:build`

- **Usage:** `ts:build`

Compile the TypeScript library into ts/dist/ (JavaScript and types), which its package exports: after changing ts/src, before an example uses it

### `ts:lint`

- **Usage:** `ts:lint`

Typecheck the TypeScript library and its tests

### `ts:test`

- **Usage:** `ts:test`

The TypeScript library's tests: follow(), auth, ratelimit

### `ts:check`

- **Usage:** `ts:check`

LOCAL: the TypeScript library's typecheck and tests

### `ts:dist`

- **Usage:** `ts:dist`

Build the TypeScript library and pack it into dist/charter-ts-X.Y.Z.tgz, X.Y.Z from the version tag (none: 0.0.0-dev): what a project installs from the release's URL

### `ts:release`

- **Usage:** `ts:release`

REMOTE on a version tag (vX.Y.Z): attach dist/* (what ts:dist wrote) to its GitHub Release, if it lacks them. Anywhere else: a dry run that lists dist/

### `cloudflare:secrets`

- **Usage:** `cloudflare:secrets`

REMOTE, once per repo: copy CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID from fnox into this repo's GitHub secrets, for the deploy workflow

### `docs:setup`

- **Usage:** `docs:setup`

Write charter's pages (docs/charter/: the rules every repo shares, using charter, writing docs, the tasks) and the docs site's files, where they are charter's (GitHub Pages renders the folder), and the pages docs/_generated.toml lists

### `docs:lint`

- **Usage:** `docs:lint`

Check docs/ for what a program can check: front matter, links, the index, tasks and paths that don't exist

### `docs:check`

- **Usage:** `docs:check`

Fail if the docs site's config or a generated page differs from what docs:setup writes, or docs/ fails the lint

### `docs:review`

- **Usage:** `docs:review`

Have Claude (the claude command) bring docs/ up to date and into line with docs/charter/writing.md

### `docs:pages`

- **Usage:** `docs:pages`

REMOTE, once per repo: turn on GitHub Pages for docs/ on main (GitHub renders the markdown itself)

### `repo`

- **Usage:** `repo`

REMOTE: keep the repo in shape from charter.toml at its root: docs site and charter's pages, issue forms, labels, the workflow, description, homepage, topics, GitHub Pages; never a file of the repo's own

### `repo:check`

- **Usage:** `repo:check`

REMOTE, read-only: fail if the repo differs from charter.toml and the tool's templates (charter repo -check)

### `issues`

- **Usage:** `issues`

REMOTE, read-only: the open issues, newest first: what is reported and what is planned (a plan is an issue)

### `repo:ci`

- **Usage:** `repo:ci`

Everything the charter workflow runs (charter.yml, which calls charter's repo-check), here: docs:check (the docs site and generated pages are fresh, docs/ passes the lint), repo:check (the repo is as charter.toml says), upstream:status. The workflow runs this one task and nothing else, so a pass here is a pass on GitHub. REMOTE, read-only

### `issue`

- **Usage:** `issue`

Print the body to fill in for an issue of one kind, with that form's headings, and the gh command that files it with the form's labels: mise run issue -- bug (or feature, upstream, plan)

### `upstream:status`

- **Usage:** `upstream:status`

REMOTE, read-only: the upstream issues the code works around (every `Upstream:` tag): CLOSED means that workaround can go
