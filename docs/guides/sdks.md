---
title: SDKs
nav_order: 3
parent: Guides
---

# SDKs: generate, add features and languages

[Fern](https://buildwithfern.com) generates typed clients and a CLI from the specs your contract writes. It runs in Docker. Method names, paging and streaming come from the contract.

## Generate and check

```sh
mise run sdk:gen typescript         # -> sdk/out/typescript (not committed)
mise run sdk:gen go                 # -> sdk/out/go
mise run sdk:check go               # build, vet, Fern's tests against a mock
mise run sdk:cli:build              # HEAVY the first time: the Rust CLI
mise run sdk:clean                  # after a contract change: a stale SDK is not regenerated
```

```ts
import { NotesClient } from "./sdk/out/typescript-dist/esm/index.mjs";
const client = new NotesClient({ baseUrl: "http://localhost:5174" });
for await (const n of await client.notes.list({ limit: 50 })) console.log(n.id);   // pages by itself
```

## Give the Go SDK to another repo

```sh
mise run sdk:publish    # generates, checks and copies the Go SDK into sdk/go
git add sdk/go          # commit it with its contract change; never edit it
```

Then `go get <your module>/sdk/go@main`, or `@latest` once a release has tagged it.

## Add a language

Add a group under `groups:` in `fern/generators.yml`, then `mise run sdk:gen python`. A release ships it; `sdk:check` covers only Go and TypeScript.

```yaml
  python:
    generators:
      - name: fernapi/fern-python-sdk
        version: 5.34.0
        output:
          location: local-file-system
          path: ../sdk/out/python
```

## Fern features

The showcase (`conformance/showcase-go/`, and `conformance/showcase-ts/` from oRPC) switches each on; copy the lines from its contract and `api/spec.go`.

| Feature | In the Go contract | The caller gets |
|---|---|---|
| OAuth client credentials | A security scheme in `config()`; a form-encoded token operation (`humaworkers.WithForm`); `auth-schemes` in `generators.yml` | `new ShowcaseClient({ clientId, clientSecret })` |
| Idempotency | `x-fern-idempotency-headers`, `"x-fern-idempotent": true`, a hidden header field | `notes.create(body, { idempotencyKey })` |
| Multipart upload | `RawBody multipart.Form` and the form's schema | `files.uploadFile({ file, note })` |
| Webhooks, signed | `doc.Webhooks`; `x-fern-webhook-signature`; `crypto/hmac` in the handler | `WebhooksHelper.verifySignature(...)` |
| A WebSocket both ways | `asyncapi.Operation` on the GET, `asyncapi.SendOperation` on a POST to the same path | A typed `sendSubscribe()` (TypeScript) |
| Audiences | `x-fern-audiences`; a group with `audiences: [public]` | A public SDK without internal operations |

The spec only describes: the server must enforce auth, and idempotency is the handler's job.

## Release

`mise run release -- vX.Y.Z` ships the SDKs, the specs and the CLI for every OS from your machine: [Release](release.md).

## Update charter

```sh
mise up --bump github:joeblew999/charter                    # the tool; then the same version in ?ref= of the includes in mise.toml
go get -u github.com/joeblew999/charter/go && go mod tidy   # the Go library
mise run workflows && mise run check
```

In TypeScript, change the version (it is there twice) in the URL of `@charter/ts` in `package.json`, then `npm install`. A project whose `mise.toml` still holds every task, or pins `"go:github.com/joeblew999/charter/cmd/charter"`: make a new project with `charter new` and take its `[tools]`, `CHARTER_TOOL` and `[task_config]` lines, then delete the tasks that `tasks/` now defines.

Releases are `v0`: anything can change between them. A project made before v0.8.0 (from orpc-api) does not update: make a new one and move `api/`, `migrations/` and your bindings into it.
