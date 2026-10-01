---
title: MCP internals (humamcp)
nav_order: 6
parent: This repository
---
# MCP: the Go contract as tools

The Go Worker serves its contract a third way, beside REST and the specs: as MCP tools (Model Context Protocol) at `POST /api/mcp`, made from the same Huma operations by the package in `api-go/humamcp/`. Read this page to use the endpoint, to add it to another Huma API, or before changing the package. The oRPC Worker has no MCP endpoint; that and authorization are planned in [plans/mcp.md](plans/mcp.md).

## Using it

```sh
mise run api-go:run        # the Go API natively on :5174 (or mise run api-go:dev, under workerd)
mise run api-go:mcp-test   # the official TypeScript MCP client against it, in both protocol eras (test/mcp-test.mjs)
mise run api-go:live-test  # REMOTE: the real-time tests, then the same MCP test against the deployed Go Worker
```

By hand, with a tool this repo doesn't pin: `npx @modelcontextprotocol/inspector --cli http://localhost:5174/api/mcp --transport http --method tools/list`.

In another Huma project: `humamcp.Handler(api)` with a `*humaworkers.API`, mounted on one path (`api-go/api/handlers.go` shows it). Give each `humaworkers.Route` its `OperationID`, so a tool call registers only its own operation.

## What a client sees

- **Every operation that answers once is a tool:** `hello`, `listNotes`, `createNote`. The name is the `OperationID`; the description is the `Summary`, then the `Description`.
- **The arguments are the input struct, flat:** path, query, header and cookie parameters, and the properties of a JSON object body, in one `inputSchema`. `createNote` takes `{"body": "..."}`, as the oRPC procedure does. A body that isn't an object is the argument `body`. So is an object body with a property named like one of the parameters (`PUT /things/{id}` with the whole thing, `id` included, as its body): the tool takes `{"id": ..., "body": {...}}`.
- **The result is what the REST route answers:** the body as text, and also as `structuredContent` when it is a JSON object. The 2xx response's schema is the `outputSchema`, when it is an object.
- **A 4xx or 5xx is a tool result with `isError`** and Huma's problem as its text (`errors[].location`, e.g. `query.limit`), which a model can correct itself from. Only a call that can't be made is a JSON-RPC error: an unknown tool is -32602, an unknown method -32601, malformed JSON -32700.
- **Not tools:** `watchNotes` (SSE) and `liveNotes` (the WebSocket). A tool call is one request and one answer. `humamcp.Expose(op, true|false)` overrides the default for an operation.
- **Annotations come from the method:** GET is `readOnlyHint`; the others say whether they overwrite or delete (`destructiveHint` for all but POST, `idempotentHint` for PUT and DELETE).
- **Two protocol eras on the one endpoint:** the stateless revision 2026-07-28 (`server/discover`, the version in every request) and the handshake revisions 2025-11-25 and 2025-06-18 (`initialize`, `ping`). No session id is given in either.

## How it works, and why

| Decision | Why |
|---|---|
| No MCP SDK: the Streamable HTTP transport is written out (JSON-RPC in one POST, one `application/json` answer) | The official Go SDK doesn't compile with TinyGo ([findings.md](findings.md)), and a tools-only server needs nothing else |
| No state | A Go runtime serves only a few requests and two requests may be in two runtimes, so there can be no session. Revision 2026-07-28 made MCP stateless, and the handshake revisions let a server decline to give a session id |
| A tool call builds an `http.Request` and runs it through `humaworkers.API.ServeHTTP` | One code path: Huma's validation, `Resolve`, middleware and the handler are the REST ones. Nothing is validated twice or differently |
| Both eras are answered | Clients are mid-migration: the TypeScript client 2.x speaks both, and the 1.x SDK opens with `initialize` |
| Arguments are flat | It is what a model writes most easily, and it is oRPC's input shape, so both servers could expose the same tool schemas. When a parameter and a body property share a name, that one operation keeps its body under `body`, so both values have a place |
| An operation that cannot be a tool is left out of `tools/list` and logged; `humamcp.Check(api)` returns the reasons | One operation's problem must not take the other tools down, and it should be found before a client lists them: `TestEveryOperationCanBeItsMCPTool` (`api-go/api/contract_test.go`) and the spec command (`mise run api-go:spec`, `api-go:spec:check`) fail on it. Today the one reason is a parameter named `body` beside a body that is the argument `body`; rename it, or `humamcp.Expose(op, false)` |
| `$ref` becomes `#/$defs/...`, with the schemas included per tool | A tool's schema must stand alone. JSON Schema 2020-12 is MCP's default dialect |
| `outputSchema` and `structuredContent` only for object responses | The handshake-era revisions only allow objects there. The text block always carries the body |
| Streams are left out by default: an operation that is `Hidden` or answers `text/event-stream` | This server opens no stream for progress. `humamcp.Expose` in `Operation.Metadata` overrides it, as `asyncapi.Operation` marks channels |
| `humaworkers.Route.OperationID` and `API.Operation(id)` | A tool call registers only its own operation, as a REST request does. `tools/list` registers them all |
| Anything but `POST` is 405; a foreign `Origin` is 403; one message per POST, at most 1 MB | The spec's rules for a server with no stream and no session |
| `tools/list` and `server/discover` carry `ttlMs` of 5 minutes and `cacheScope: public` | They only change with a deploy |

Spec pages the package follows: the 2026-07-28 revision's [basic](https://modelcontextprotocol.io/specification/2026-07-28/basic), [versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning), [Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http), [discover](https://modelcontextprotocol.io/specification/2026-07-28/server/discover) and [tools](https://modelcontextprotocol.io/specification/2026-07-28/server/tools) pages, and for the handshake the 2025-11-25 [lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle) page.

## Limits

- **Authorization: none of its own.** The endpoint is as open as the REST API, and `tools/list` is always open. The `Authorization` header is passed on to the operation, so an operation that checks a bearer token checks it for tool calls too. MCP's own scheme (OAuth protected-resource metadata, a 401 with `WWW-Authenticate`) is not implemented.
- **`Origin`:** only the host the request came to is accepted. That stops other sites' pages; it is not a `Host` allowlist, which is what DNS rebinding against a local native server needs.
- **No stream.** Answers are `application/json`: no progress notifications, no list-changed subscriptions, no long-running calls.
- **No resources and no prompts.**
- **Not implemented from 2026-07-28:** multi-round-trip results (`input_required`), `x-mcp-header` parameters, pagination of `tools/list`, caching beyond the `ttlMs` and `cacheScope` hints. The revisions 2025-03-26 and 2024-11-05 are not offered (they allow batches).
- **What a call can't carry:** bodies that aren't JSON (multipart, files); response headers (dropped); a path parameter containing `/`.
- **It needs a `*humaworkers.API`** (for `Operation(id)` and lazy registration), not a plain Huma API.
- **Not tried with a model behind the client** (Claude, an IDE).

## What was verified

In [findings.md](findings.md), dated 2026-10-01: `test/mcp-test.mjs` passes 29/29 against the native build, against the Wasm under workerd, and against the deployed Go Worker; what the endpoint added to the Wasm; and what an MCP call costs next to a REST call.

## Other libraries looked at (2026-10-01)

Nothing found that makes MCP tools from Huma operations, and nothing aimed at TinyGo.

| What | Fit |
|---|---|
| `github.com/modelcontextprotocol/go-sdk` v1.8.0 (official) | Tried: doesn't compile with TinyGo 0.42 for Wasm. It is also session-shaped where a Worker wants none |
| `github.com/mark3labs/mcp-go` | Not tried |
| `github.com/evcc-io/openapi-mcp` | Go, OpenAPI to MCP tools. Known only from its description: it starts from a spec document rather than from Huma's operations |
| `higress-group/openapi-to-mcpserver` | Generates configuration for the Higress gateway, not a Go server |
| `kioie/tiny-go-mcp-server` | A toolkit on the official Go SDK ("tiny" is not TinyGo) |

Going through OpenAPI (spec, then tools, then HTTP calls back to the API) would work as a separate proxy. In the same process it is a detour: `humamcp` reads the operations Huma already holds and calls the handler directly.
