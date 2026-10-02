---
title: Configuration
nav_order: 4
parent: Reference
---

# Configuration: variables, ports, bindings, files and pins

Everything a project made by `charter new` can be set with, and every file it has. Read it to point the project at your own Cloudflare account, to find which file to edit, or to move a pinned version. The values are those of a project created on 2026-10-01 with `-name billing-api`.

## Environment variables

`mise.toml` sets the first two under `[env]` for every task. Each of those lines takes the value from your environment when it is set there, and the default otherwise.

| Variable | Default | Who reads it |
|---|---|---|
| `API_PORT` | `5174` | The tasks `run`, `dev` and `mcp-test`, and `charter migrate-local` |

| `API_URL` | `https://<name>.gedw99.workers.dev` | The tasks `spec` and `spec:check` (the server the specs name), `live-test`, `soak` and `bench` (the Worker they call), and the tool wherever it needs the Worker's name, which is the first label of this URL's host (`migrate`, `migrate:local`, `perf:try`) |
| `PORT` | none | The server itself. Natively (`go run .` in the project's folder) it listens there, or on 9900. Under `cf dev`, `vite.config.ts` listens there, or on 5173. The tasks set it from `API_PORT` |
| `APP_NAME` | natively `<name> (go run)`; on Cloudflare the Worker's name | The hello operation (`/api/hello`). Natively it is an environment variable; on Cloudflare it is a binding ([below](#the-workers-bindings)) |
| `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` | none | The Cloudflare CLI (`cf`), instead of `cf auth login`. The task `cloudflare:token` fails without them, and `cloudflare:secrets` copies them to GitHub. The deploy workflow needs them as repository secrets |
| `FERN_TOKEN` | none | Fern. `mise run doctor` warns when it is not set. Generating SDKs locally ran without one on 2026-10-01 |
| `GITHUB_REF_TYPE`, `GITHUB_REF_NAME` | set by GitHub Actions | `charter release` and `charter release-tags`: on a version tag they publish, otherwise they are a dry run |
| `GH_TOKEN` | set by the workflows | `gh`, in GitHub Actions |
| `WIREMOCK_URL` | set by `charter sdk-check` | The tests of a generated Go SDK. Nothing to set by hand |

The default of `API_URL` names the `workers.dev` subdomain of the charter project's own Cloudflare account. On any other account it is wrong until you set it: see the next section.

## Local settings: `mise.local.toml`

A file `mise.local.toml` beside `mise.toml` overrides it on your machine. Git ignores it. Put your Worker's URL there, which `mise run deploy` prints:

```toml
[env]
API_URL = "https://billing-api.<your-subdomain>.workers.dev"
```

Then write the specs again, because they name the server:

```sh
mise run spec         # the specs now name your URL
mise run spec:check   # passes again
```

For everyone who works on the project, change the default in `mise.toml` instead, and commit it with the new specs.

Secrets do not go in either file. Git also ignores `.dev.vars*`, `.env*` and `.secrets/`.

## Ports and URLs

| What | Where |
|---|---|
| `mise run run` (native) | `http://localhost:5174` |
| `mise run dev` (workerd) | `http://localhost:5174`: the same port, so run one at a time or set `API_PORT` for one of them |
| `mise run test:native`, `mise run test:workerd` | A free port each run, so checks can run beside a dev server |
| `go run .` in the project's folder, without the task | `PORT`, or 9900 |
| The deployed Worker | `https://<name>.<your-subdomain>.workers.dev` |

The routes of a new project, on any of those:

| Path | What |
|---|---|
| `/api/hello`, `/api/notes`, `/api/notes/watch` | The notes API: REST and the SSE stream |
| `/api/notes/live` | The WebSocket |
| `/api/openapi.json`, `/api/asyncapi.json` | The specs, generated on request with the request's origin as their server |
| `/api/mcp` | The MCP endpoint |

## The Worker's bindings

`cloudflare.config.ts` declares the Worker: its name, its compatibility date, its entry (`worker.mjs`) and three bindings.

| Binding | What it is | How the Go code reads it |
|---|---|---|
| `DB` | The D1 database. On Cloudflare it is named `<name>-db` | `d1.OpenConnector("DB")`, then `sql.OpenDB`: plain `database/sql` |
| `HUB` | The Durable Object namespace of the hub (the library's class `Hub`, `go/worker/hub.mjs` in charter, which `worker.mjs` exports as `NotesHub`). One object, named `notes` | To publish: `cloudflare.NewDurableObjectNamespace("HUB")` and a fetch to the object. To subscribe: a WebSocket to it through `syscall/js` |
| `APP_NAME` | A text variable, set to the Worker's name | `cloudflare.Getenv("APP_NAME")` |

All three are read in one file, `platform_js.go`, which hands them to the handlers as an `api.Env` value with three fields: `Var` (a variable by name), `Store` (the database) and `Hub`. The native build has the same value from `platform_other.go`: environment variables, and one in-memory store and hub. The handlers never touch a binding directly. `Store` and `Hub` are opened per request, because a binding belongs to the request's environment.

To add a binding: declare it in `cloudflare.config.ts`, read it in `platform_js.go`, give the native build its stand-in in `platform_other.go`, and add a field to `api.Env` in `api/handlers.go`.

## The files of a project

What `charter new` creates. **Edit** marks what you change, **generated** what a task writes and you never edit, **never edit** what the tool owns. The rest is yours to change, and usually left alone.

```text
billing-api/
├── mise.toml                    every task and tool pin; with fern/ beside it, it marks the project (edit: the default of API_URL; add tasks)
├── README.md                    the project's first page
├── AGENTS.md, CLAUDE.md         point agents at docs/
├── .gitignore, .gitattributes
├── api/
│   ├── contract.go              EDIT: the contract. Every route, its input and output structs
│   ├── handlers.go              EDIT: the contract implemented
│   ├── store.go, store_js.go    EDIT: the log and the live signal, in memory or on D1
│   ├── spec.go                  both specs from the contract
│   ├── api_test.go              EDIT: the API's tests
│   ├── contract_test.go         the contract's rules: examples, MCP tools
│   └── mcp_test.go              the MCP endpoint's tests
├── cmd/spec/main.go             the command that writes the spec files
├── main.go                      the entry of both builds
├── platform_js.go               the Worker's bindings (edit to add a binding)
├── platform_other.go            the native build's stand-ins
├── cloudflare.config.ts         the Worker: name, bindings, compatibility date (edit to add a binding)
├── vite.config.ts               the dev server's port
├── worker.mjs                   the Worker's entry in JavaScript. What it imports from build/ is written by the build,
│                                from the Go library: go.mjs, websocket.mjs, hub.mjs, tinygo-clock.mjs
├── go.mod, go.sum               the Go module, its dependencies and their pins
├── package.json                 the pins of the Cloudflare CLI (cf), Fern, TypeScript and the test clients
├── package-lock.json            generated by npm; committed
├── migrations/
│   └── 0001_init.sql            EDIT: the D1 schema. Add a file per change; never change an applied one
├── fern/
│   ├── fern.config.json         Fern's organization name
│   ├── generators.yml           EDIT: which SDKs Fern generates, their names and generator versions
│   ├── docs.yml                 the API docs site (mise run sdk:docs)
│   ├── openapi.json             GENERATED by mise run spec. Never edit
│   └── asyncapi.json            GENERATED by mise run spec. Never edit
├── sdk/
│   ├── go/                      GENERATED by mise run sdk:publish, committed: the Go SDK as a module. Not there at first
│   └── out/                     GENERATED by mise run sdk:gen, ignored
├── test/
│   ├── live-test.mjs            SSE and the WebSocket against a URL
│   ├── sdk-live-test.mjs        the same through the generated TypeScript SDK
│   ├── mcp-test.mjs             the MCP endpoint through the official client
│   ├── soak.mjs                 the real-time matrix, across a redeploy
│   └── soak-go/                 the Go SDK's client for the soak
├── .github/workflows/           GENERATED by mise run workflows. Never edit
└── docs/
    ├── README.md                EDIT: the start page and index of your docs
    ├── rules.md                 EDIT: the project's working rules
    └── writing.md               NEVER EDIT: written by the tool, the same in every repo
```

Created later, by tasks:

| Path | Written by | Committed |
|---|---|---|
| `build/` | `mise run build`: the Wasm and its glue | No (ignored) |
| `sdk/out/` | `mise run sdk:gen`: the generated SDKs and CLI | No (ignored) |
| `dist/` | `mise run sdk:dist`, `mise run sdk:dist:cli`: release files | No (it ignores itself) |
| `node_modules/` | `mise run setup` | No (ignored) |
| `.github/workflows/check.yml`, `.github/workflows/deploy.yml`, `.github/workflows/sdk-check.yml`, `.github/workflows/release.yml` | `mise run workflows`. Never edit | Yes |
| `docs/_config.yml`, `docs/_sass/custom/custom.scss`, `docs/llms.txt` | `mise run docs:setup`. Never edit | Yes |
| `mise.local.toml` | You | No (ignored) |

`mise run check` fails when a written workflow or docs file was edited by hand (`workflows:check`, `docs:check`).

## Pinned versions, and where each pin lives

Every tool has an exact version in one file. `mise install` and `mise run setup` install exactly those.

| What | Version in a new project | Where the pin lives |
|---|---|---|
| The tool the tasks run (`charter`) | the release the project was made from | `mise.toml`, the line `"go:github.com/joeblew999/charter/cmd/charter"` under `[tools]` |
| The charter Go packages | the same release | `go.mod`, the line `github.com/joeblew999/charter/go` |
| Go | 1.27.1 | `mise.toml` |
| TinyGo | 0.42.0 | The charter tool (`cmd/charter/wasm.go`): it moves with the tool's release |
| binaryen (`wasm-opt`, which TinyGo runs) | 133 | The charter tool, beside TinyGo |
| Node | 26.10.0 | `mise.toml` |
| gh | 2.101.0 | `mise.toml` |
| fnox | 1.35.2 | `mise.toml` |
| Rust | 1.98.1 | `mise.toml` |
| Huma | 2.39.1 | `go.mod` |
| workers-go | 0.36.0 | `go.mod` |
| The Cloudflare CLI (`cf`) | 1.0.0-beta.5 | `package.json` |
| Fern's CLI (`fern-api`) | 5.140.0 | `package.json` |
| Fern's generators | Go SDK 1.64.0, TypeScript SDK 3.98.0, CLI 0.45.1 | `fern/generators.yml` |
| The Workers compatibility date | 2026-09-25 | `cloudflare.config.ts` |

The lockfiles (`go.sum`, `package-lock.json`) are committed. How to move the first two rows to a newer release: [Releases](releases.md#how-a-project-picks-up-a-new-release).

## Limits

- **A project made from a checkout pins nothing for the first two rows.** Made with `charter new -from <checkout>`, the tasks run the tool from the checkout (`CHARTER` in `mise.toml`, and a `go.work`) and `go.mod` has a `replace` line that points at the checkout's library. Remove the `replace` line and run `go get` once you depend on a release.
- **`vite.config.ts` and `package.json` do not pin everything exactly:** `vite` is a range and `@cloudflare/vite-plugin` follows the `beta` tag. `package-lock.json` holds what was installed.
- **What was checked for this page:** the tree, `mise.toml`, the override through `mise.local.toml` and the installed versions were read from a new project on 2026-10-01. The deployed Worker's database name and URL are read from the source and from charter's own Worker, not from a deploy of the new project.
