# Plan: generate the AsyncAPI spec from the oRPC contract

Tracked in issue #3. Part of [realtime.md](realtime.md) (rule 4: both transports are declared in the one contract).

Goal: `sdk/fern/apis/api/asyncapi.yml` is generated from `api/src/contract.ts`, like `openapi.json`, and never written by hand. First in this repo, then offered to oRPC (middleapi/orpc#2115).

## Status (2026-09-30)

Steps 1–3 are done: `api/src/asyncapi.ts` generates `sdk/fern/apis/api/asyncapi.json` (the hand-written `asyncapi.yml` is gone), `/api/notes/live` is served from the contract procedure, and `mise run api:check` fails on spec drift. Left: offer it upstream (step 4). The output is JSON, not YAML, so there's no extra dependency.

## Why it's doable now (oRPC 2.0.0-beta.40, checked 2026-09-30)

- No oRPC package generates AsyncAPI yet (#2115 has no maintainer reply).
- oRPC 2.0 has the pieces as public APIs:
  - **Metadata plugins:** `.meta(plugin)` with `{ name, init(meta) }`. `openapi()` is built the same way (`meta["~openapi"]`).
  - **Router walking:** `walkProcedureContractsAsync(router, (contract, path) => ...)` from `@orpc/server`.
  - **Schemas:** `ZodToJsonSchemaConverter.convert(schema, direction)` from `@orpc/zod`, the same converter `OpenAPIGenerator` uses.
- An `asyncapi()` metadata helper plus an `AsyncAPIGenerator` can be built on these, with no fork.

## Design

- The WebSocket channel becomes a contract procedure. Its output is `asyncIteratorObject(note)`: the messages a client receives.
  ```ts
  live: oc
    .meta(asyncapi({ channel: "liveNotes", address: "/api/notes/live", operationId: "receiveNote", summary: "..." }))
    .input(z.object({ after: z.string().optional() }))   // resume position (a note id), as a query parameter
    .output(asyncIteratorObject(note)),
  ```
- It stays out of `openapi.json` (the generator's `filter` option, or a flag in the metadata) and appears only in AsyncAPI.
- `AsyncAPIGenerator.generate(contract, { version: "3.0.0", base: { info, servers } })` works like this:
  - It walks the router, and every procedure with `~asyncapi` metadata becomes a channel.
  - The output schema (`asyncIteratorObject(x)`) becomes a `receive` operation with message payload `x`.
  - If there's an input schema, it becomes a `send` operation. That comes later; this API only receives.
  - Shared Zod schemas go into `components.schemas` (and `components.messages`).
  - A `spec` hook per channel, like `openapi({ spec })`, lets the contract add `x-fern-*` extensions.
- The server host comes from the deployed URL, as in `api:spec`.

## Steps

1. **Prototype in this repo: `api/asyncapi.ts`, roughly 100 lines.** Add the metadata helper and the generator, add `notes.live` to the contract, and have `api/spec.ts` write both specs. `mise run api:spec` then writes `asyncapi.yml` too.
   - **Done when:**
     - the generated file matches today's hand-written one;
     - `fern check` passes;
     - `sdk:gen api typescript` still gives `liveNotes.connect()`;
     - `api:live-test` passes 5/5.
2. **Serve the channel from the contract.** `/api/notes/live` is implemented by the router (`live` handler: a `publisher.subscribe()` loop, like `watch`). A small WebSocket adapter in the Worker sends each yielded value as plain JSON. The channel and its implementation are then type-checked against the same contract, so they can't drift apart.
   - The handler is `follow()` from [realtime.md](realtime.md), the same one `watch` uses.
   - This also fixes today's `asyncapi.yml` comment, which says the Durable Object holds the socket. The Worker does.
3. **Put it in `mise run check`.** Regenerate both specs and fail if either differs from the committed files.
4. **Offer it upstream (outward-facing; needs a go-ahead first).**
   - Post on #2115: the design above, a link to the working prototype, and the question whether they want it as `@orpc/asyncapi` or would rather keep it outside oRPC.
   - If they want a PR: port it to their repo style (package, tests, a docs page next to `openapi/specification`, and a playground). Match their API naming: `asyncapi()` metadata, `AsyncAPIGenerator` with `converters` / `version` / `base`.
   - If not: keep it here, or publish it as a small package.
5. **Write it up in FINDINGS.md** (verified results only).

## Open questions (to settle in step 1)

- Which AsyncAPI versions Fern accepts. 3.0.0 works today; check 3.1.
- Whether Fern needs extensions (group or method names) on channels, the way it does on OpenAPI operations.
- Whether SSE should also be described in AsyncAPI. AsyncAPI 3 lists `sse` as a protocol, but Fern gets SSE from OpenAPI already. Leave it out unless something needs it.
