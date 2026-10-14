---
title: Tasks
nav_order: 4
parent: How this repo is kept
---

# Tasks

Written by `charter adopt` from the repo's mise tasks (`charter docs-tasks`): don't edit, change the tasks, then `charter adopt`.

Run one with `mise run <task>`; its arguments and flags go after `--`. `mise tasks` lists them, and `mise run <task> --help` shows one.

## From `mise.toml`

| Task | What it does |
|---|---|
| [`setup`](#setup) | Install every example's npm packages (its own mise run setup) |
| [`ts:setup`](#tssetup) | Install the TypeScript library's npm packages and build it (each TypeScript example's setup does it too) |
| [`ts:build`](#tsbuild) | Compile the TypeScript library into ts/dist/ (JavaScript and types), which its package exports: after changing ts/src, before an example uses it |
| [`charter:check`](#chartercheck) | The tool's own checks (gofmt, vet, tests, with the test that holds the two notes examples to one surface), the generated workflows and docs config match their templates, and docs/ passes the lint |
| [`go:check`](#gocheck) | LOCAL: the Go library's lint (gofmt, go vet, for the host and for Wasm) and tests |
| [`ts:check`](#tscheck) | LOCAL: the TypeScript library's typecheck and tests |
| [`projects:check`](#projectscheck) | LOCAL: mise run check in every project (examples/ and conformance/), one after the other, against the TypeScript library as built now |
| [`check`](#check) | Every LOCAL check (no Cloudflare account; needs Docker): the tool, the Go library, the TypeScript library, then each project's own check |
| [`ci:scope`](#ciscope) | In the check workflow: which Windows and macOS jobs to run (all on a push; on a pull request, those for what it changes) |
| [`release`](#release) | REMOTE: cut a release of the repo from this machine: mise run release \-\- vX.Y.Z [-dry-run]. Runs check and dist, tags, pushes, makes the GitHub Release, runs release:publish and release:tags |
| [`dist`](#dist) | HEAVY: what the notes examples ship (their SDKs, specs and CLI for every OS), each into its own dist/: what mise run release builds before the tag |
| [`ts:dist`](#tsdist) | Build the TypeScript library and pack it into dist/charter-ts-X.Y.Z.tgz, X.Y.Z from the version tag (none: 0.0.0-dev): what a project installs from the release's URL |
| [`charter:release`](#charterrelease) | GoReleaser on the tool (.goreleaser.yaml). REMOTE on a version tag (vX.Y.Z): archives and checksums go to its GitHub Release. Anywhere else: a snapshot into dist/ |
| [`release:publish`](#releasepublish) | REMOTE on a version tag (vX.Y.Z): the TypeScript library's package, then the tool (GoReleaser), then the notes examples' dist/, onto its GitHub Release. Anywhere else: the same as a dry run |
| [`ts:release`](#tsrelease) | REMOTE on a version tag (vX.Y.Z): attach dist/* (what ts:dist wrote) to its GitHub Release, if it lacks them. Anywhere else: a dry run that lists dist/ |
| [`release:tags`](#releasetags) | REMOTE on a version tag (vX.Y.Z): tag the Go modules in subdirectories that others import (go/vX.Y.Z, examples/notes-go/sdk/go/vX.Y.Z). Anywhere else: a dry run |
| [`doctor`](#doctor) | Check what each example's tasks need: npm packages, Docker, Go, Rust (with a CLI) |
| [`sdk:clean`](#sdkclean) | Remove every example's generated SDKs and stop leftover WireMock containers |
| [`compare`](#compare) | REMOTE, writes test notes: the same bench against the TypeScript notes Worker and then the Go one, with writes and the CPU of each request: are they level? About two minutes |
| [`catalog`](#catalog) | REMOTE, read-only: the owner's charter repos, what each publishes, and the repos that pin it, as Markdown (\-\- -json for JSON, \-\- -page for a docs page) |
| [`cloudflare:secrets`](#cloudflaresecrets) | REMOTE, once per repo: copy CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID from fnox into this repo's GitHub secrets, for the deploy workflow |

## From `tasks/repo`

| Task | What it does |
|---|---|
| [`adopt`](#adopt) | Write charter's kit into this repo as this release has it: docs/charter/, the docs site's files, the issue forms, the workflows, AGENTS.md, CLAUDE.md, renovate.json; never a file of the repo's own |
| [`repo`](#repo) | REMOTE: keep the repo on GitHub in shape from charter.toml at its root: labels, description, homepage, topics, GitHub Pages. It writes no file: mise run adopt writes the kit |
| [`issues`](#issues) | REMOTE, read-only: the open issues, newest first: what is reported and what is planned (a plan is an issue) |
| [`issue`](#issue) | Print the body to fill in for an issue of one kind, with that form's headings, and the gh command that files it with the form's labels: mise run issue \-\- bug (or feature, upstream, plan) |
| [`build`](#build) | Build the Worker's Wasm with TinyGo into build/, tuned for Workers, with the library's Worker glue beside it; fails over 3 MB gzipped |
| [`run`](#run) | The API natively on API_PORT: no Cloudflare, an in-memory store |
| [`dev`](#dev) | The API under workerd on API_PORT (cf dev). First time: mise run migrate:local. After a Go change: mise run build |
| [`spec`](#spec) | Write both specs from the contract (api/contract.go) into fern/ |
| [`test`](#test) | Go tests: the API, the MCP endpoint, the contract's rules |
| [`migrate:local`](#migratelocal) | Apply migrations/ to the local D1 of a running mise run dev |
| [`docs:lint`](#docslint) | Check docs/ for what a program can check: front matter, links, the index, tasks and paths that don't exist |
| [`docs:check`](#docscheck) | Fail if a file of charter's kit or a generated page differs from what adopt writes, or docs/ fails the lint |
| [`repo:check`](#repocheck) | REMOTE, read-only: fail if the repo on GitHub differs from charter.toml (charter repo -check) |
| [`upstream:status`](#upstreamstatus) | REMOTE, read-only: the upstream issues the code works around (every `Upstream:` tag): CLOSED means that workaround can go |
| [`repo:ci`](#repoci) | Everything the charter workflow runs (charter.yml, which calls charter's repo-check), here: docs:check (the docs site and generated pages are fresh, docs/ passes the lint), repo:check (the repo is as charter.toml says), upstream:status. The workflow runs this one task and nothing else, so a pass here is a pass on GitHub. REMOTE, read-only |
| [`spec:check`](#speccheck) | Fail if a committed spec is stale against the contract |
| [`sdk:list`](#sdklist) | The SDK groups fern/generators.yml defines |
| [`sdk:gen`](#sdkgen) | Generate an SDK with Fern (Docker): mise run sdk:gen &lt;group&gt; -&gt; sdk/out/&lt;group&gt; |
| [`sdk:check`](#sdkcheck) | Prove a generated SDK works (Go: build, vet, tests against WireMock; TypeScript: typecheck): mise run sdk:check &lt;group&gt; |
| [`sdk:check-spec`](#sdkcheck-spec) | Validate the specs and Fern's settings with Fern |
| [`sdk:ready`](#sdkready) | Generate and build what the tests use, if missing or made from other specs (TypeScript and Go SDKs, the CLI if the project has one): mise run sdk:ready [group...] |
| [`sdk:publish`](#sdkpublish) | Generate and check the Go SDK, then copy its sources into sdk/go, the committed module another repo can go get (Docker). Commit the result |
| [`sdk:publish:check`](#sdkpublishcheck) | Fail if sdk/go is not what the committed specs generate now (Docker) |
| [`sdk:publish:fresh`](#sdkpublishfresh) | Fail if the specs changed since sdk/go was made from them (no Docker: it compares a hash). sdk:publish:check proves the content |
| [`sdk:cli:build`](#sdkclibuild) | HEAVY: build the generated Rust CLI in sdk/out/cli (a project with the cli group): mise run sdk:cli:build [-linux] |
| [`sdk:cli:smoke`](#sdkclismoke) | Run the CLI that dist/ holds for this machine with \-\-version (the release workflow does it on Windows), in a project with the cli group |
| [`sdk:dist`](#sdkdist) | Generate, check and archive what a release ships into dist/: Go and TypeScript SDK sources, the specs (Docker) |
| [`sdk:dist:cli`](#sdkdistcli) | HEAVY: generate the CLI (a project with the cli group) and build it into dist/ for darwin, linux, windows x amd64, arm64 (all six on a Mac; elsewhere all but darwin): mise run sdk:dist:cli [-target linux-arm64,...] |
| [`sdk:docs`](#sdkdocs) | Preview the API's reference site locally (fern docs dev): http://localhost:3030 |
| [`cloudflare:token`](#cloudflaretoken) | Fail unless CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID are set: what a deploy from GitHub needs (locally, cf auth login is enough) |
| [`migrate`](#migrate) | REMOTE: apply pending migrations/ to the Worker's D1 database |
| [`tail`](#tail) | REMOTE, read-only: stream the deployed Worker's live logs (requests, console output, exceptions) until Ctrl-C; \-\- -for 30s stops by itself, \-\- -worker &lt;name&gt; tails another |
| [`access:setup`](#accesssetup) | REMOTE, once and after changing who may log in: Cloudflare Access in front of the Worker (people log in with GitHub: mise run access:setup \-\- &lt;email&gt;...), and the Worker's ACCESS_TEAM_DOMAIN and ACCESS_AUD; the IDs in fnox. Idempotent |
| [`access:token`](#accesstoken) | REMOTE: a machine's own Access service token: mise run access:token \-\- create &lt;machine&gt; &lt;file\|fnox&gt;, list, revoke &lt;machine&gt; |
| [`access:delete`](#accessdelete) | REMOTE: delete the Worker's Access application and its machines' service tokens |
| [`spec:diff`](#specdiff) | Compare fern/openapi.json and fern/asyncapi.json with the previous release's (git tags) and fail on a breaking change, unless -tag is a major release: mise run spec:diff [\-\- -from vX.Y.Z -catalog catalog.json] |
| [`docs:review`](#docsreview) | Have Claude (the claude command) bring docs/ up to date and into line with docs/charter/writing.md |
| [`docs:pages`](#docspages) | REMOTE, once per repo: turn on GitHub Pages for docs/ on main (GitHub renders the markdown itself) |
| [`perf`](#perf) | REMOTE, writes test notes: the quick loop for performance work: build, deploy, then bench the new isolate from its first request (8 at once, then each operation). About two minutes |
| [`perf:try`](#perftry) | REMOTE: try a change on a scratch Worker of its own and delete it after: mise run perf:try \-\- -name &lt;experiment&gt; [-build '&lt;wasm-build flags&gt;'] [-keep]. Several can run at once |
| [`perf:clean`](#perfclean) | REMOTE: delete the scratch Workers and databases that perf:try runs left |

## Each task

### `setup`

- **Usage:** `setup`

Install every example's npm packages (its own mise run setup)

### `ts:setup`

- **Usage:** `ts:setup`

Install the TypeScript library's npm packages and build it (each TypeScript example's setup does it too)

### `ts:build`

- **Usage:** `ts:build`

Compile the TypeScript library into ts/dist/ (JavaScript and types), which its package exports: after changing ts/src, before an example uses it

### `charter:check`

- **Usage:** `charter:check`

The tool's own checks (gofmt, vet, tests, with the test that holds the two notes examples to one surface), the generated workflows and docs config match their templates, and docs/ passes the lint

### `go:check`

- **Usage:** `go:check`

LOCAL: the Go library's lint (gofmt, go vet, for the host and for Wasm) and tests

### `ts:check`

- **Usage:** `ts:check`

LOCAL: the TypeScript library's typecheck and tests

### `projects:check`

- **Usage:** `projects:check`

LOCAL: mise run check in every project (examples/ and conformance/), one after the other, against the TypeScript library as built now

### `check`

- **Usage:** `check`

Every LOCAL check (no Cloudflare account; needs Docker): the tool, the Go library, the TypeScript library, then each project's own check

### `ci:scope`

- **Usage:** `ci:scope`

In the check workflow: which Windows and macOS jobs to run (all on a push; on a pull request, those for what it changes)

### `release`

- **Usage:** `release`

REMOTE: cut a release of the repo from this machine: mise run release -- vX.Y.Z [-dry-run]. Runs check and dist, tags, pushes, makes the GitHub Release, runs release:publish and release:tags

### `dist`

- **Usage:** `dist`

HEAVY: what the notes examples ship (their SDKs, specs and CLI for every OS), each into its own dist/: what mise run release builds before the tag

### `ts:dist`

- **Usage:** `ts:dist`

Build the TypeScript library and pack it into dist/charter-ts-X.Y.Z.tgz, X.Y.Z from the version tag (none: 0.0.0-dev): what a project installs from the release's URL

### `charter:release`

- **Usage:** `charter:release`

GoReleaser on the tool (.goreleaser.yaml). REMOTE on a version tag (vX.Y.Z): archives and checksums go to its GitHub Release. Anywhere else: a snapshot into dist/

### `release:publish`

- **Usage:** `release:publish`

REMOTE on a version tag (vX.Y.Z): the TypeScript library's package, then the tool (GoReleaser), then the notes examples' dist/, onto its GitHub Release. Anywhere else: the same as a dry run

### `ts:release`

- **Usage:** `ts:release`

REMOTE on a version tag (vX.Y.Z): attach dist/* (what ts:dist wrote) to its GitHub Release, if it lacks them. Anywhere else: a dry run that lists dist/

### `release:tags`

- **Usage:** `release:tags`

REMOTE on a version tag (vX.Y.Z): tag the Go modules in subdirectories that others import (go/vX.Y.Z, examples/notes-go/sdk/go/vX.Y.Z). Anywhere else: a dry run

### `doctor`

- **Usage:** `doctor`

Check what each example's tasks need: npm packages, Docker, Go, Rust (with a CLI)

### `sdk:clean`

- **Usage:** `sdk:clean`

Remove every example's generated SDKs and stop leftover WireMock containers

### `compare`

- **Usage:** `compare`

REMOTE, writes test notes: the same bench against the TypeScript notes Worker and then the Go one, with writes and the CPU of each request: are they level? About two minutes

### `catalog`

- **Usage:** `catalog`

REMOTE, read-only: the owner's charter repos, what each publishes, and the repos that pin it, as Markdown (-- -json for JSON, -- -page for a docs page)

### `cloudflare:secrets`

- **Usage:** `cloudflare:secrets`

REMOTE, once per repo: copy CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID from fnox into this repo's GitHub secrets, for the deploy workflow

### `adopt`

- **Usage:** `adopt`

Write charter's kit into this repo as this release has it: docs/charter/, the docs site's files, the issue forms, the workflows, AGENTS.md, CLAUDE.md, renovate.json; never a file of the repo's own

### `repo`

- **Usage:** `repo`

REMOTE: keep the repo on GitHub in shape from charter.toml at its root: labels, description, homepage, topics, GitHub Pages. It writes no file: mise run adopt writes the kit

### `issues`

- **Usage:** `issues`

REMOTE, read-only: the open issues, newest first: what is reported and what is planned (a plan is an issue)

### `issue`

- **Usage:** `issue`

Print the body to fill in for an issue of one kind, with that form's headings, and the gh command that files it with the form's labels: mise run issue -- bug (or feature, upstream, plan)

### `build`

- **Usage:** `build`

Build the Worker's Wasm with TinyGo into build/, tuned for Workers, with the library's Worker glue beside it; fails over 3 MB gzipped

### `run`

- **Usage:** `run`

The API natively on API_PORT: no Cloudflare, an in-memory store

### `dev`

- **Usage:** `dev`

The API under workerd on API_PORT (cf dev). First time: mise run migrate:local. After a Go change: mise run build

### `spec`

- **Usage:** `spec`

Write both specs from the contract (api/contract.go) into fern/

### `test`

- **Usage:** `test`

Go tests: the API, the MCP endpoint, the contract's rules

### `migrate:local`

- **Usage:** `migrate:local`

Apply migrations/ to the local D1 of a running mise run dev

### `docs:lint`

- **Usage:** `docs:lint`

Check docs/ for what a program can check: front matter, links, the index, tasks and paths that don't exist

### `docs:check`

- **Usage:** `docs:check`

Fail if a file of charter's kit or a generated page differs from what adopt writes, or docs/ fails the lint

### `repo:check`

- **Usage:** `repo:check`

REMOTE, read-only: fail if the repo on GitHub differs from charter.toml (charter repo -check)

### `upstream:status`

- **Usage:** `upstream:status`

REMOTE, read-only: the upstream issues the code works around (every `Upstream:` tag): CLOSED means that workaround can go

### `repo:ci`

- **Usage:** `repo:ci`

Everything the charter workflow runs (charter.yml, which calls charter's repo-check), here: docs:check (the docs site and generated pages are fresh, docs/ passes the lint), repo:check (the repo is as charter.toml says), upstream:status. The workflow runs this one task and nothing else, so a pass here is a pass on GitHub. REMOTE, read-only

### `spec:check`

- **Usage:** `spec:check`

Fail if a committed spec is stale against the contract

### `sdk:list`

- **Usage:** `sdk:list`

The SDK groups fern/generators.yml defines

### `sdk:gen`

- **Usage:** `sdk:gen`

Generate an SDK with Fern (Docker): mise run sdk:gen &lt;group> -> sdk/out/&lt;group>

### `sdk:check`

- **Usage:** `sdk:check`

Prove a generated SDK works (Go: build, vet, tests against WireMock; TypeScript: typecheck): mise run sdk:check &lt;group>

### `sdk:check-spec`

- **Usage:** `sdk:check-spec`

Validate the specs and Fern's settings with Fern

### `sdk:ready`

- **Usage:** `sdk:ready`

Generate and build what the tests use, if missing or made from other specs (TypeScript and Go SDKs, the CLI if the project has one): mise run sdk:ready [group...]

### `sdk:publish`

- **Usage:** `sdk:publish`

Generate and check the Go SDK, then copy its sources into sdk/go, the committed module another repo can go get (Docker). Commit the result

### `sdk:publish:check`

- **Usage:** `sdk:publish:check`

Fail if sdk/go is not what the committed specs generate now (Docker)

### `sdk:publish:fresh`

- **Usage:** `sdk:publish:fresh`

Fail if the specs changed since sdk/go was made from them (no Docker: it compares a hash). sdk:publish:check proves the content

### `sdk:cli:build`

- **Usage:** `sdk:cli:build`

HEAVY: build the generated Rust CLI in sdk/out/cli (a project with the cli group): mise run sdk:cli:build [-linux]

### `sdk:cli:smoke`

- **Usage:** `sdk:cli:smoke`

Run the CLI that dist/ holds for this machine with --version (the release workflow does it on Windows), in a project with the cli group

### `sdk:dist`

- **Usage:** `sdk:dist`

Generate, check and archive what a release ships into dist/: Go and TypeScript SDK sources, the specs (Docker)

### `sdk:dist:cli`

- **Usage:** `sdk:dist:cli`

HEAVY: generate the CLI (a project with the cli group) and build it into dist/ for darwin, linux, windows x amd64, arm64 (all six on a Mac; elsewhere all but darwin): mise run sdk:dist:cli [-target linux-arm64,...]

### `sdk:docs`

- **Usage:** `sdk:docs`

Preview the API's reference site locally (fern docs dev): http://localhost:3030

### `cloudflare:token`

- **Usage:** `cloudflare:token`

Fail unless CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID are set: what a deploy from GitHub needs (locally, cf auth login is enough)

### `migrate`

- **Usage:** `migrate`

REMOTE: apply pending migrations/ to the Worker's D1 database

### `tail`

- **Usage:** `tail`

REMOTE, read-only: stream the deployed Worker's live logs (requests, console output, exceptions) until Ctrl-C; -- -for 30s stops by itself, -- -worker &lt;name> tails another

### `access:setup`

- **Usage:** `access:setup`

REMOTE, once and after changing who may log in: Cloudflare Access in front of the Worker (people log in with GitHub: mise run access:setup -- &lt;email>...), and the Worker's ACCESS_TEAM_DOMAIN and ACCESS_AUD; the IDs in fnox. Idempotent

### `access:token`

- **Usage:** `access:token`

REMOTE: a machine's own Access service token: mise run access:token -- create &lt;machine> &lt;file|fnox>, list, revoke &lt;machine>

### `access:delete`

- **Usage:** `access:delete`

REMOTE: delete the Worker's Access application and its machines' service tokens

### `spec:diff`

- **Usage:** `spec:diff`

Compare fern/openapi.json and fern/asyncapi.json with the previous release's (git tags) and fail on a breaking change, unless -tag is a major release: mise run spec:diff [-- -from vX.Y.Z -catalog catalog.json]

### `docs:review`

- **Usage:** `docs:review`

Have Claude (the claude command) bring docs/ up to date and into line with docs/charter/writing.md

### `docs:pages`

- **Usage:** `docs:pages`

REMOTE, once per repo: turn on GitHub Pages for docs/ on main (GitHub renders the markdown itself)

### `perf`

- **Usage:** `perf`

REMOTE, writes test notes: the quick loop for performance work: build, deploy, then bench the new isolate from its first request (8 at once, then each operation). About two minutes

### `perf:try`

- **Usage:** `perf:try`

REMOTE: try a change on a scratch Worker of its own and delete it after: mise run perf:try -- -name &lt;experiment> [-build '&lt;wasm-build flags>'] [-keep]. Several can run at once

### `perf:clean`

- **Usage:** `perf:clean`

REMOTE: delete the scratch Workers and databases that perf:try runs left
