---
title: MCP
nav_order: 3
parent: Plans
---

# Plan: the contract as MCP tools

Goal: the schemas written once in the contract serve REST, the specs and MCP tools (Model Context Protocol), on Cloudflare and natively. Done for the Go server (`api-go/humamcp`, `/api/mcp`); the oRPC server is next. What was measured is in [findings.md](../findings.md) ("MCP from the Huma contract").

## Status (2026-10-01)

- `api-go/humamcp` serves the Huma operations as tools at `POST /api/mcp`, in the Worker (TinyGo) and in the native build. No MCP SDK.
- Tested with real clients in both protocol eras, natively and under workerd (`test/mcp-test.mjs`, inside `mise run api-go:check`).
- Not deployed. Not tried with a model behind the client.

## Does the approach hold up?

Yes. MCP over Streamable HTTP is JSON-RPC in one POST with one JSON answer, and a tools-only server needs nothing else. It got easier while this was being built: **revision 2026-07-28 made MCP stateless** (no `initialize` handshake, no `Mcp-Session-Id`, no GET stream; every request names its version and capabilities in `params._meta`, and `server/discover` reports the server). That is exactly what workers-go allows, since it starts a fresh Go runtime per request.

The official Go SDK is not an option on Workers: v1.8.0 doesn't compile with TinyGo 0.42 (its JSON Schema dependency pulls in `hash/maphash`, whose Go 1.27 implementation needs runtime internals TinyGo doesn't have).

## Design, and why

| Decision | Why |
|---|---|
| A tool call builds an `http.Request` and runs it through `humaworkers.API.ServeHTTP` | One code path: Huma's validation, `Resolve`, middleware and the handler are the REST ones. Nothing is validated twice or differently |
| Both eras on one endpoint: 2026-07-28 (stateless), and 2025-11-25 / 2025-06-18 (`initialize`, `ping`) | Clients are mid-migration: the TypeScript client 2.x speaks both, the 1.x SDK and most hosts still open with `initialize`. The handshake costs nothing to answer statelessly, because a server may decline to give a session id |
| Arguments are flat: parameters and the JSON body's properties in one object | It is what a model writes most easily, and it is oRPC's input shape, so both servers can expose the same tool schemas. A parameter and a body property with one name is an error at `tools/list`; a body that isn't an object is the argument `body` |
| `$ref` becomes `#/$defs/...`, with the schemas included per tool | A tool's schema must stand alone. JSON Schema 2020-12 is MCP's default dialect, and the TypeScript client and the Inspector accepted it |
| `outputSchema` and `structuredContent` only for object responses | The handshake-era revisions only allow objects there (2026-07-28 allows any JSON value). The text block always carries the body |
| A 4xx/5xx is a result with `isError` and Huma's problem as text; only "no such tool" and malformed calls are JSON-RPC errors | The spec's split: a model can correct a validation error if it sees it (`errors[].location`), not a protocol error |
| Streams are not tools: `watchNotes` (SSE) and `liveNotes` (WebSocket) are left out by default (an operation that is `Hidden` or answers `text/event-stream`) | A tool call is one request and one answer, and this server opens no stream for progress. `humamcp.Expose(op, true|false)` in `Operation.Metadata` overrides it, as `asyncapi.Operation` marks channels |
| `humaworkers.Route.OperationID` and `API.Operation(id)` | A tool call registers only its own operation, as a REST request does. `tools/list` registers them all |
| Annotations from the method: GET is `readOnlyHint`; others say whether they overwrite or delete | Hosts use them to decide what needs confirmation |
| `GET` and `DELETE` are 405; a foreign `Origin` is 403; one message per POST, at most 1 MB | The spec's rules for a server with no stream and no session |

Spec pages relied on:

- <https://modelcontextprotocol.io/specification/2026-07-28/basic> (messages, `resultType`, `_meta`, error codes, JSON Schema usage)
- <https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning> (eras, `UnsupportedProtocolVersionError`)
- <https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http> (POST, 202, 404/-32601, `MCP-Protocol-Version`, `Mcp-Method`, `Mcp-Name`, `HeaderMismatch`, 405 for GET/DELETE, `Origin`)
- <https://modelcontextprotocol.io/specification/2026-07-28/server/discover>, <https://modelcontextprotocol.io/specification/2026-07-28/server/tools>, <https://modelcontextprotocol.io/specification/2026-07-28/changelog>
- The handshake (`initialize`, version negotiation) is from the 2025-11-25 revision as I know it and as the two TypeScript clients exercise it; that page was not re-read for this work.

## Limits

- **Authorization: none of its own.** The endpoint is as open as the REST API, and `tools/list` is always open. The `Authorization` header is passed to the operation, so an operation that checks a bearer token would check it for tool calls too, but MCP's own scheme (OAuth protected-resource metadata, `401` with `WWW-Authenticate`) is not implemented. Needed before this fronts anything private.
- **`Origin`:** only "the same host as the request" is accepted. That stops other sites' pages; it is not a `Host` allowlist, which is what DNS rebinding against a local native server needs.
- **Streaming tools: none.** Answers are `application/json`, so no progress notifications, no `subscriptions/listen` (the tool list only changes with a deploy), and no long-running calls. If agents need "what's new", add a plain operation to the contract (notes after an id, with a limit: `Store.Since` exists) rather than exposing the stream; it would then be REST, an SDK method and a tool at once.
- **Resources and prompts: none.** GET operations could also be resources, but tools already cover them and hosts support tools best.
- **Sessions: none, by design.** Nothing in the handshake era required one for tools. A server that needs state across calls should hand out an explicit handle as a tool result, which is also what 2026-07-28 says.
- **Not implemented from 2026-07-28:** multi-round-trip results (`input_required`), `x-mcp-header` parameters, pagination of `tools/list`, caching beyond the `ttlMs`/`cacheScope` hints. The older revisions 2025-03-26 and 2024-11-05 are not offered (they allow batches).
- **What a call can't carry:** bodies that aren't JSON (multipart, files); response headers (dropped); a path parameter containing `/`.
- **`humamcp` needs a `*humaworkers.API`** (for `Operation(id)` and lazy registration). A plain Huma API would need a small interface instead.

## Existing libraries (a quick search, 2026-10-01)

Nothing found that does Huma -> MCP, and nothing aimed at TinyGo.

| What | Fit |
|---|---|
| `github.com/modelcontextprotocol/go-sdk` v1.8.0 (official) | Tried: doesn't compile with TinyGo 0.42 for Wasm. It is also session-shaped where a Worker wants none |
| `github.com/mark3labs/mcp-go` | Not tried (the owner's experience: it fails under TinyGo on Cloudflare) |
| `github.com/evcc-io/openapi-mcp` (`openapi2mcp`) | Go, OpenAPI -> MCP tools. Known only from its description, not tried: it starts from a spec document rather than from Huma's operations. Whether it builds with TinyGo depends on the MCP library under it, which wasn't checked |
| `higress-group/openapi-to-mcpserver` | Generates configuration for the Higress gateway, not a Go server |
| `kioie/tiny-go-mcp-server` | A toolkit on the official Go SDK ("tiny" is not TinyGo) |

Going through OpenAPI (spec -> tools -> HTTP calls back to the API) would also work as a separate proxy, but in the same process it is a detour: `humamcp` reads the operations Huma already holds and calls the handler directly.

## The oRPC side

oRPC has no official MCP package (none on npm under `@orpc/`; `@orpc/ai-sdk` makes AI SDK tools, not an MCP server). There is a third-party `orpc-mcp` 0.1.3 (peer `@orpc/* ^2.0.0-beta.16`, "serve an oRPC router as an MCP server"), not tried. The equivalent of `humamcp` would be `api/src/mcp.ts`, built like `api/src/asyncapi.ts` on oRPC's public APIs: walk the contract's procedures; a tool per procedure that isn't an event iterator (so not `watch`, not `live`), named by its OpenAPI `operationId` so the names match the Go server's (`hello`, `listNotes`, `createNote`); `inputSchema` and `outputSchema` from the Zod schemas through `ZodToJsonSchemaConverter`, which is already flat because oRPC's input is one object; `tools/call` by calling the procedure in-process with the request's context, turning an `ORPCError` (and oRPC's 400 for bad input) into an `isError` result. The transport can be the same hundred lines of JSON-RPC, or the official TypeScript SDK's server side, which should run on Workers (not tried). `test/mcp-test.mjs` takes a URL, so it would then run against both servers, and a tools/list comparison could join `TestSameSurfaceAsTheORPCContract`. Expect small schema differences to settle (Huma names schemas and writes `format: int64`; Zod inlines), as with the OpenAPI specs.

## Next

1. Deploy `api-go` and run `test/mcp-test.mjs` against it (`mise run api-go:deploy` needs a go-ahead); then connect a real host (Claude, an IDE) and watch a model use the tools.
2. `api/src/mcp.ts` for the oRPC Worker, and the same test against it.
3. Authorization, when an API that needs it uses this.
