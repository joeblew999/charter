---
title: AsyncAPI generator
nav_order: 2
parent: Plans
grand_parent: This repository
---
# Plan: the AsyncAPI generator, upstream

Tracked in issue #3 (the comment at the top of `api/ts/src/asyncapi.ts` says #4; which is right was not checked). The generator itself is built: `api/ts/src/asyncapi.ts` writes `sdk/fern/apis/api-ts/asyncapi.json` from the oRPC contract, the WebSocket is served from the contract procedure, and `mise run api:ts:check` fails on spec drift. How it works and its limits are in [../api.md](../api.md#the-asyncapi-generator). This page keeps what is left: getting it out of this repo.

## Left

1. **Publish it as a community package,** as promised on middleapi/orpc#2115 ([next.md](next.md)): `asyncapi()` and `AsyncAPIGenerator`, API-compatible with `openapi()` and `OpenAPIGenerator` (`converters`, `version`, `base`). The `send` side and the OpenAPI `webhooks` writer in `api/ts/src/specs.ts` belong in it.
2. **If oRPC wants it as `@orpc/asyncapi`:** port it to their repo style (package, tests, a docs page next to `openapi/specification`, a playground).
3. **If oRPC ships its own:** switch to it and delete the file, as its `Upstream:` tag says.

Posting upstream is outward-facing and needs a go-ahead first.

## Open questions

- **Which AsyncAPI versions Fern accepts.** 3.0.0 works; 3.1 is not checked.
- **Whether Fern needs extensions on channels** (group or method names), the way it does on OpenAPI operations.
- **Whether SSE should also be described in AsyncAPI.** AsyncAPI 3 lists `sse` as a protocol, but Fern gets SSE from OpenAPI already. Leave it out unless something needs it.
- **Query parameters and a send side on one channel.** The Go generator (`api/go/asyncapi/`) allows both; this one doesn't yet.
