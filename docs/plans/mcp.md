---
title: MCP
nav_order: 3
parent: Plans
grand_parent: This repository
---
# Plan: the contract as MCP tools

Goal: the schemas written once in the contract serve REST, the specs and MCP tools (Model Context Protocol), on Cloudflare and natively. The Go server's half is built: how it works, its limits and the libraries looked at are in [../mcp.md](../mcp.md), and what was measured is in [../findings.md](../findings.md). This page keeps what is left.

## Left

1. **Connect a real host** (Claude, an IDE) to the deployed Go Worker and watch a model use the tools. So far only test clients have called it.
2. **The oRPC side** (below), and `examples/notes-go/test/mcp-test.mjs` against it.
3. **Authorization,** when an API that needs it uses this: MCP's own scheme (OAuth protected-resource metadata, a 401 with `WWW-Authenticate`). Needed before the endpoint fronts anything private.
4. **A `Host` allowlist** for a native server that is reachable from a browser: the `Origin` check alone doesn't stop DNS rebinding.
5. **"What's new" for agents, as a plain operation.** If agents need the notes after an id, add an operation to the contract (with a limit; `Store.Since` exists) rather than exposing the stream. It would then be REST, an SDK method and a tool at once.
6. **A plain Huma API as input.** `humamcp` needs a `*humaworkers.API`; a small interface would let it take any Huma API.

Not planned unless something needs them: resources and prompts (tools already cover GET operations, and hosts support tools best), and sessions (a server that needs state across calls should hand out an explicit handle as a tool result, which is also what revision 2026-07-28 says).

## The oRPC side

oRPC has no official MCP package (none on npm under `@orpc/`; `@orpc/ai-sdk` makes AI SDK tools, not an MCP server). There is a third-party `orpc-mcp` 0.1.3 (peer `@orpc/* ^2.0.0-beta.16`, "serve an oRPC router as an MCP server"), not tried.

The equivalent of `humamcp` would be a file `mcp.ts` beside `examples/notes-ts/src/asyncapi.ts`, built like it on oRPC's public APIs:

- **Walk the contract's procedures.** A tool per procedure that isn't an event iterator (so not `watch`, not `live`), named by its OpenAPI `operationId`, so the names match the Go server's (`hello`, `listNotes`, `createNote`).
- **Schemas from Zod:** `inputSchema` and `outputSchema` through `ZodToJsonSchemaConverter`, which is already flat because oRPC's input is one object.
- **`tools/call` calls the procedure in-process** with the request's context, turning an `ORPCError` (and oRPC's 400 for bad input) into an `isError` result.
- **The transport** can be the same hundred lines of JSON-RPC, or the official TypeScript SDK's server side, which should run on Workers (not tried).

`examples/notes-go/test/mcp-test.mjs` takes a URL, so it would then run against both servers, and a `tools/list` comparison could join `TestTheNotesExamplesHaveTheSameSurface`. Expect small schema differences to settle (Huma names schemas and writes `format: int64`; Zod inlines), as with the OpenAPI specs.
