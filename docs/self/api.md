---
title: Your API
nav_order: 3
---

# Your API

The contract is the source: the validation, the specs, the SDKs and the MCP tools follow from it. Add your operations beside the example's, then take the example's out.

```sh
npm run spec     # after every change to the contract
npm run check    # fails on a stale spec, a failing test, or (in Go) a TinyGo gap
```

## The files

| | TypeScript ([oRPC](https://orpc.dev), Zod) | Go ([Huma](https://huma.rocks)) |
|---|---|---|
| The contract | `api/contract.ts` | `api/contract.go` |
| The handlers | `api/handlers.ts` | `api/handlers.go` |
| The storage | `api/store.ts` (D1) | `api/store.go` (`Store`, and `MemStore` for `go test` and `--native`), `api/store_d1.go` (D1) |
| The specs' document | `api/spec.ts` | `api/spec.go`, written by `cmd/spec` |
| The Worker | `worker.ts` | `main.go`, with the bindings in `platform_js.go` |
| The tests | `test/`, and unit tests beside the code (`api/*.test.ts`) | `test/`, the same as its TypeScript twin's; unit tests in `api/*_test.go` |
| Cloudflare's | `cloudflare.config.ts` (the Worker, its database `DB`, the hub `HUB`), `migrations/*.sql` | the same |
| charter's | `charter.config.ts` ([below](#charterconfigts)) | the same |

## Add an operation

1. **In the contract:** its method, path and `operationId` (also the MCP tool's name), its input with the rules each value follows, its output, and who may call it ([Auth](auth.md#in-the-contract)). In Go, struct tags say where a value comes from and its rules:

   ```go
   type GetInput struct {
   	ID int64 `path:"id" minimum:"1" example:"42" doc:"The note's id"`
   }
   ```

2. **The handler** gets its input already checked. To refuse, return Huma's error in Go (`huma.Error404NotFound("...")`), throw oRPC's in TypeScript; every refusal goes out as problem details ([The standard API](standard.md#refusals)).
3. **The storage:** a table change is the next numbered file in `migrations/`; never edit one that was applied.
4. **The SDK's names:** `x-fern-sdk-group-name` and `x-fern-sdk-method-name` on the operation name `client.<group>.<method>()` (the example's `sdk(...)` helper). Every constrained field needs a valid example.

## Write for Workers

- **No memory between requests.** State goes in D1 or a Durable Object.
- **No background work** after the answer; little work at start-up, which runs for every new instance.
- **In Go, TinyGo lacks parts of `reflect`, the disk and some of `net`:** `npm run check` runs the API under workerd, which finds it; `go test` cannot.

## MCP

Every operation that answers once is also a tool at `/api/mcp`, with the same checks and the same sign-in: a call is let in, refused and limited as the route is. A stream or a WebSocket is not a tool. To hide one: `mcp.Expose(op, false)` in Go, `.meta(notATool())` in TypeScript.

```sh
claude mcp add --transport http billing-api https://billing-api.<your-subdomain>.workers.dev/api/mcp
```

## charter.config.ts

What charter needs to know about the project, typed. The example's:

```ts
import { defineConfig } from "charter-ts/config";
import { specs } from "./api/spec.ts";

export default defineConfig({
	worker: "billing-api",
	url: process.env.API_URL ?? "https://billing-api.you.workers.dev",
	port: 5178,
	specs,
	test: { run: ({ origin, node }) => node("test/live.ts", origin) },
	live: ({ url, node }) => node("test/live.ts", url),
});
```

`test` runs against the API on your machine (`npm run check`), `live` against the deployed one (`npm run live-test`). Every field is in [`Config`](https://github.com/joeblew999/charter/blob/main/charter-ts/src/config.ts): the secrets, throwaway local ones, programs to serve beside the API for a test (a webhook's receiver), and the most the Worker may be as a deploy uploads it (3 MB gzipped by default, what Workers Free allows).

## Take the example out

| What | What to do |
|---|---|
| The contract, handlers and storage | Replace the notes; keep `hello` and the wiring |
| `migrations/0001_init.sql` | Replace it if never deployed; otherwise add a file |
| `test/` | Rewrite the notes checks |
| `fern/generators.yml` | Your SDKs' names: `namespaceExport`, `packageName`, `binaryName` |

Without a stream, remove the hub too: the `Hub` export, the `HUB` binding and the stream's code. `grep -rni note api test migrations fern/generators.yml charter.config.ts` shows what is left.

## In Go

`charter-ts new <name> --go` makes a Go project: its module is `<name>` (change it in `go.mod` and the imports), and it imports the Go library of the same release, `github.com/joeblew999/charter/go/lib`. Its tests against a server are TypeScript, the same as its TypeScript twin's, in `test/`, with the schemas they check answers against in `test/schemas.ts`.
