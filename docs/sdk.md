---
title: Fern folder internals (sdk/)
nav_order: 5
parent: This repository
---
# sdk/: Fern, the SDKs it generates, and the Fern CLI

Typed SDKs, a command-line program and a docs site, generated from each API's specs by **[Fern](https://buildwithfern.com)** (the `fern-api` npm package, running its generators locally in Docker). Read this page to generate or check an SDK, to add an API, to see which Fern feature is switched on how, or to work on the oRPC showcase and the harness Worker that runs the TypeScript SDK inside workerd. The Go showcase has its own page ([showcase-go.md](showcase-go.md)).

Fern is also what Cloudflare's own [Forge](https://github.com/cloudflare/forge) builds `cf` on. Forge itself isn't used here, only its spec of the Cloudflare API, in `mise run sdk:cloudflare`.

## Tasks (from the repo root)

```sh
mise run doctor                           # check the setup: npm installs, Docker, Go, TinyGo, Rust
mise run setup                            # npm packages, including Fern (fern-api) into sdk/node_modules
mise run sdk:list                         # the APIs in sdk/fern/apis and the groups each one defines
mise run sdk:check-spec petstore          # fern check: validate an API's specs and settings
mise run sdk:gen petstore go              # generate one SDK (Docker) into sdk/out/petstore/go
mise run sdk:check sdk/out/petstore/go    # prove it works. Go: build, vet, tests against WireMock. TypeScript: typecheck
mise run sdk:ready api                    # generate and build what the tests use, if missing: typescript-dist, go, cli
mise run sdk:demo                         # LOCAL, small end to end: Go and TypeScript SDKs for petstore, checked
mise run sdk:cli:build sdk/out/api/cli    # HEAVY: build a generated Fern CLI natively; -linux before the folder builds for Linux in Docker
mise run sdk:docs                         # preview Fern's API docs site: http://localhost:3030
mise run sdk:cloudflare                   # HEAVY: add Cloudflare products as an API (below)
mise run sdk:clean                        # remove sdk/out, stop leftover WireMock containers

mise run showcase:spec                    # the oRPC showcase: write both specs again, after changing its contract
mise run showcase:check                   # LOCAL: showcase:spec:check, fern check, showcase:test, showcase:typecheck
mise run showcase:test                    # sdk/harness/test, in Node: the server's routes, and the specs' surface
mise run showcase:typecheck               # typecheck the harness Worker (copies the SDK in first)
mise run sdk:harness:test                 # LOCAL: the TypeScript SDK inside a Worker, under cf dev
mise run sdk:harness:test -remote         # REMOTE: the same on the deployed harness Worker
mise run sdk:harness:deploy               # REMOTE: deploy the harness Worker twice
```

`mise run sdk:ready <api> <group>...` takes the groups wanted; with none it makes the three above and builds the CLI. `mise run sdk:dist` and `mise run sdk:dist:cli` build what a release ships ([dev.md](dev.md#cutting-a-release)).

## Layout

`sdk/fern/` is a **standard Fern project**, so Fern's own docs apply as they are.

```
sdk/
├── package.json                  fern-api (pinned), TypeScript for sdk:check, and what the test programs import (ws, the MCP client)
├── tsconfig.base.json            what sdk:check typechecks a TypeScript SDK against: standard fetch, no Node types
├── fern/
│   ├── fern.config.json          organization, and the CLI version ("*" = the local CLI)
│   ├── docs.yml                  the docs site (mise run sdk:docs)
│   └── apis/
│       └── <api>/                one folder per API
│           ├── openapi.json      the spec; asyncapi.json beside it where the API has a WebSocket
│           ├── generators.yml    one group per SDK, options under config:
│           └── overlays.yml      the two showcases only: an OpenAPI Overlay applied to the spec
├── harness/                      the harness Worker: the oRPC showcase, and the TypeScript SDK inside workerd
└── out/<api>/<group>/            Fern's output (gitignored)
```

| API folder | Its specs | Groups |
|---|---|---|
| `sdk/fern/apis/api/` | Generated from the oRPC contract (`mise run api:spec`) | `go`, `typescript`, `typescript-dist`, `cli` |
| `sdk/fern/apis/api-go/` | Generated from the Go contract (`mise run api-go:spec`) | the same four |
| `sdk/fern/apis/showcase/` | Generated from the oRPC showcase's contract (`mise run showcase:spec`) | `go`, `typescript`, `typescript-public`, `typescript-dist`, `cli` |
| `sdk/fern/apis/showcase-go/` | Generated from the Go showcase's contract (`mise run showcase-go:spec`) | the same five |
| `sdk/fern/apis/petstore/` | A hand-written sample with no server | `go`, `typescript`, `python`, `cli` |
| `sdk/fern/apis/modern/` | A hand-written sample with no server: SSE streaming and cursor pagination | `go`, `typescript` |

- **`typescript-dist`** is the TypeScript SDK compiled to `.js` plus `.d.ts` (`outputSourceFiles: false`), the way an npm package reaches its users. The test programs and the harness Worker import it.
- **`typescript-public`** is the same spec without the operations not tagged `public` (audiences).
- **The names:** the notes API's SDKs are `OrpcApiClient` (TypeScript), the Go module `github.com/joeblew999/orpc-api/sdk/go`, and the CLI binary `orpc-api`, from either Fern folder. Both Fern folders declare that one module path, so `test/soak-go` builds against either SDK; the one committed at that path (`sdk/go/`) is the Go API's ([dev.md](dev.md#cutting-a-release)). The showcases' are `ShowcaseClient`, `example.com/showcase` and `showcase`.
- **Never edit a generated spec.** `generators.yml` and `overlays.yml` are the only hand-written files in the four generated folders.

## Adding an API

Copy `sdk/fern/apis/petstore/` to a new folder, replace `openapi.json`, and adjust `generators.yml`: the output paths, the Go module, and each generator's options. The options for each language are documented at `buildwithfern.com/learn/sdks/generators/<lang>/configuration`. Here we use:

- **`namespaceExport`,** which names the TypeScript client (`Petstore` gives `PetstoreClient`);
- **`module` and `packageName`** for Go;
- **`guardProcessEnvAccess: true`** and **`generateWebSocketClients: true`** for TypeScript SDKs that run in Workers or have a WebSocket;
- **`binaryName`** for the CLI.

For an API of your own, generate the spec from a contract instead ([api.md](api.md), [api-go.md](api-go.md)) and copy that API's Fern folder.

`mise run sdk:cloudflare` makes an API out of Cloudflare's own: it downloads Forge's 26 MB spec into `.forge/` (gitignored), keeps the account-level paths of the chosen products (default `d1,kv`; `-products workers,r2` to choose), and writes `openapi.json` and a `generators.yml` into a folder `cloudflare` beside the other APIs. Then `mise run sdk:gen cloudflare go`.

## What Fern does, feature by feature

Everything here is switched on through **standard options only**: the spec's OpenAPI features, `x-fern-*` extensions and `generators.yml`. The oRPC column is the oRPC showcase's contract (`sdk/harness/src/contract.ts`); the same table for a Go contract is in [showcase-go.md](showcase-go.md#feature-by-feature).

| Feature | How to switch it on | In the oRPC contract | What the SDKs get |
|---|---|---|---|
| OAuth client credentials | `auth-schemes:` in `generators.yml` plus a form-encoded token endpoint | `document` (`security`, `securitySchemes`); the token operation's `spec` hook makes its body form-encoded | Go: `option.WithClientID/WithClientSecret`. The token is fetched and reused |
| Idempotency | `x-fern-idempotency-headers` plus `x-fern-idempotent: true` | `document`, and the operation's `spec` hook | Go: `option.WithIdempotencyKey` |
| Retries | built in (per endpoint: `x-fern-retries`) | nothing | Go: `option.WithMaxAttempts` |
| Cursor/offset pagination | `x-fern-pagination` | the operation's `spec` hook | Go: `*core.Page[...]`, auto-paging |
| SSE / streaming | `x-fern-streaming: { format: sse }` | output `asyncIteratorObject(chunk)`, plus the `spec` hook | Go: `core.Stream[T]` |
| File upload | `multipart/form-data` body | `z.file()` in the input: oRPC writes and reads multipart by itself | Go: a typed `UploadFile(...)` |
| Webhooks | OpenAPI 3.1 `webhooks:` | the `webhooks` contract: one procedure per webhook, its input is the payload | typed payload structs |
| Webhook signatures | `x-fern-webhook-signature` (HMAC or asymmetric) | `document` | TypeScript: `WebhooksHelper.verifySignature(...)`. Go: `webhooks_helper.go` |
| WebSockets | an AsyncAPI spec beside the OpenAPI one, plus TypeScript `generateWebSocketClients: true` | `asyncapi({...})` on a procedure: its output is what the server sends, and an `asyncIteratorObject` input is what the client sends | Go: message types only. TypeScript: a reconnecting socket with `connect`, a typed `sendSubscribe`, `on('message')` and `close` (needs the `ws` package on Node) |
| Audiences | `x-fern-audiences` on endpoints plus `audiences: [public]` on a group | the operation's `spec` hook | one spec gives a full SDK and a public one (the group `typescript-public` has no internal `uploadFile`) |
| Overlays | `overlays: overlays.yml` beside the spec (OpenAPI Overlay 1.0) | not in the contract: the overlay file is applied to the generated spec | changes the SDK without editing the spec: `notes.listNotes` becomes `notes.list` |

The right-hand column was read from the SDKs generated on 2026-09-29. The TypeScript entries are exercised by the tests below; the Go showcase SDK only by Fern's own generated tests (`mise run sdk:check`).

## The oRPC showcase

The showcase is one small API with every Fern feature in the table. Its `openapi.json` and `asyncapi.json` are generated from an oRPC contract, like `api/`'s, and the harness Worker serves that contract.

| Path | What it is |
|---|---|
| `sdk/harness/src/contract.ts` | **The contract (edit this):** every operation, the WebSocket channel, the webhooks, and the document-level settings (`document`) |
| `sdk/harness/src/showcase.ts` | The contract implemented with oRPC, served by the harness Worker under `/api/mock/*` |
| `sdk/harness/src/specs.ts`, `sdk/harness/spec.ts` | Both specs from the contract, with `api/`'s generators (`api/src/specs.ts`, `api/src/asyncapi.ts`, `api/spec-files.ts`), imported, not copied |
| `sdk/harness/test/showcase.test.ts` | Run in Node, no Worker: the server's routes and its channel |
| `sdk/harness/test/surface.test.ts`, `sdk/harness/test/handwritten-surface.json` | The generated specs give Fern the surface the last hand-written specs had, but for the differences the test names |

The contract lives in the harness because the harness Worker is what serves it and tests it. `api/` and `sdk/harness/` each install the same pinned oRPC and Zod; keep the two pins equal.

After a contract change, run `mise run showcase:spec`. `mise run sdk:harness:test` generates the SDK again when a spec is newer than it.

**What oRPC's generators can't say, and where it is added in code.** None of it is patched into the JSON. Only the last row has an upstream issue; nothing is filed for the others.

| Fern needs | oRPC 2.0.0-beta.40 | Where it is added |
|---|---|---|
| A form-encoded request body (the OAuth token endpoint) | The handler reads `application/x-www-form-urlencoded`, but the generator always writes the body as `application/json` | The operation's `spec` hook renames the media type |
| `security: []` on one operation; `x-fern-*` on an operation | No field for them | The operation's `spec` hook |
| `security`, `components.securitySchemes`, `x-fern-idempotency-headers`, `x-fern-webhook-signature` | The contract has no document level. The generator takes them as `base` | `document` in the contract, passed as `base` by `openapiSpec` |
| OpenAPI 3.1 `webhooks` | Not generated | `openapiSpec({ webhooks })` in `api/src/specs.ts` writes one from a contract of webhook procedures |
| An SSE response whose schema is the event's data | Describes its own envelope (`event: message` / `close` / `error`) | The operation's `spec` hook (the same as `api/`'s `notes.watch`) |
| AsyncAPI, with messages both ways | No AsyncAPI generator (middleapi/orpc#2115) | `api/src/asyncapi.ts` ([api.md](api.md#the-asyncapi-generator)) |

Multipart is not on the list: a `z.file()` in the input is enough.

How the server behaves:

- **It checks no tokens.** The token endpoint gives the fixed `tok-1`, and every other route answers anyone. The Go showcase checks them ([showcase-go.md](showcase-go.md#differences-from-the-orpc-showcase)).
- **Errors are oRPC's JSON:** 401 for a wrong client, 400 for invalid input, 404 for an unknown path, 426 for the channel without an upgrade.
- **A page is two notes** unless `limit` says otherwise.
- **The chat stream is oRPC's SSE:** `event: message` per chunk, then `event: close` without data. Fern's reader takes it as it is.
- **Fern ignores an AsyncAPI server's `pathname`,** so the SDK's default WebSocket URL lacks `/api/mock`. The tests pass `baseUrl`.
- **The OAuth token URL comes from the spec's `servers` entry,** which is the deployed harness. The Fern CLI's `--base-url` doesn't move it.

One test program, `test/showcase-test.mjs`, runs the generated TypeScript SDK against either showcase server. Each server passes it with the SDK made from the other's specs too (2026-10-01, locally; [findings.md](findings.md)).

## The TypeScript SDK inside a Worker: the harness Worker

`sdk/harness/` is a small cf Worker project. It serves the oRPC showcase (`/api/mock/*`), and two routes run the generated SDK against it inside workerd: `/api/sdk-test`, and `/api/ws-test` for the WebSocket client. `mise run sdk:harness:test` calls both, then runs the SDK's WebSocket client from Node (`sdk/harness/ws-client.mjs`).

Nine checks, which pass under `cf dev` and on Cloudflare (2026-10-01, [findings.md](findings.md)):

- auto-pagination over 3 pages;
- OAuth client credentials (the token fetched form-encoded, then reused);
- an idempotent create (`Idempotency-Key` plus the bearer token);
- an SSE stream of typed chunks;
- a multipart file upload with a form field;
- webhook HMAC verification (a valid signature accepted, a forged one rejected);
- the WebSocket client inside a Worker, two checks: typed events, and the bearer token received;
- the WebSocket client from Node, over the network.

What it takes:

- **`guardProcessEnvAccess: true`.** Workers have no `process`, and the OAuth code reads `process.env`.
- **`outputSourceFiles: false`** (the group `typescript-dist`). The raw `.ts` source clashes with Cloudflare's Worker types (`Headers`, `Response`); the compiled package typechecks cleanly.
- **The WebSocket token as a query parameter.** The SDK sends the token as a handshake header, which Workers (like browsers) can't set, so from a Worker it never arrives. The way round uses standard SDK calls: `client.auth.getToken(...)`, then `liveNotes.connect({ queryParams: { access_token } })`, and the server accepts either the header or `?access_token=`.
- **Two Workers on Cloudflare.** A Worker can't call its own URL (error 1042), so the harness deploys twice, as `orpc-sdk-harness` and (with `--mode api`) `orpc-sdk-harness-api`, and the WebSocket test connects from the first to the second. Calling another Worker on the same `workers.dev` zone needs the `global_fetch_strictly_public` compatibility flag.

`-remote` tests the deployed pair at `HARNESS_URL` and `HARNESS_API_URL` (set in `mise.toml`; on another Cloudflare account, in `mise.local.toml`).

## SDKs and a CLI for the notes API

```sh
mise run api:spec                         # contract -> sdk/fern/apis/api/{openapi,asyncapi}.json (offline; the server is API_URL)
mise run sdk:gen api go                   # the other groups: typescript, typescript-dist, cli
mise run sdk:check sdk/out/api/go
mise run sdk:cli:build sdk/out/api/cli    # then: sdk/out/api/cli/target/release/orpc-api notes list --page-all
mise run api:live-test                    # SSE and WebSocket, raw and through the SDK
```

The same with `api-go` in place of `api` gives the SDKs and CLI from the Go contract's specs. The two Fern folders have the same groups and names, so the same test programs run against either SDK, and the SDKs from one server's specs work against the other server (2026-10-01, [findings.md](findings.md)).

- **What works** (2026-09-30 for the oRPC specs, 2026-10-01 for the Go ones): the Go SDK builds, vets and passes its tests; the TypeScript SDK typechecks; the CLI runs against the deployed Worker: `meta hello`, `notes create`, `notes list --page-all` across pages, and `notes watch --after`.
- **The one difference between the two:** Huma names its schemas (`components.schemas.Note`), so the SDKs from the Go specs have a shared `Note` type where the oRPC spec gives one type per response.
- **Real-time:** `x-fern-streaming: { format: sse, terminator, resumable: true }` is set in the contract, so the TypeScript and Go SDKs reconnect by themselves with `Last-Event-ID` after a clean end without the terminator. What a client still has to do, with a loop for each client, is in [realtime.md](realtime.md#what-a-client-does). Fern's known client gaps, and the workaround for each, are in [upstream.md](upstream.md).

## The Fern CLI (Fern's CLI generator, Rust)

"The CLI" in these pages is the command-line program Fern generates for an API, from the group `cli` in its `generators.yml`. The output is a Rust workspace with a cargo-dist config for 7 targets: macOS, Linux gnu/musl and Windows msvc.

What the next two lists say was tried by hand on 2026-09-29 and is recorded only here, not in [findings.md](findings.md).

**The petstore CLI, against a local mock.** Generating took 56 s in Docker; the first macOS build took 55 s and gave an 11.7 MB arm64 binary.

- **Commands per resource:** `petstore pets list-pets`, `create-pet --name rex`.
- **Output and requests:** `--format json|table|yaml|csv|jsonl|http`, `--query` (JMESPath), `--dry-run` (shows the request without sending it), `--base-url`, `--debug`.
- **For agents:** `--schema` gives the command surface as JSON, and `generate-skills` writes Claude-style `SKILL.md` files for the CLI (one shared file, one per resource).
- **Also included:** `auth` login, shell completions and a man page.

**The showcase CLI, against the deployed harness Worker.**

- **These worked:** OAuth client credentials (the token fetched automatically; `--debug` shows `authorization: [REDACTED]`), `notes list`, `notes list --page-all` (3 pages), `notes create --idempotency-key`, `chat` streaming SSE chunks live, and `files upload-file`.
- **No WebSocket command:** nothing came out for the AsyncAPI channel with generator 0.44.0 (tried with AsyncAPI 3.0 and 2.6). The output of 0.45.1 has an `asyncapi` module; whether it gives a working WebSocket command was not tried.

Limits of the generator:

- **A stream prints in json and jsonl only when it ends.** `--format raw` streams, as raw SSE lines ([upstream.md](upstream.md)).
- **`--page-all` with `--format jsonl` prints one line per page,** not per item ([findings.md](findings.md)).
- **A paginated method on the root client breaks the Rust build** (an overlay renaming it with no `x-fern-sdk-group-name`). Keep methods in a group. Seen on 2026-09-29; not filed.
- **Building is heavy the first time:** it compiles every Rust dependency. Natively it needs Rust (`rust-toolchain.toml`); with `-linux` it builds in Docker for the container's architecture. A Linux container can't build macOS binaries (no Apple SDK) or the `windows-msvc` target, so all 7 platforms need cargo-dist on GitHub Actions, one native runner per OS, which is not set up here ([plans/next.md](plans/next.md)).

## Good to know

- **Versions are pinned.** `fern-api` 5.140.0 in `sdk/package.json`; in each `generators.yml`, `fern-go-sdk` 1.64.0, `fern-typescript-sdk` 3.98.0, `fern-python-sdk` 5.34.0 and `fern-cli-generator` 0.45.1 for the two notes APIs, 0.44.0 for the others. They were the latest when checked on 2026-09-29; Forge pins older ones.
- **Licensing is not settled.** Fern's docs call local generation, WebSocket clients, webhook signatures and the CLI generator Enterprise or early access, needing a `FERN_TOKEN`. All of it has run here without one ([plans/next.md](plans/next.md)).
- **Where output goes:** `sdk/out/`, which is gitignored. The exception is `sdk/go/`, the committed copy of the Go API's Go SDK that another repo fetches with `go get`. Releases attach the SDK sources and the Linux CLIs to the GitHub Release ([dev.md](dev.md#cutting-a-release)). Publishing the SDKs as packages (npm, a repository per SDK) is planned, not done.
- **The docs site covers two APIs.** `sdk/fern/docs.yml` lists the showcase and petstore; `mise run sdk:docs` previews it locally. No task publishes it.
- **`mise run sdk:check` on a Go SDK starts a WireMock container** and stops it again. `mise run sdk:clean` stops any that were left behind.
