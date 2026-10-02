---
title: Configuration
nav_order: 4
parent: Reference
---

# Configuration: variables, ports, bindings, files and pins

Everything a project made by `dev new` can be set with, and every file it has. Read it to point the project at your own Cloudflare account, to find which file to edit, or to move a pinned version. The values are those of a project created on 2026-10-01 with `-name billing-api`.

## Environment variables

`mise.toml` sets the first two under `[env]` for every task. Each of those lines takes the value from your environment when it is set there, and the default otherwise.

| Variable | Default | Who reads it |
|---|---|---|
| `API_GO_PORT` | `5174` | The tasks `api:go:run`, `api:go:dev` and `api:go:mcp-test`, and `dev migrate-local` |
| `API_GO_URL` | `https://<name>.gedw99.workers.dev` | The tasks `api:go:spec` and `api:go:spec:check` (the server the specs name), `api:go:live-test`, `api:go:soak` and `api:go:bench` (the Worker they call) |
| `PORT` | none | The server itself. Natively (`go run .` in `api/go/`) it listens there, or on 9900. Under `cf dev`, `api/go/vite.config.ts` listens there, or on 5173. The tasks set it from `API_GO_PORT` |
| `APP_NAME` | natively `<name> (go run)`; on Cloudflare the Worker's name | The hello operation (`/api/hello`). Natively it is an environment variable; on Cloudflare it is a binding ([below](#the-workers-bindings)) |
| `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` | none | The Cloudflare CLI (`cf`), instead of `cf auth login`. The task `cloudflare:token` fails without them, and `cloudflare:secrets` copies them to GitHub. The deploy workflow needs them as repository secrets |
| `FERN_TOKEN` | none | Fern. `mise run doctor` warns when it is not set. Generating SDKs locally ran without one on 2026-10-01 |
| `GITHUB_REF_TYPE`, `GITHUB_REF_NAME` | set by GitHub Actions | `dev release` and `dev release-tags`: on a version tag they publish, otherwise they are a dry run |
| `GH_TOKEN` | set by the workflows | `gh`, in GitHub Actions |
| `WIREMOCK_URL` | set by `dev sdk-check` | The tests of a generated Go SDK. Nothing to set by hand |

The default of `API_GO_URL` names the `workers.dev` subdomain of the orpc-api project's own Cloudflare account. On any other account it is wrong until you set it: see the next section.

## Local settings: `mise.local.toml`

A file `mise.local.toml` beside `mise.toml` overrides it on your machine. Git ignores it. Put your Worker's URL there, which `mise run api:go:deploy` prints:

```toml
[env]
API_GO_URL = "https://billing-api.<your-subdomain>.workers.dev"
```

Then write the specs again, because they name the server:

```sh
mise run api:go:spec         # the specs now name your URL
mise run api:go:spec:check   # passes again
```

For everyone who works on the project, change the default in `mise.toml` instead, and commit it with the new specs.

Secrets do not go in either file. Git also ignores `.dev.vars*`, `.env*` and `.secrets/`.

## Ports and URLs

| What | Where |
|---|---|
| `mise run api:go:run` (native) | `http://localhost:5174` |
| `mise run api:go:dev` (workerd) | `http://localhost:5174`: the same port, so run one at a time or set `API_GO_PORT` for one of them |
| `mise run api:go:test:native`, `mise run api:go:test:workerd` | A free port each run, so checks can run beside a dev server |
| `go run .` in `api/go/`, without the task | `PORT`, or 9900 |
| The deployed Worker | `https://<name>.<your-subdomain>.workers.dev` |

The routes of a new project, on any of those:

| Path | What |
|---|---|
| `/api/hello`, `/api/notes`, `/api/notes/watch` | The notes API: REST and the SSE stream |
| `/api/notes/live` | The WebSocket |
| `/api/openapi.json`, `/api/asyncapi.json` | The specs, generated on request with the request's origin as their server |
| `/api/mcp` | The MCP endpoint |

## The Worker's bindings

`api/go/cloudflare.config.ts` declares the Worker: its name, its compatibility date, its entry (`api/go/worker/index.mjs`) and three bindings.

| Binding | What it is | How the Go code reads it |
|---|---|---|
| `DB` | The D1 database. On Cloudflare it is named `<name>-db` | `d1.OpenConnector("DB")`, then `sql.OpenDB`: plain `database/sql` |
| `HUB` | The Durable Object namespace of the hub (the class `NotesHub` in `api/go/worker/hub.mjs`). One object, named `notes` | To publish: `cloudflare.NewDurableObjectNamespace("HUB")` and a fetch to the object. To subscribe: a WebSocket to it through `syscall/js` |
| `APP_NAME` | A text variable, set to the Worker's name | `cloudflare.Getenv("APP_NAME")` |

All three are read in one file, `api/go/platform_js.go`, which hands them to the handlers as an `api.Env` value with three fields: `Var` (a variable by name), `Store` (the database) and `Hub`. The native build has the same value from `api/go/platform_other.go`: environment variables, and one in-memory store and hub. The handlers never touch a binding directly. `Store` and `Hub` are opened per request, because a binding belongs to the request's environment.

To add a binding: declare it in `api/go/cloudflare.config.ts`, read it in `api/go/platform_js.go`, give the native build its stand-in in `api/go/platform_other.go`, and add a field to `api.Env` in `api/go/api/handlers.go`.

## The files of a project

What `dev new` creates. **Edit** marks what you change, **generated** what a task writes and you never edit, **never edit** what the tool owns. The rest is yours to change, and usually left alone.

```text
billing-api/
├── mise.toml                    every task and tool pin (edit: the default of API_GO_URL; add tasks)
├── go.work                      the Go version; marks the project's root
├── rust-toolchain.toml          the Rust version, for the Fern CLI
├── README.md                    the project's first page
├── AGENTS.md, CLAUDE.md         point agents at docs/
├── .gitignore
├── api/go/                      the API: one Go module
│   ├── api/
│   │   ├── contract.go          EDIT: the contract. Every route, its input and output structs
│   │   ├── handlers.go          EDIT: the contract implemented
│   │   ├── store.go             EDIT: the log and the live signal, on D1 or in memory
│   │   ├── spec.go              both specs from the contract
│   │   ├── api_test.go          EDIT: the API's tests
│   │   └── mcp_test.go          the MCP endpoint's tests
│   ├── cmd/spec/main.go         the command that writes the spec files
│   ├── main.go                  the entry of both builds
│   ├── platform_js.go           the Worker's bindings (edit to add a binding)
│   ├── platform_other.go        the native build's stand-ins
│   ├── cloudflare.config.ts     the Worker: name, bindings, compatibility date (edit to add a binding)
│   ├── vite.config.ts           the dev server's port
│   ├── worker/
│   │   ├── index.mjs            the Worker's entry in JavaScript
│   │   ├── go.mjs               runs the Go Wasm, reusing a Go runtime for the next request
│   │   ├── websocket.mjs        the WebSocket adapter
│   │   ├── hub.mjs              the hub: a Durable Object class
│   │   └── tinygo-clock.mjs     the fix that makes Go timers fire on Cloudflare
│   ├── go.mod, go.sum           the Go dependencies and their pins
│   ├── package.json             the pin of the Cloudflare CLI (cf)
│   ├── package-lock.json        generated by npm; committed
│   └── .gitignore
├── migrations/
│   └── 0001_init.sql            EDIT: the D1 schema. Add a file per change; never change an applied one
├── sdk/
│   ├── fern/
│   │   ├── fern.config.json     Fern's organization name
│   │   └── apis/api-go/
│   │       ├── generators.yml   EDIT: which SDKs Fern generates, their names and generator versions
│   │       ├── openapi.json     GENERATED by mise run api:go:spec. Never edit
│   │       └── asyncapi.json    GENERATED by mise run api:go:spec. Never edit
│   ├── package.json             the pins of Fern, TypeScript and the test clients
│   ├── package-lock.json        generated by npm; committed
│   ├── tsconfig.base.json       what sdk:check typechecks a TypeScript SDK with
│   └── .gitignore
├── test/
│   ├── live-test.mjs            SSE and the WebSocket against a URL
│   ├── sdk-live-test.mjs        the same through the generated TypeScript SDK
│   ├── mcp-test.mjs             the MCP endpoint through the official client
│   ├── soak.mjs                 the real-time matrix, across a redeploy
│   ├── soak-go/                 the Go SDK's client for the soak
│   └── .gitignore
└── docs/
    ├── README.md                EDIT: the start page and index of your docs
    ├── rules.md                 EDIT: the project's working rules
    └── writing.md               NEVER EDIT: written by the tool, the same in every repo
```

Created later, by tasks:

| Path | Written by | Committed |
|---|---|---|
| `api/go/build/` | `mise run api:go:build`: the Wasm and its glue | No (ignored) |
| `sdk/out/` | `mise run sdk:gen`: the generated SDKs and CLI | No (ignored) |
| `dist/` | `mise run sdk:dist`, `mise run sdk:dist:cli`: release files | No (it ignores itself) |
| `node_modules/` in `api/go/` and `sdk/` | `mise run setup` | No (ignored) |
| `.github/workflows/api-check.yml`, `.github/workflows/api-deploy.yml`, `.github/workflows/sdk-check.yml`, `.github/workflows/sdk-release.yml` | `mise run dev:workflows`. Never edit | Yes |
| `docs/_config.yml`, `docs/_sass/custom/custom.scss`, `docs/llms.txt` | `mise run docs:setup`. Never edit | Yes |
| `mise.local.toml` | You | No (ignored) |

`mise run dev:check` fails when a written workflow or docs file was edited by hand.

## Pinned versions, and where each pin lives

Every tool has an exact version in one file. `mise install` and `mise run setup` install exactly those.

| What | Version in a new project | Where the pin lives |
|---|---|---|
| The tool the tasks run (`dev`) | the release the project was made from | `mise.toml`, the line `"go:github.com/joeblew999/orpc-api/dev"` under `[tools]` |
| The orpc-api Go packages | the same release | `api/go/go.mod`, the line `github.com/joeblew999/orpc-api/api/go` |
| Go | 1.27.1 | `go.work`, the `toolchain` line |
| TinyGo | 0.42.0 | `mise.toml` |
| binaryen (`wasm-opt`, which TinyGo runs) | 133 | `mise.toml` |
| Node | 26.10.0 | `mise.toml` |
| jq | 1.8.2 | `mise.toml` |
| gh | 2.101.0 | `mise.toml` |
| fnox | 1.35.2 | `mise.toml` |
| Rust | 1.98.1 | `rust-toolchain.toml` |
| Huma | 2.39.1 | `api/go/go.mod` |
| workers-go | 0.36.0 | `api/go/go.mod` |
| The Cloudflare CLI (`cf`) | 1.0.0-beta.5 | `api/go/package.json` |
| Fern's CLI (`fern-api`) | 5.140.0 | `sdk/package.json` |
| Fern's generators | Go SDK 1.64.0, TypeScript SDK 3.98.0, CLI 0.45.1 | `sdk/fern/apis/api-go/generators.yml` |
| The Workers compatibility date | 2026-09-25 | `api/go/cloudflare.config.ts` |

The lockfiles (`api/go/go.sum`, `api/go/package-lock.json`, `sdk/package-lock.json`) are committed. How to move the first two rows to a newer release: [Releases](releases.md#how-a-project-picks-up-a-new-release).

## Limits

- **A project made from a checkout pins nothing for the first two rows.** Made with `dev new -from <checkout>`, the tool's pin is `latest` and `api/go/go.mod` has a `replace` line that points at the checkout. Remove the `replace` line and run `go get` once you depend on a release.
- **`api/go/vite.config.ts` and `api/go/package.json` do not pin everything exactly:** `vite` is a range and `@cloudflare/vite-plugin` follows the `beta` tag. `api/go/package-lock.json` holds what was installed.
- **What was checked for this page:** the tree, `mise.toml`, the override through `mise.local.toml` and the installed versions were read from a new project on 2026-10-01. The deployed Worker's database name and URL are read from the source and from orpc-api's own Worker, not from a deploy of the new project.
