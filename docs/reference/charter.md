---
title: The charter command
nav_order: 2
parent: Reference
---

# The charter command: every command and flag

The tool the tasks run (`cmd/charter/`). `charter help` lists the commands. A command works on the project it is run in: the nearest folder upwards with a `mise.toml` beside a `fern/` folder. Commands marked * also run outside one, and those under [Any repo](#any-repo) are for a repo that is not a project as much as for one. Flags come before arguments.

| Where | How to run it |
|---|---|
| Anywhere | `mise x github:joeblew999/charter -- charter <command>`: the newest release's binary |
| Anywhere, with Go | `go run github.com/joeblew999/charter/cmd/charter@latest <command>` |
| In a project | `charter <command>`: `mise.toml` pins its release (`"github:joeblew999/charter"`) |
| In this repo | `go run ./cmd/charter <command>` |

## Make a project

| Command | Flags | What it does |
|---|---|---|
| `new` * | `-name <name>` (required), `-lang go` or `ts` (default `go`), `-empty`, `-ui htmx` or `datastar` (with `-empty`, in Go), `-cli` (with `-empty`), `-module <go module>` (default `github.com/<gh login>/<name>`), `-subdomain <workers.dev subdomain>` (default a placeholder), `-into <empty dir>` (default `./<name>`), `-from <checkout>` | Copies `examples/notes-go/` (without `sdk/go/`), or with `-lang ts` `examples/notes-ts/` and the tests it shares, under your name; with `-empty`, `examples/start-go/` or `examples/start-ts/` (one route, `GET /api/hello`); with `-empty -ui htmx` or `datastar`, `examples/start-htmx/` or `examples/start-datastar/` (and [pages](../guides/pages.md)); a start project has no CLI, and `-cli` adds the notes example's `cli` group and its Rust, zig and cargo-zigbuild pins ([Add the CLI](../guides/sdks.md#add-the-cli)); adds docs and workflows; pins the tool and the library to its own release (with `-from`, to the checkout). `-lang ts` needs Node |
| `version` * | | The release the tool is |

## Build

| Command | Flags | What it does |
|---|---|---|
| `wasm-build` | below | Writes the Worker glue into `build/`, patches a copy of TinyGo's runtime once per TinyGo version, builds `build/app.wasm` and checks its size |
| `size` * | `-max <bytes> <file>` | The size check on its own |
| `unchanged` | `-files <pattern,...>`, then `<program> [args]` | Runs a generator whose output the project commits (gsx's `*.x.go`) and fails, naming them, if it changed, added or removed a matching file; what it wrote stays |

### wasm-build

| Flag | Meaning | Default |
|---|---|---|
| `-heap <MB>` | The starting heap; 0 is TinyGo's own | 8 |
| `-stack <size>` | The stack per goroutine | `128kb` |
| `-opt <level>` | TinyGo's optimisation level: `z`, `s`, `1`, `2` | `z` |
| `-max <bytes>` | Fail if the Wasm, gzipped, is larger | 3000000 |
| `-tinygo <which>` | `pinned` (the one tested, from mise) or `system` (on the path, untested) | `pinned` |
| `-plain` | TinyGo as it is: no patch, no starting heap | off |

| Patch | Upstream |
|---|---|
| `finalizerGCThreshold` 32 becomes 0 (`src/runtime/gc_finalizer.go`) | tinygo-org/tinygo#5800 |
| A finished goroutine's stack is kept for the next (`src/internal/task/task_asyncify.go`); left out with a TinyGo that has the fix | tinygo-org/tinygo#5801 |

## Run and test

| Command | Flags | What it does |
|---|---|---|
| `with-server` * | `-url <url>`, `-env NAME=VALUE`, `-start <cmd>`, `-run <cmd>`, `-show` | Starts a server, waits for the URL, runs the commands, stops the server and what it started. `{port}`, `{port2}`: free ports. Output only on failure, or with `-show` |
| `exec` * | `-env NAME=VALUE`, `-quiet`, `-secrets`, then `<program> [args]` | Runs a program with those variables; `-quiet` shows its output only if it fails; `-secrets` adds the secrets `WORKER_SECRETS` names, from fnox unless already set |
| `lint` * | `-vet <packages>`, `-wasm <packages>`, then `<file or folder>...` | Fails if `gofmt` would change a file, then `go vet` for the host and for Wasm |
| `migrate-local` | `-port <port>`, `-worker <name>` | Applies `migrations/*.sql` to a running dev server's local D1 |
| `deploy` | `cf deploy` flags | REMOTE. Deploys the Worker with the secrets `WORKER_SECRETS` names, and those of `WORKER_OPTIONAL_SECRETS` that are set, from the environment through a private file (never printed), then `migrate` |
| `migrate` | `-worker <name>` | REMOTE. Applies pending migrations to `<worker>-db` |
| `tail` | `-worker <name>` (default: `API_URL`'s), `-for <duration>` (default: until Ctrl-C) | REMOTE, read-only. Streams the Worker's live logs with Cloudflare's tail API (no wrangler): a line per request (UTC time, method, URL, status, outcome), then its console output and exceptions. Deletes the tail when it stops. Needs `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` (the environment, or fnox) |
| `access` | `setup [email...]`, `token create <machine> <file\|fnox>`, `token list`, `token revoke <machine>`, `delete` | REMOTE. Cloudflare Access in front of the Worker (`API_URL`): people log in with GitHub, each machine has a service token of its own; the Worker's `ACCESS_TEAM_DOMAIN` and `ACCESS_AUD`, and the IDs in fnox ([Auth](../guides/auth.md#cloudflare-access)). Needs `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `CF_ACCESS_TEAM_DOMAIN`, `CF_ACCESS_GITHUB_IDP_ID` |
| `doctor` | | Says what the project's tasks need and lack: Rust and cargo-zigbuild only with a CLI; warns when the CLI's pins in `mise.toml` and the `cli` group disagree |
| `harness-sync`, `harness-test`, `harness-deploy` | `harness-test -remote` | For a Worker that imports its own generated SDK (`conformance/showcase-ts/`): copy it in, test it under `cf dev` or deployed, deploy it twice |

`with-server` and `exec` run a program an npm package of the project installs (`cf`, `fern`) from `node_modules/.bin`, never from the path.

## Measure

| Command | Flags | What it does |
|---|---|---|
| `bench` * | below, then `[<url>]` (default `API_URL`) | Times every GET operation with examples in the spec, plus one missing path |
| `perf` | `-name <experiment>`, `-build '<wasm-build flags>'`, `-prebuilt`, `-keep`, then `-- <bench flags>` | REMOTE. Builds, deploys to `<worker>-perf-<experiment>`, benches it from its first request, deletes it |
| `perf-clean` | `-all` | REMOTE. Deletes scratch Workers and databases; without `-all`, not those of the last 20 minutes |

### bench

| Flag | Meaning | Default |
|---|---|---|
| `-n <count>` | Requests per operation, after 3 warm-up ones | 20 |
| `-write` | Also POST, PUT, PATCH and DELETE | off |
| `-cpu` | Also Cloudflare's CPU time, median and p99 (needs the two `CLOUDFLARE_` variables) | off |
| `-each` | Also every request's CPU and runtime kind. Implies `-cpu` | off |
| `-burst <k>` | First send `k` requests at once | 0 |
| `-warm <duration>` | Send requests for this long first | 0 |
| `-spec <file or url>` | The OpenAPI spec | `<url>/api/openapi.json` |
| `-body 'POST /path={...}'` | A JSON body for an operation without an example. Repeat | |
| `-header 'Name: value'` | A header for every request. Repeat | |
| `-worker <name>` | The Worker's name, for the CPU figures | From the URL |

## SDKs and releases

The task `sdk:gen` runs `sdk-gen`, and so on ([Tasks](tasks.md)).

| Command | Arguments and flags |
|---|---|
| `sdk-list`, `sdk-clean`, `dist-sdk` | none |
| `sdk-gen`, `sdk-check` | `<group>` |
| `sdk-ready` | `[group...]`: default `typescript-dist`, `go`, and `cli` if the project has it |
| `sdk-publish` | `-check` (fail if `sdk/go` is stale), `-quick` (with it: by a hash), `-into <dir>` |
| `cli-build` | `-linux`: for Linux, in Docker. This, `dist-cli` and `cli-smoke` fail in a project without the `cli` group, saying it has no CLI |
| `dist-cli` | `-target <os>-<arch>,...`: of `darwin`, `linux`, `windows` × `amd64`, `arm64`; default every one this machine builds (darwin only on a Mac). Writes `dist/<project>-cli-<os>-<arch>[.exe]` and runs this machine's |
| `cli-smoke` | none: runs the CLI in `dist/` for this machine with `--version` |
| `dist` | `-target` as `dist-cli`: empties `dist/`, then `dist-sdk` and, with a CLI, `dist-cli` |
| `release` * | `vX.Y.Z`, `-dry-run`, `-prerelease`, `-notes-footer <text or file>`, in any order. At the repo's root: fails unless the tree is clean, `HEAD` is the default branch on `origin`, every workflow run on it passed and the tag is new; runs the tasks `setup`, `check`, `spec:diff` (a breaking change needs a major release) and `dist`, tags, pushes the tag, makes the GitHub Release (notes: the commits since the last version tag), runs `release:publish` and `release:tags`. A task the repo lacks is skipped (no `dist`: it ships no files). `-dry-run` stops before the tag |
| `publish` * | `-tag vX.Y.Z`: makes the Release if missing, attaches the files in `dist/` it lacks, writes `SHA256SUMS` of every file on it. Without a tag (or one from `release` or a workflow), a dry run |
| `dist-ts` * | `-tag vX.Y.Z`: builds `ts/` and packs it as `dist/charter-ts-X.Y.Z.tgz`, in this repo |
| `release-tags` * | `-tag vX.Y.Z`, then `<module dir>...` |
| `release-tool` * | `-tag vX.Y.Z`: GoReleaser on the tool, in this repo; a Release that has its files already gets a build only |
| `ci-scope` | none: in the check workflow, writes `tool`, `library`, `library-ts` and `examples` to `$GITHUB_OUTPUT`: all of them on a push, on a pull request those for what it changes ([which](tasks.md#windows-and-macos)) |

## Repos that use each other

[The guide](../guides/repos.md).

| Command | Flags | What it does |
|---|---|---|
| `catalog` * | `-owner <login>` (default: the token's), `-json`, `-page` | REMOTE, read-only. The owner's charter repos, what each publishes, and the repos that pin it, read from their `mise.toml`, `go.mod` and `package.json`. Markdown; `-page` adds a docs page's front matter |
| `spec-diff` | `-from vX.Y.Z` (default: the newest version tag before `-tag`), `-to vX.Y.Z` (default: the working tree), `-tag vX.Y.Z` (default: the workflow's tag), `-catalog <file or url>` | Compares `fern/openapi.json` and `fern/asyncapi.json`, marking breaking changes `!`. Fails on one unless `-tag` is a major release; with `-catalog`, names the repos that pin this one |

## Any repo

What a repo takes whether or not it holds a project ([the guide](../guides/any-repo.md)); the tasks of `tasks/repo/` run these.

| Command | Flags | What it does |
|---|---|---|
| `adopt` * | `-description <text>` (default: the repo's on GitHub, or its folder's name), `-from <checkout>` (include `tasks/repo` from a checkout of charter, and pin no tool) | In a git repo, at its root, writes what is missing: `charter.toml`, `AGENTS.md`, `CLAUDE.md`, the start page and the rules in `docs/`, a `mise.toml` with the tool pinned at its own release and `tasks/repo` included at that tag; and what `docs` writes (without the repo on GitHub, only `docs/writing.md`). A `mise.toml` the repo has is kept: it prints the lines to add. Changes nothing on GitHub; safe to run again |
| `repo` * | `-check` | REMOTE. At the repo's root, from its `charter.toml`: the docs site (with its generated pages), the issue forms (not one the repo wrote itself) and `labels.tsv`, `renovate.json` (unless `renovate = false`), the workflows (if it is a project or lists `projects`; otherwise the one `repo-check` workflow, unless `workflow = false`), the labels, the description, homepage and topics (always with `charter`), GitHub Pages. Prints `ok` or `changed` per item; `-check` changes nothing and fails on drift ([how](../guides/deploy.md#keep-the-repo-in-shape)) |
| `issue` * | `<bug\|feature\|upstream\|plan>` | Prints an issue body with that form's headings (the repo's own form's, where it has one), for `gh issue create --body-file`. Plans are `plan` issues, never pages |
| `labels` * | | REMOTE. Creates or updates the repo's labels from the labels file; removes GitHub's default labels it lacks that nothing uses |
| `docs` * | `-check`, `-into <repo dir>` | Writes the docs site's config, `docs/writing.md`, `docs/llms.txt`, and each page that `_generated.toml` in `docs/` lists, from its command's output; `-check` fails if one is stale ([how](../guides/deploy.md#generated-pages)) |
| `docs-tasks` * | `-only <source,...>`, `-not <source,...>` (a source: `mise.toml`, an included file or folder as written, a git include as `owner/repo//folder`; a part of the name is enough), `-title <text>` (default `Tasks`) | Prints the repo's mise tasks as a page, for `_generated.toml`: a table per source in the order the tasks are written, then each task's usage, arguments and flags. No hidden tasks, none of a mise config outside the repo ([how](../guides/any-repo.md#the-tasks-page)) |
| `files` * | | Lists every file in the repo whose first lines say charter wrote it: charter's (`matches`, `differs`, `missing`, or not compared and why) and those `adopt` or `new` started, which are the repo's. Changes nothing; reads GitHub only for the two files that hold the repo's name or branch ([how](../guides/any-repo.md#tell-what-is-charters)) |
| `docs-lint` * | `-into <repo dir>` | Fails on missing front matter, an unlinked page, a dead link or anchor, an unknown task, a missing path, a release version; in a generated page, only the first three |
| `docs-review` * | `-print` | Hands Claude the review prompt with what the lint found |
| `upstream` * | | Every `Upstream:` tag in the code, with its issue's state |
| `need-env` * | `<NAME>...` | Fails unless these variables are set |
| `github-secrets` * | `<NAME>...` | Copies variables into the repo's GitHub secrets, never printing them |

## A repo that holds projects

| Command | Flags | What it does |
|---|---|---|
| `workflows` * | `-check`, `-into <repo dir>` | Writes `.github/`: the workflows `check`, `deploy`, `sdk-check`, `release` (its CLI jobs only with the `cli` group), the issue forms (not one the repo wrote itself) and `labels.tsv`. A repo that is not a project takes the forms from `repo` |
| `each` * | `-only <name,...>`, then `<task> [args]` | Runs a mise task in every project below this folder that has it |
