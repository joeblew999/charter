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

Which SDKs a project has is `sdks` in `charter.config.ts`; `npm run spec` writes Fern's settings, `fern/generators.yml`, from it and the spec, with the generators charter-ts pins:

```ts
sdks: { name: "Notes", more: ["python", "cli"] },
```

| Field | Gives |
|---|---|
| `name` | The TypeScript SDK's namespace; in lower case, the Go package and the CLI's command |
| (always) | The groups `go`, `typescript` (sources) and `typescript-dist` (compiled); the Go SDK's module is the project's Go module, or in TypeScript the Worker's name, with `/sdk/go` |
| `more` | `python`, and `cli`: a command-line client in Rust (Fern's early access), which `npm run sdk -- --prove cli` builds |
| `audiences` | For each, a TypeScript SDK of only what the contract tags for it (`x-fern-audiences`): the group `typescript-<audience>` |

The credentials each SDK sends are the spec's security schemes, restated there: bearer tokens, header keys (a Cloudflare Access service token) and OAuth client credentials, any of them.

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
| MCP tools | `serve.API` serves them; `mcp.NotATool(op)` for what is no tool | Any MCP client: the operations as tools at `/api/mcp` |

The spec only describes: the server must enforce auth, and idempotency is the handler's job (the conformance projects keep the first note under each key, in D1).
