---
title: SDKs
nav_order: 3
parent: Guides
---

# SDKs: generate, add features and languages

[Fern](https://buildwithfern.com) generates typed clients, and a CLI if the project has one, from the specs your contract writes. It runs in Docker. Method names, paging and streaming come from the contract.

## Generate and check

```sh
npm run sdk -- typescript         # -> sdk/out/typescript (not committed)
npm run sdk -- go                 # -> sdk/out/go
```

```ts
import { NotesClient } from "./sdk/out/typescript-dist/esm/index.mjs";
const client = new NotesClient({ baseUrl: "http://localhost:5174" });
for await (const n of await client.notes.list({ limit: 50 })) console.log(n.id);   // pages by itself
```

## Give the Go SDK to another repo

```sh
npm run sdk:publish    # generates, checks and copies the Go SDK into sdk/go
git add sdk/go          # commit it with its contract change; never edit it
```

Then `go get <your module>/sdk/go@main`, or `@latest` once a release has tagged it.

## Add a language

Add a group under `groups:` in `fern/generators.yml`. `npx charter-ts clients` generates every group and proves each in its language: Go built, vetted and tested against WireMock; TypeScript typechecked; Python checked by mypy and its own tests; the Rust CLI built (a debug build, into charter's cache, so only what changed is built again). Every client is generated at once, each in a Docker container of its own, and proven as soon as it is generated, so a project's clients take about as long as its slowest one; each says how long its generation and its proof took. The toolchains come from mise, at versions charter-ts pins; Fern itself runs in Docker. The notes and the conformance projects declare Go, TypeScript, Python and the CLI; this repo's CI proves the conformance projects' clients (`npx charter-ts ci clients`), which carry every Fern feature.

```yaml
  python:
    generators:
      - name: fernapi/fern-python-sdk
        version: 5.41.2
        output:
          location: local-file-system
          path: ../sdk/out/python
```

## Add the CLI

A project has a CLI if `fern/generators.yml` has the group `cli`: Fern writes it in Rust, and `npx charter-ts clients cli` generates it into `sdk/out/cli` and builds it. The notes projects and the conformance projects have the group. Building it for every OS is not one of charter-ts's steps yet.

## Fern features

The conformance (`go/conformance/`, and `ts/conformance/` from oRPC) switches each on; copy the lines from its contract and `api/spec.go`.

| Feature | In the Go contract | The caller gets |
|---|---|---|
| OAuth client credentials | A security scheme in `config()`; a form-encoded token operation (`humaworkers.WithForm`); `auth-schemes` in `generators.yml` | `new ConformanceClient({ clientId, clientSecret })` |
| Idempotency | `x-fern-idempotency-headers`, `"x-fern-idempotent": true`, a hidden header field | `notes.create(body, { idempotencyKey })` |
| Multipart upload | `RawBody multipart.Form` and the form's schema | `files.uploadFile({ file, note })` |
| Webhooks, signed | `doc.Webhooks`; `x-fern-webhook-signature`; `crypto/hmac` in the handler | `WebhooksHelper.verifySignature(...)` |
| A WebSocket both ways | `asyncapi.Operation` on the GET, `asyncapi.SendOperation` on a POST to the same path | A typed `sendSubscribe()` (TypeScript) |
| Audiences | `x-fern-audiences`; a group with `audiences: [public]` | A public SDK without internal operations |
| Resumable SSE | A `text/event-stream` response; `x-fern-streaming` with `terminator` and `resumable`; each event's `id` its place in the log, and the handler resuming from the newer of `after` and `Last-Event-ID` | `notes.watch({ after, seconds })`, which reconnects on a drop and stops at the terminator |
| MCP tools | `humamcp.Handler(routes)`; `humamcp.Expose(op, false)` for what is no tool | Any MCP client: the operations as tools at `/mcp` |

The spec only describes: the server must enforce auth, and idempotency is the handler's job (the conformance projects keep the first note under each key, in D1).

## Update charter-ts

```sh
npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz
npm run adopt && npm run check
```

In Go, also the library: `go get github.com/joeblew999/charter/go@vX.Y.Z && go mod tidy`. Renovate opens these pull requests by itself ([Repos that use each other](repos.md)).
