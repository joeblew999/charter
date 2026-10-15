---
title: SDKs
nav_order: 5
---

# SDKs: generate, prove and publish them

[Fern](https://buildwithfern.com) generates typed clients, and a CLI if the project has one, from the specs your contract writes. It runs in Docker. Method names, paging and streaming come from the contract.

## Generate

```sh
npm run sdk                       # the TypeScript SDK -> sdk/out/typescript-dist (not committed)
npm run sdk -- go                 # any group of fern/generators.yml -> sdk/out/<group>
```

```ts
import { NotesClient } from "./sdk/out/typescript-dist/esm/index.mjs";
const client = new NotesClient({ baseUrl: "http://localhost:<the project's port>" });
for await (const n of await client.notes.list({ limit: 50 })) console.log(n.id);   // pages by itself
```

## Prove them, in every language

`npm run sdk -- --prove` generates every group of `fern/generators.yml` at once, each in a Docker container of its own, and proves each in its language as soon as it is generated: Go built, vetted and tested against WireMock; TypeScript typechecked; Python checked by mypy and its own tests; the Rust CLI built. Each says how long it took. The toolchains come from mise, at versions charter-ts pins. This repo's CI proves the conformance projects' clients, which carry every Fern feature.

A language is a group under `groups:` in `fern/generators.yml`, with Fern's generator for it (a version from Fern's releases):

```yaml
  python:
    generators:
      - name: fernapi/fern-python-sdk
        version: <a version of fern-python-sdk>
        output:
          location: local-file-system
          path: ../sdk/out/python
```

A project has a CLI if it has the group `cli`: Fern writes it in Rust, and `npm run sdk -- --prove cli` builds it.

## Give the Go SDK to another repo

In a Go project with `go: { sdk: true }` in `charter.config.ts`:

```sh
npm run sdk -- --publish   # generates and proves the Go SDK, and copies it into sdk/go
git add sdk/go             # commit it with its contract change; never edit it
```

Then another repo runs `go get <your module>/sdk/go@main`, or `@<tag>`.

## Fern features

The conformance (`go/conformance/`, and `ts/conformance/` from oRPC) switches each on; copy the lines from its contract and `api/spec.go`.

| Feature | In the Go contract | The caller gets |
|---|---|---|
| OAuth client credentials | A security scheme in `config()`; a form-encoded token operation (`humaworkers.WithForm`); `auth-schemes` in `generators.yml` | `new ConformanceClient({ oauth: { clientId, clientSecret } })` |
| Idempotency | `x-fern-idempotency-headers`, `"x-fern-idempotent": true`, a hidden header field | `notes.create(body, { idempotencyKey })` |
| Multipart upload | `RawBody multipart.Form` and the form's schema | `files.uploadFile({ file, note })` |
| Webhooks, signed | `doc.Webhooks`; `x-fern-webhook-signature`; `crypto/hmac` in the handler | `WebhooksHelper.verifySignature(...)` |
| A WebSocket both ways | `asyncapi.Operation` on the GET, `asyncapi.SendOperation` on a POST to the same path | A typed `sendSubscribe()` (TypeScript) |
| Audiences | `x-fern-audiences`; a group with `audiences: [public]` | A public SDK without internal operations |
| Resumable SSE | A `text/event-stream` response; `x-fern-streaming` with `terminator` and `resumable`; each event's `id` its place in the log, and the handler resuming from the newer of `after` and `Last-Event-ID` | `notes.watch({ after, seconds })`, which reconnects on a drop (because it is resumable) and stops at the terminator |
| MCP tools | `serve.API` serves them; `mcp.Expose(op, false)` for what is no tool | Any MCP client: the operations as tools at `/api/mcp` |

The spec only describes: the server must enforce auth, and idempotency is the handler's job (the conformance projects keep the first note under each key, in D1).
