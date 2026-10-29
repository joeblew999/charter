---
title: Your API
nav_order: 3
---

# Your API

The contract is the source: the validation, the specs, the SDKs and the MCP tools follow from it. Add your operations beside the example's, then take the example's out.

```sh
npm run spec     # after every change to the contract, migrations/ or the queries
npm run check    # fails on a stale spec, a failing test, or (in Go) a TinyGo gap
```

## The files

| | TypeScript ([oRPC](https://orpc.dev), Zod) | Go ([Huma](https://huma.rocks)) |
|---|---|---|
| The contract | `api/contract.ts` | `api/contract.go` |
| The handlers | `api/handlers.ts` | `api/handlers.go` |
| The storage | `api/store.ts` (D1, with Drizzle on `api/tables.ts`) | `api/store.go` (`Store`, and `MemStore` for `go test` and `--native`), `api/store_d1.go` (D1, through `api/queries.go`) |
| The schema and its typed code | `migrations/*.sql`; `api/tables.ts`, written from them | `migrations/*.sql` and `api/queries.sql`; `api/queries.go`, written from them ([below](#the-database)) |
| The specs' document | `api/spec.ts` | `api/spec.go`: `API(Env)`, which `npm run spec` writes the specs from |
| The Worker | `worker.ts` | `main.go`, with the bindings in `platform_js.go`; `worker.mjs`, the Worker's entry. Its module (`go.mod`) is the project's name, on the Go library `github.com/joeblew999/charter` |
| The tests | `test/`, and unit tests beside the code (`api/*.test.ts`) | `test/`, the TypeScript twin's tests, with the schemas they check answers against (`test/schemas.ts`); unit tests in `api/*_test.go` |
| Cloudflare's | `cloudflare.config.ts` (the Worker, its database `DB`, and with a stream the hub `HUB`), `migrations/*.sql` | the same |
| charter's | `charter.config.ts` ([below](#charterconfigts)) | the same |

## Add an operation

1. **In the contract:** its method, path and `operationId` (also the MCP tool's name), its input with the rules each value follows, its output, and who may call it ([Auth](auth.md#in-the-contract)). In Go, struct tags say where a value comes from and its rules:

   ```go
   type GetInput struct {
   	ID int64 `path:"id" minimum:"1" example:"42" doc:"The note's id"`
   }
   ```

2. **The handler** gets its input already checked. To refuse, return Huma's error in Go (`huma.Error404NotFound("...")`), throw oRPC's in TypeScript; every refusal goes out as problem details ([The standard API](standard.md#refusals)).
3. **The storage:** a table change is the next numbered file in `migrations/`; never edit one that was applied. Then `npm run spec` writes the typed code again ([The database](#the-database)).
4. **The SDK's names:** `x-fern-sdk-group-name` and `x-fern-sdk-method-name` on the operation name `client.<group>.<method>()` (the example's `sdk(...)` helper). Every constrained field needs a valid example.

## The database

The schema is `migrations/*.sql`, the one place the tables are said: a deploy applies them to D1, `npm run dev` to its local database. From them `npm run spec` writes the code that reads and writes the tables, typed, as `db` in `charter.config.ts` says, and `npm run check` fails when it is stale.

| `db` | You write | `npm run spec` writes |
|---|---|---|
| `"drizzle"` (TypeScript) | The queries with [Drizzle](https://orm.drizzle.team), on `drizzle(env.DB)` | `api/tables.ts`: each table, as SQLite has it once the migrations ran |
| `"sqlc"` (Go) | The queries in `api/queries.sql`, each named (`-- name: CreateNote :one`, `:many` or `:exec`), with a comment that starts with its name as its doc | `api/queries.go`: a function for each, over `charter/d1`, its arguments and rows typed by [sqlc](https://sqlc.dev) against the schema. A row of a whole table is that table's type (`NotesRow`); a column that may be NULL is a pointer |

The Go is written over `charter/d1`, not `database/sql`, which D1 does not need: a statement is one call, and its rows come back as one string. The notes examples keep their notes this way.

## Write for Workers

- **No memory between requests.** State goes in D1 or a Durable Object.
- **No background work** after the answer; little work at start-up, which runs for every new instance.
- **In Go, TinyGo lacks parts of `reflect`, the disk and some of `net`:** `npm run check` runs the API under workerd, which finds it; `go test` cannot.

## MCP

Every operation that answers once is also a tool at `/api/mcp`, with the same checks and the same sign-in: a call is let in, refused and limited as the route is. A stream or a WebSocket is not a tool. To hide one: `mcp.NotATool(op)` in Go, `.meta(notATool())` in TypeScript.

```sh
claude mcp add --transport http billing-api https://billing-api.<your-subdomain>.workers.dev/api/mcp
```

## charter.config.ts

What charter needs to know about the project, typed: its Worker's name and URL, its port, its specs, its secrets (and throwaway local ones), and its tests: `test` runs against the API on your machine (`npm run check`), `live` against the deployed one (`npm run live-test`). Every field is in [`Config`](https://github.com/joeblew999/charter/blob/main/charter-ts/src/config.ts).

## Take the example out

| What | What to do |
|---|---|
| The contract, handlers and storage | Replace the notes; keep `hello` and the wiring |
| `migrations/0001_init.sql` | Replace it if never deployed; otherwise add a file |
| `test/` | Rewrite the notes checks |
| `charter.config.ts` | Your SDKs' name: `sdks.name`, then `npm run spec` |

Without a stream, remove the hub too: the `Hub` export, the `HUB` binding and the stream's code. `grep -rni note api test migrations charter.config.ts` shows what is left.
