# api: APIs from a contract

Write the contract in Go ([Huma](https://huma.rocks) operations); charter serves it and writes everything else from
it. [`examples/api`](../examples/api) is the smallest one.

| Package | What it is |
|---|---|
| `serve` | The standard API: the operations under `/api`, its specs at `/api/openapi.json` and `/api/asyncapi.json`, the operations as MCP tools at `/api/mcp` |
| `specs` | Writes the specs from the contract (`charter spec`) and says what they do not show |
| `humaworkers` | Huma on Workers: operations registered when first needed |
| `asyncapi` | The streams of a contract, as AsyncAPI |
| `mcp` | The operations as MCP tools |
| `auth` | Bearer tokens, Cloudflare Access and OIDC, as the contract declares them |
| `follow` | A stream of items over SSE or a WebSocket, none lost or sent twice |
| `problem` | Refusals as RFC 9457 problem details |
