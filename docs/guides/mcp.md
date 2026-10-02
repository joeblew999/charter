---
title: MCP
nav_order: 5
parent: Guides
---

# MCP: your operations as tools for agents

How an agent (Claude Code, an IDE, your own program) calls your [Huma](https://huma.rocks) API's operations as tools, and how to choose which it sees. A Go project needs no setup: it already serves [MCP](https://modelcontextprotocol.io) (Model Context Protocol) at `/api/mcp`. The TypeScript example has no MCP endpoint.

## What `/api/mcp` is

One URL that speaks MCP over HTTP: a client POSTs JSON-RPC and gets JSON back. A tool call runs the same operation as the REST route: the same validation, middleware and handler. It is mounted in `api/handlers.go` (`humamcp.Handler(routes)`).

```sh
mise run run     # natively: http://localhost:5174
curl -s -X POST localhost:5174/api/mcp -H 'content-type: application/json' -H 'accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"createNote","arguments":{"body":"from an agent"}}}'
```

## Which operations are tools

| Rule | In the notes API |
|---|---|
| Every operation with an `OperationID` that answers once is a tool | `hello`, `listNotes`, `createNote` |
| A stream is not: an operation that is `Hidden` or answers `text/event-stream` | `watchNotes` (SSE), `liveNotes` (the WebSocket) |
| `humamcp.Expose(op, false)` hides one; `humamcp.Expose(op, true)` makes a stream a tool, which must then end by itself | none |

Hidden from agents is not hidden from callers: the REST route stays.

## What a tool looks like

| Part | Where it comes from |
|---|---|
| Name | The `OperationID` |
| Description | The `Summary`, then the `Description` |
| `inputSchema` | The input struct, flat: path, query, header and cookie parameters and the properties of a JSON object body, side by side, with their validation tags. `createNote` takes `{"body": "..."}` |
| `outputSchema` | The 2xx response's schema, when it is an object |
| Result | The body as text, and as `structuredContent` when it is a JSON object |
| Annotations | From the method: GET is `readOnlyHint`; the others say `destructiveHint` (all but POST) and `idempotentHint` (PUT, DELETE) |
| A refused input | A tool result with `isError` and Huma's problem as its text (`errors[].location`), so a model can correct itself |

So write `doc` and `example` tags on your fields: they are what the model reads.

When a parameter and a body property share a name, that operation keeps its body under the argument `body`. An operation that cannot be a tool is left out and logged, and `mise run spec`, `mise run spec:check` and `TestEveryOperationCanBeItsMCPTool` fail on it, saying why.

## Connect a client

The address is `http://localhost:5174/api/mcp` natively and `<your Worker's URL>/api/mcp` once deployed.

```sh
claude mcp add --transport http billing-api https://billing-api.<your-subdomain>.workers.dev/api/mcp
npx @modelcontextprotocol/inspector --cli http://localhost:5174/api/mcp --transport http --method tools/list
```

```js
import { Client, StreamableHTTPClientTransport } from "@modelcontextprotocol/client";

const client = new Client({ name: "my-agent", version: "1.0.0" });
await client.connect(new StreamableHTTPClientTransport(new URL("http://localhost:5174/api/mcp")));
const { tools } = await client.listTools();
const result = await client.callTool({ name: "createNote", arguments: { body: "from an agent" } });
```

The endpoint speaks two protocol eras on the one URL: the stateless revision 2026-07-28, and the handshake revisions 2025-11-25 and 2025-06-18. No session id in either.

Not run for this page: the `curl` call above as written (the same call with another tool was), the `claude` and inspector commands, and the snippet, which is the shape of what `test/mcp-test.mjs` does. No model has used the tools yet: only test clients have.

## Check it

```sh
mise run run         # in one shell
mise run mcp-test    # in another: the official TypeScript client, in both eras. It writes test notes
```

`mise run check` runs the same program natively and under workerd, and `mise run live-test` against the deployed Worker. It passed 29 of 29 in all three places on 2026-10-01 ([Findings](../findings.md)). After you add, hide or rename an operation, update the tool names it expects in `test/mcp-test.mjs` and `api/mcp_test.go`.

## What it does not do

- **No authorization of its own.** The endpoint is as open as the REST API, and `tools/list` is always open. The `Authorization` header is passed on, so an operation that checks a bearer token checks it for a tool call too. MCP's own scheme is not implemented. Do not publish a tool you would not give a stranger.
- **Tools only:** no resources, no prompts, no stream, no progress notifications.
- **JSON only:** no file bodies. Response headers are dropped.
- **Only a `*humaworkers.API`** can be served, not a plain Huma API.

The package, its functions and why it uses no MCP SDK: [Go packages](../reference/packages.md#humamcp). What is planned: [What is next](../plans/next.md).
