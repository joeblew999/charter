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

## Refusals

Every refusal is [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details, `application/problem+json`, the same from Go and from TypeScript:

```json
{"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected number >= 1","location":"query.limit","value":0}]}
```

They are checked in this order, the first that fails answers:

| Status | When | `detail`, `errors` |
|---|---|---|
| 404 | No route has the path (a trailing slash included) | `not found` |
| 405 | The path has other methods: `Allow` names them, sorted | `method not allowed` |
| 401, 403 | No credentials the API knows, or not the scope: `WWW-Authenticate` says how to get in | Why, e.g. `credentials are required: ...` |
| 429 | Over the operation's limit: `Retry-After` | `too many calls: ...` |
| 415 | A body of a media type the operation does not take | `validation failed`, at `body` |
| 400 | A required body that is empty, or JSON that does not parse | `request body is required`; `validation failed` with `the body is not valid JSON` |
| 422 | The contract refuses the input: each error says where (`query.limit`, `body.note`, `path.id`) | `validation failed`, in Huma's words (`expected required property body to be present`) |
| 426 | A channel's address without a WebSocket upgrade, once let in | `expected a WebSocket upgrade` |

A handler that refuses what the contract lets through says where: `refused("query.cursor", "not a cursor from next_cursor", value)` in TypeScript, `huma.Error422UnprocessableEntity` with an `ErrorDetail` in Go. At `/api/mcp`, a refused tool call's text is the same problem details, and one refused for its credentials is that 401 or 403 itself.

## The document

Both libraries write the OpenAPI document the same way, so the SDKs Fern makes from either are the same: OpenAPI 3.1.0; a request or response body inline (as Fern names its type from the operation); every operation's refusals declared as problem details (401 when it needs credentials, 422 when it takes input, its 429 when it is limited, any other it declares, and a default); a parameter's description on it and on its schema; a file as `contentMediaType: application/octet-stream`; examples on every field.

## Held the same

Each API that is here in TypeScript and in Go (conformance, notes, start) answers the same in both, with the TypeScript project as the reference:

```sh
npx charter-ts twins ts/conformance go/conformance
```

It runs both under workerd, as deployed, and sends both the requests the reference's specs give rise to: for each operation the right one and each kind of wrong one, for each channel one that is no upgrade, and what every API serves. It prints every answer that differs and fails if one does; the `twins` job runs it for each pair. The requests come from the spec, so a new route is covered without editing anything.

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
