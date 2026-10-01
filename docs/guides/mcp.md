---
title: Expose the API to AI agents (MCP)
nav_order: 4
parent: Guides
---

# Expose the API to AI agents (MCP)

This page gets an AI agent (Claude Code, an IDE, your own program) calling your API's operations as tools, and shows how to choose which operations it sees. Your project from `dev new` needs no setup for this: the API already serves [MCP](https://modelcontextprotocol.io) (Model Context Protocol) at `/api/mcp`.

## What `/api/mcp` is

One URL that speaks MCP over HTTP: a client POSTs JSON-RPC messages and gets JSON answers. It lists your operations as tools, and a tool call runs the same operation as the REST route, with the same validation and the same handler. There is nothing to add to your handlers.

It is mounted in `api-go/api/handlers.go` (`humamcp.Handler(routes)`), next to the specs. Try it against the API running natively (`mise run api-go:run`, port 5174). These are real answers, from a project with the notes API plus the `getNote` operation of [Define your API](contract.md):

```sh
curl -s -X POST localhost:5174/api/mcp -H 'content-type: application/json' -H 'accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"getNote","arguments":{"id":1}}}'
# {"id":1,"jsonrpc":"2.0","result":{"content":[{"text":"{\"id\":1,\"body\":\"first\",\"created_at\":\"2026-10-01 10:38:01\"}","type":"text"}],"structuredContent":{"id":1,"body":"first","created_at":"2026-10-01 10:38:01"}}}
```

## Which operations become tools

By default, every operation that has an `OperationID` and answers once. The tool's name is the `OperationID`; its description is the `Summary`, then the `Description`. In the notes API:

| Operation | Tool? | Why |
|---|---|---|
| `hello`, `listNotes`, `createNote` | yes | One request, one answer |
| `watchNotes` (SSE) | no | A stream is not one answer |
| `liveNotes` (WebSocket) | no | The same |

An operation marked `Hidden`, or one that answers `text/event-stream`, is left out.

### Opt one in or out: `humamcp.Expose`

Wrap the operation in `humamcp.Expose(op, true)` or `humamcp.Expose(op, false)`. To keep `hello` from agents, in `api-go/api/contract.go` (add `github.com/joeblew999/orpc-api/api-go/humamcp` to the imports, next to `humaworkers`):

```go
huma.Register(api, humamcp.Expose(huma.Operation{
	OperationID: "hello", Method: http.MethodGet, Path: "/api/hello",
	Summary: "Say hello", Tags: []string{"meta"},
	Extensions: sdk("meta", "hello", nil),
}, false), env.hello)
```

After that, `tools/list` answered `['listNotes', 'getNote', 'createNote']` (a real run: `hello` gone, `getNote` there by default). `Expose(op, true)` on a streaming operation makes it a tool; it must then end by itself, because the whole response becomes the result.

Hidden from agents is not hidden from callers: `Expose(false)` only changes the tool list. The REST route is still there.

## How a tool's input comes from the operation

The tool's `inputSchema` is the operation's input struct, flattened into one object: path, query, header and cookie parameters, and the properties of a JSON object body, side by side. The validation tags go with them. For `getNote`, with `GetInput` as in the [contract guide](contract.md) (a real `tools/list` answer):

```json
{"additionalProperties": false, "properties": {"X-Trace-Id": {"description": "Your id for this call", "pattern": "^[a-z0-9-]+$", "type": "string"}, "format": {"default": "short", "description": "How much of the note to return", "enum": ["short", "full"], "type": "string"}, "id": {"description": "The note's id", "examples": [42], "format": "int64", "minimum": 1, "type": "integer"}}, "required": ["id"], "type": "object"}
```

`createNote` takes `{"body": "..."}`: the body's properties, not a nested `body` object. A parameter and a body property with the same name is an error at `tools/list`. The 2xx response's schema is the tool's `outputSchema`, and the result comes back as text and as `structuredContent`.

So write `doc` tags and `example` tags on your fields: they are what the model reads to decide how to call the tool.

When the input is refused, the tool result has `isError` set and Huma's problem as its text, so a model can correct itself (a real answer for `id: 0`):

```
{"id":3,"jsonrpc":"2.0","result":{"content":[{"text":"{\"title\":\"Unprocessable Entity\",\"status\":422,\"detail\":\"validation failed\",\"errors\":[{\"message\":\"expected number \\u003e= 1\",\"location\":\"path.id\",\"value\":0}]}","type":"text"}],"isError":true}}
```

## Connect a client

The address is `http://localhost:5174/api/mcp` natively, and `https://billing-api.<your-subdomain>.workers.dev/api/mcp` (the URL `mise run api-go:deploy` printed) once deployed.

**Claude Code**, a remote HTTP server:

```sh
claude mcp add --transport http billing-api https://billing-api.<your-subdomain>.workers.dev/api/mcp
```

or in a `.mcp.json` file at the root of the project that should use it:

```json
{
  "mcpServers": {
    "billing-api": {
      "type": "http",
      "url": "https://billing-api.<your-subdomain>.workers.dev/api/mcp"
    }
  }
}
```

I did not run this with Claude Code or any model for this page; the endpoint is only tested with the clients below.

**The MCP Inspector, from the command line** (a tool the repo does not pin; it is fetched when run):

```sh
npx @modelcontextprotocol/inspector --cli http://localhost:5174/api/mcp --transport http --method tools/list
```

**The official TypeScript client**, as `test/mcp-test.mjs` in your project uses it (the package `@modelcontextprotocol/client`, which it loads from the packages installed in the `sdk/` folder):

```js
import { Client, StreamableHTTPClientTransport } from "@modelcontextprotocol/client";

const client = new Client({ name: "my-agent", version: "1.0.0" });
await client.connect(new StreamableHTTPClientTransport(new URL("http://localhost:5174/api/mcp")));
const { tools } = await client.listTools();
const result = await client.callTool({ name: "createNote", arguments: { body: "from an agent" } });
```

The endpoint speaks two protocol revisions on the one URL: the stateless 2026-07-28 and the handshake ones (2025-11-25 and 2025-06-18), with no session id in either. A client picks the one it knows. The snippet above is the shape of what `test/mcp-test.mjs` does, which takes the package from the `sdk/` folder's packages; the snippet itself was not run.

## Check it

```sh
mise run api-go:run        # in one shell
mise run api-go:mcp-test   # in another: the official client against it; it creates test notes
```

`api-go:mcp-test` runs `test/mcp-test.mjs` in both protocol eras. It checks the tool list, that each schema comes from the contract, that a tool call answers what the REST route answers, that refused input is an `isError` result with its location, and that a stream (`watchNotes`) is not a tool. It lists the tools it expects by name (`hello,listNotes,createNote`), so after you add, hide or expose an operation, update that list in `test/mcp-test.mjs` and the one in `api-go/api/mcp_test.go`. I did not run `api-go:mcp-test` for this page; the repo's findings record it passing 29 of 29 against the native build, under workerd and on a deployed Worker (2026-10-01). Against the deployed Worker, `mise run api-go:live-test` runs it with the real-time tests.

## What it does not do

- **No authorization of its own.** The endpoint is as open as the REST API, and `tools/list` is always open. The `Authorization` header is passed on to the operation, so an operation that checks a bearer token checks it for a tool call too, but MCP's own scheme (OAuth metadata, a 401 with `WWW-Authenticate`) is not implemented. Do not put an endpoint on the internet that gives an agent a tool you would not give a stranger.
- **No streaming tools.** Answers are one JSON document: no progress messages, no long-running calls.
- **No resources and no prompts.** Tools only.
- **No file bodies.** A call carries JSON: multipart uploads are not tools, and response headers are dropped.
- **Only a `*humaworkers.API`** can be served. `api-go/api/handlers.go` does it for you.

The package, its decisions and the full list of limits are in the orpc-api repo's [MCP page](../mcp.md).
