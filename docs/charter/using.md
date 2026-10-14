---
title: Using charter
nav_order: 2
parent: How this repo is kept
---

# Using charter

Written by `charter docs` from the tool itself (the same page in every repo that uses it, at the release its `mise.toml` pins): don't edit it here, change the commands in `cmd/charter/` in [charter](https://github.com/joeblew999/charter).

charter is the tool this repo is kept with. mise installs the release `mise.toml` pins, and puts it on the path of every task.

| To | Run |
|---|---|
| See what there is to run | `mise tasks` |
| Run a task | `mise run <task>` |
| Run the tool | `charter <command>`; from anywhere, `mise x github:joeblew999/charter -- charter <command>` |
| Move to another release | `mise x github:joeblew999/charter@X.Y.Z -- charter upgrade`: the pin and the includes together |
| Read more | [charter's docs](https://joeblew999.github.io/charter/); for machines, [llms.txt](https://joeblew999.github.io/charter/llms.txt) |

## The commands

Those marked * work in any repo; the others in a project, a folder with a `mise.toml` beside a `fern/` folder.

| Command | What it does |
|---|---|
| `access setup [email...] \| token create <machine> <file\|fnox> \| token list \| token revoke <machine> \| delete` | REMOTE: Cloudflare Access in front of the Worker (API_URL): people log in with GitHub, each machine has a service token of its own. Needs CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_GITHUB_IDP_ID (the environment, or fnox). Secrets are never printed |
| `adopt [-description <text>] [-from <checkout>]` * | take the repo you are in to the charter way, writing only what it lacks and never a file of its own: charter.toml, AGENTS.md, CLAUDE.md, docs/README.md, docs/rules.md, charter's pages (docs/charter/), the docs site, and a mise.toml with the tool's pin and the tasks any repo takes (tasks/repo), or the lines to add to the one it has |
| `bench [-n 20] [-spec <file or url>] [-write] [-cpu] [-warm 45s] [-worker <name>] [-header 'Name: value']... [<url>]` * | time every operation of an API from its OpenAPI spec: wall time, and with -cpu the CPU time Cloudflare measured |
| `catalog [-owner <login>] [-json] [-page]` * | REMOTE, read-only: the owner's charter repos, what each publishes, and the repos that pin it (their mise.toml, go.mod, package.json), as Markdown or -json; -page adds a docs page's front matter |
| `ci-scope` * | in the check workflow: which Windows and macOS jobs to run, all of them on a push, on a pull request those for what it changes |
| `cli-build [-linux]` | HEAVY: build the generated Rust CLI (sdk/out/cli), natively or for Linux in Docker |
| `cli-smoke` | run the CLI that dist/ holds for this machine with --version: proves a binary built elsewhere starts here |
| `deploy [cf deploy flags]` | deploy the Worker with the secrets WORKER_SECRETS names, and those of WORKER_OPTIONAL_SECRETS that are set, from the environment (values are never printed), then apply pending migrations |
| `dist [-target <os>-<arch>,...]` | build everything the project ships into an emptied dist/: its SDKs and specs (dist-sdk), then its CLI if it has one (dist-cli) |
| `dist-cli [-target <os>-<arch>,...]` | HEAVY: generate the project's Fern CLI and build it into dist/ for darwin, linux and windows on amd64 and arm64: every target this machine builds (a Mac builds all six) |
| `dist-sdk` | generate, check and archive the project's SDKs and its specs into dist/ (Docker) |
| `dist-ts [-tag vX.Y.Z]` * | build the TypeScript library (ts/) and pack it, versioned as the tag, into dist/charter-ts-X.Y.Z.tgz: the package a project installs from the GitHub Release's URL (no tag: version 0.0.0-dev) |
| `docs [-into <repo dir>] [-check]` * | write charter's pages (docs/charter/: the rules every repo shares, using charter, writing docs, the tasks) and the docs site's files into a repo's docs/ folder (GitHub Pages renders it; any repo, same look), then the pages docs/_generated.toml lists; a file without charter's mark is kept; -check fails if one is stale |
| `docs-lint [-into <repo dir>]` * | check docs/ for what can be checked without reading: front matter, links, the index, tasks and paths that don't exist |
| `docs-review [-print]` * | have Claude bring docs/ up to date and into line with docs/charter/writing.md (-print: only show the prompt) |
| `docs-tasks [-only <source,...>] [-not <source,...>] [-title <text>]` * | print the repo's mise tasks as a docs page, for docs/_generated.toml: grouped by the file or include they come from, in the order they are written, each with its description, arguments and flags; -only and -not choose the sources |
| `doctor` | check what the project's tasks need: npm installs, Docker, Go, Rust (with a CLI), leftover containers |
| `each [-only <name,...>] <task> [args]` * | run a mise task in every project below this folder that has it (a repo that holds several projects, as this tool's own does in examples/) |
| `exec [-env NAME=VALUE]... [-quiet] [-secrets] <program> [args]` * | run a program with these variables set; one that an npm package of the project installed (cf, fern, tsc) is found in node_modules/.bin. -quiet: its output only if it fails. -secrets: with the secrets WORKER_SECRETS names, from fnox unless they are set already (as on GitHub) |
| `files` * | list every file in this repo that charter wrote, by the mark in its first lines: charter's (and whether it is what this release writes) or yours since charter started it |
| `github-secrets <NAME>...` * | copy environment variables into this repo's GitHub Actions secrets (values are never printed) |
| `harness-deploy` | REMOTE: deploy the project's Worker twice: as itself and, --mode api, as <worker>-api |
| `harness-sync` | copy the project's compiled TypeScript SDK (sdk/out/typescript-dist) into its Worker (src/client), generating it first if the specs are newer |
| `harness-test [-remote]` | run the SDK test inside the project's Worker: under cf dev, or the deployed one (API_URL, API_MOCK_URL) |
| `issue <bug\|feature\|upstream\|plan>` * | print an issue body with the headings of that issue form, for gh issue create --body-file |
| `labels` * | REMOTE: create or update this repo's GitHub labels from the tool's own list (cmd/charter/labels.tsv), and remove GitHub's default ones that are unused |
| `lint [-vet <packages>] [-wasm <packages>] <file or folder>...` * | fail if gofmt would change one of the files (a * pattern is matched here, not by the shell), then go vet the packages (default ./...), and those of -wasm for Wasm (GOOS=js GOARCH=wasm) |
| `migrate [-worker <name>]` | REMOTE: apply pending migrations/ to the Worker's D1 database (<worker>-db; the Worker: API_URL's) |
| `migrate-local [-port <port>]` | apply migrations/*.sql to a running dev server's local D1, each once (the port: API_PORT) |
| `need-env <NAME>...` * | fail, naming them, unless these environment variables are set (the secrets a workflow needs) |
| `new -name <name> [-lang go\|ts] [-empty [-ui htmx\|datastar] [-cli]] [-module <go module>] [-subdomain <workers.dev subdomain>] [-into <dir>] [-from <checkout>]` * | create a new API project: a copy of a tested example (examples/notes-go in the charter repo; -empty: examples/start-go; -empty -ui htmx: examples/start-htmx; -empty -ui datastar: examples/start-datastar) under your name; -cli adds the notes example's CLI to an -empty one |
| `perf -name <experiment> [-build '<wasm-build flags>'] [-prebuilt] [-keep] [-- <bench flags>]` | REMOTE: build, deploy to a scratch Worker <worker>-perf-<experiment>, bench it from its first request, delete it (-keep: leave it) |
| `perf-clean [-all]` | REMOTE: delete every scratch Worker <worker>-perf-* and its database that a perf run left |
| `publish [-tag vX.Y.Z]` * | create the tag's GitHub Release if it has none, attach the files in dist/ it lacks, and write SHA256SUMS of everything on it (no tag: a dry run that lists dist/ and prints the notes) |
| `release vX.Y.Z [-dry-run] [-prerelease] [-notes-footer <text or file>]` * | cut a release from this machine: check the repo is clean, is the default branch on GitHub and has no failed workflow there, run its setup, check, spec:diff and dist tasks, tag, push the tag, make the GitHub Release (notes: the commits since the last tag) and run its release:publish and release:tags tasks; -dry-run changes nothing |
| `release-tags [-tag vX.Y.Z] <module dir>...` * | tag each Go module in a subdirectory <path in the repo>/vX.Y.Z at the tag's commit (no tag: a dry run) |
| `release-tool [-tag vX.Y.Z]` * | GoReleaser on this tool, in its own repo: on a version tag, archives and checksums go to its GitHub Release (if they are not there yet); no tag: a snapshot into dist/ |
| `repo [-check]` * | keep the repo in shape from charter.toml at its root: docs site, issue forms, labels, workflows, description, homepage, topics, Pages; -check changes nothing and fails on drift |
| `sdk-check <group>` | prove a generated SDK works (sdk/out/<group>): Go = build, vet, tests against WireMock; TypeScript = typecheck |
| `sdk-clean` | remove generated SDKs (sdk/out) and stop leftover WireMock containers |
| `sdk-gen <group>` | generate one SDK with Fern (Docker) into sdk/out/<group> |
| `sdk-list` | the SDK groups the project defines (fern/generators.yml) |
| `sdk-publish [-check [-quick]] [-into <dir>]` | generate the Go SDK, check it, and copy its sources into the committed folder another repo can go get (sdk/go); -check: fail if that copy is stale (Docker) |
| `sdk-ready [group...]` | generate and build what the tests use, if missing: TypeScript and Go SDKs, the Fern CLI if the project has one (or only the groups named) |
| `size -max <bytes> <file>` * | fail if <file>, gzipped, is larger than <bytes> |
| `spec-diff [-from vX.Y.Z] [-to vX.Y.Z] [-tag vX.Y.Z] [-catalog <file or url>]` | compare fern/openapi.json and fern/asyncapi.json with the previous release's (git tags) and fail on a breaking change unless -tag is a major release; -catalog names the consumers it affects |
| `tail [-worker <name>] [-for <duration>]` | REMOTE, read-only: stream the deployed Worker's live logs (each request, its console output and exceptions) until Ctrl-C or -for ends. The Worker: API_URL's, unless -worker names another. Needs CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID (the environment, or fnox) |
| `unchanged -files <pattern,...> <program> [args]` | run a program that writes generated files the project commits (gsx's *.x.go), and fail, naming them, if it changed, added or removed one: the committed ones were stale |
| `upgrade [vX.Y.Z]` * | move the repo you are in to a release of charter, by default the one running: in mise.toml the tool's pin and every include of charter's tasks together, @charter/ts in package.json, and AGENTS.md and CLAUDE.md written again; then mise run repo writes charter's own places as it has them. Run it from the release you want: mise x github:joeblew999/charter@X.Y.Z -- charter upgrade |
| `upstream` * | the upstream issues the code under this folder works around (every `Upstream:` tag) and their state |
| `version` * | the release this tool is, which is what new pins a project to |
| `wasm-build [-heap 8] [-opt z] [-stack 128kb] [-max 3000000] [-plain]` | build the project's Go Worker into build/ with TinyGo, tuned for Workers (see docs: Huma on Cloudflare Workers), and write the Go library's Worker glue beside it; -plain: TinyGo as it is |
| `with-server -url <url> [-env NAME=VALUE]... -start <cmd> [-show] -run <cmd>...` * | start a server with these variables, wait for <url>, run the commands, stop the server and what it started. A command is a program and its arguments, not a shell line. {port} in any of them is a free port, {port2} another. A command's output is shown only if it fails; -show streams it |
| `workflows [-check] [-into <repo dir>]` * | write a project's .github: the workflows (check, deploy, sdk-check, release) and charter's issue forms (charter-*.yml), each where the file there is missing or charter's; -check fails if one differs |
