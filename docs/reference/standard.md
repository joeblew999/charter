---
title: The standard API
nav_order: 1
parent: Reference
---

# The standard API: what every API serves, and where

Every API charter serves has the same shape, in Go and in TypeScript. The library does the wiring: `go/lib/serve` in Go, `charter-lib/serve` in TypeScript. A project gives its contract, its handlers and whom it trusts.

| Path | What | Only when |
|---|---|---|
| `/api/...` | The contract's routes, each call allowed and limited as its contract says (401, 403 with the challenge, 429 with `Retry-After`) | |
| `/api/openapi.json` | The OpenAPI spec, with the request's origin as its server | |
| `/api/asyncapi.json` | The AsyncAPI spec of the WebSocket channels | |
| `/api/mcp` | The routes that answer once, as MCP tools | |
| A channel's address, e.g. `/api/notes/live` | The WebSocket. Its token may come as `?access_token=`, on the channel only, for a browser or a Worker | The contract has a channel |
| `/.well-known/oauth-protected-resource`, `/.well-known/oauth-authorization-server`, `/.well-known/openid-configuration` | How a client gets in ([Auth](../guides/auth.md#how-a-client-finds-out-how-to-get-in)) | The API trusts anyone |

What else is the same in every project:

| What | Name |
|---|---|
| The D1 database | binding `DB` |
| The hub (a Durable Object) | class `Hub`, binding `HUB` |
| A rate limit | a Workers Rate Limiting binding named as in the contract (`WRITE_LIMIT`); its counter is named from the Worker's name (`rateLimits` from `"charter-ts/cloudflare"`) |
| The npm scripts | `dev`, `check`, `spec`, `sdk`, `deploy`, `live-test`; in Go also `start`, and `sdk:publish` for a committed Go SDK |
| What the tests wait for | `/api/openapi.json` answering |

`projects.test.ts` fails if a project's scripts differ, or two projects use one package at different versions.

## Go

```go
// api/spec.go
func API(env Env) serve.API {
	return serve.API{Config: config, Routes: Routes(env), LiveTitle: LiveTitle, Setting: env.Var, Trusted: Trusted}
}

// api/handlers.go
func Handler(env Env) http.Handler { return API(env).Handler() }

// cmd/spec/main.go
spec := api.API(api.Env{})
specfile.Must(spec.Tools())
specfile.Main(spec.OpenAPI, spec.AsyncAPI)
```

`Trusted` empty: no credentials are checked and `/.well-known/` is 404. `Document` adds what no route says (the webhooks).

## TypeScript

```ts
const os = implement(contract).$context<Served>();
const api = os.use(guard);
export const router = os.router({ hello: api.hello.handler(() => ({ message: "hi" })) });
const served = serve(router, { document, trusted: () => trusted, limits: () => env });
export default { fetch: request => served(request, {}) } satisfies ExportedHandler;
```

`document` is the API's `info`, its channels' `asyncInfo`, the OpenAPI `base` (security schemes) and its `webhooks`; `specsOf(contract, document)` gives `charter.config.ts`'s `specs`. The handler finds its caller at `context.caller`. The router is built from `os`, without the guard, or oRPC runs it twice per call. A channel whose input is the client's messages takes them from the socket; a refused message closes it with 1008, a feed that gives up with 1011.
