---
title: Your API
nav_order: 1
parent: Guides
---

# Your API: change the contract, then take the notes out

A new project is the notes API. Add your operations beside the notes, then take the notes out. A project made with `charter new -empty` has only `hello`: add your operations, with nothing to take out. The contract, `api/contract.go`, is the source of the validation, specs, SDKs and MCP tools.

```sh
mise run spec     # after every contract change
mise run check    # fails on a stale spec, a failing test or a TinyGo gap
```

## Add an operation

1. **The input struct** in `api/contract.go`. Tags say where a value comes from (`path`, `query`, `header`, a `Body` field) and its rules (`minimum`, `maxLength`, `pattern`, `enum`, `default`):

   ```go
   type GetInput struct {
   	ID int64 `path:"id" minimum:"1" example:"42" doc:"The note's id"`
   }
   ```

2. **The route** in `Routes`: `Method`, `Path`, `OperationID` (the MCP tool's name), and a `huma.Register` call with `Summary`, `Errors` and `Extensions: sdk("notes", "get", nil)`, which names `client.notes.get()`. Copy `listNotes` for a paginated list.
3. **The handler** in `api/handlers.go` gets `in` validated; return `huma.Error404NotFound("...")` to refuse.
4. **The storage:** the method on `Store` in `api/store.go`, for `D1Store` (`api/store_js.go`, on Cloudflare) and `MemStore` (natively and in `go test`); a `-empty` project has none yet, so copy the three from `examples/notes-go/api/`. A table change is the next numbered file in `migrations/`.

For the generated clients: every constrained request field has a valid `example`; numbers are `int32`; a cursor is a string.

## Write for Workers

- **No memory between requests:** a Go runtime is replaced after some requests. State goes in D1 or a Durable Object.
- **No background goroutines,** and little work in package initialisers: they run each time a runtime starts.
- **A binding** is declared in `cloudflare.config.ts`, read in `platform_js.go`, stubbed in `platform_other.go`, and reaches handlers through `api.Env`.
- **TinyGo lacks parts of `reflect`, a disk and some of `net`:** `mise run test:workerd` finds it, `go test` cannot.

## Agents: MCP

Every operation that answers once is also a tool at `/api/mcp`, with the same validation and handler; `humamcp.Expose(op, false)` hides one. It has no authorization of its own.

```sh
claude mcp add --transport http billing-api https://billing-api.<your-subdomain>.workers.dev/api/mcp
```

## Take the notes out

| File | What to do |
|---|---|
| `api/contract.go`, `api/handlers.go`, `api/store.go`, `api/store_js.go` | Replace the notes; keep `hello`, `Handler` and `origin` |
| `migrations/0001_init.sql` | Replace it if never deployed; otherwise add a file |
| `api/*_test.go`, `test/` | Keep the helpers; rewrite the notes checks and the tool names |
| `fern/generators.yml` | `namespaceExport`, `packageName`, `binaryName`; keep the Go SDK's path `<your module>/sdk/go` |

Without a stream, also remove the hub: `Hub` in `Env`, `feed`, the `HUB` binding, the `Hub` export, and the live and soak programs. Then `grep -rni note api test migrations fern/generators.yml mise.toml` shows what is left.

## In TypeScript

`examples/notes-ts/` is the same API on oRPC (`src/contract.ts`, Zod), without a native run or MCP; invalid input gets a 400. `charter new -lang ts` makes a project from it, as [Getting started](../getting-started.md) does in Go: the tests it shares with the Go example go into `test/`, and `@charter/ts` comes from the release. Then `src/contract.ts` is the contract, `src/index.ts` the server and `mise run dev` runs it.
