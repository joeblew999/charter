---
title: Change the contract
nav_order: 2
parent: Guides
---

# Change the contract

How to add an operation to a Go project: its route, input and output, validation, errors, SDK names, pagination and storage. The worked example is `GET /api/notes/{id}`. Why the contract is the source: [Contract first](../concepts/contract-first.md).

The route and the handler below were run natively on 2026-10-01, with the outputs shown, but for the route's `Errors` line.

## The loop

```sh
mise run spec     # writes fern/openapi.json and fern/asyncapi.json from the contract
mise run check    # fails on a stale spec, a lint error, a failing test or a TinyGo gap
```

| File | What it holds |
|---|---|
| `api/contract.go` | The contract: one entry in `Routes` per operation, and the input and output structs |
| `api/handlers.go` | The code that runs |
| `api/store.go`, `api/store_js.go` | The storage |

## Add an operation

**1. The input struct,** in `api/contract.go`. Each tag says where a value comes from and what it must be:

```go
type GetInput struct {
	ID int64 `path:"id" minimum:"1" example:"42" doc:"The note's id"`
}
```

| Tag | Meaning |
|---|---|
| `path:"id"`, `query:"cursor"`, `header:"X-Trace-Id"` | Where the value comes from |
| a `Body` field | The JSON body: a struct whose fields have `json:"..."` tags |
| `minimum`, `maximum`, `minLength`, `maxLength`, `pattern`, `enum` | The rule. Huma enforces it before the handler runs |
| `default:"20"` | The value when the caller sends none |
| `doc:"..."` | The description: in the spec, the SDK docs and the MCP tool |
| `example:"42"` | The example Fern shows. Required on every constrained request field (below) |
| `hidden:"true"` | Read by the server, left out of the spec |

**2. The route,** in the list `Routes` returns:

```go
{Method: http.MethodGet, Path: "/api/notes/{id}", OperationID: "getNote", Register: func(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getNote", Method: http.MethodGet, Path: "/api/notes/{id}",
		Summary: "Get one note by id", Tags: []string{"notes"},
		Errors:     []int{http.StatusNotFound},
		Extensions: sdk("notes", "get", nil),
	}, env.get)
}},
```

- **`OperationID`** is said on the route too, so one operation can be found without registering the others. It is the MCP tool's name; `Summary` is its description.
- **`Errors`** lists the statuses the handler itself answers with. 422 and 401 are declared for you.
- **`sdk("notes", "get", nil)`** names the SDK method: `client.notes.get()`, `notes get` in the CLI. The third argument carries other `x-fern-*` extensions.

**3. The handler,** in `api/handlers.go`. Huma has read and validated `in` already:

```go
func (env Env) get(ctx context.Context, in *GetInput) (*NoteOutput, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	note, err := store.Get(ctx, in.ID)
	if errors.Is(err, ErrNotFound) {
		return nil, huma.Error404NotFound("no note with that id")
	}
	if err != nil {
		return nil, err
	}
	return &NoteOutput{Body: note}, nil
}
```

**4. The storage:** `Get` on the store, returning `ErrNotFound` ([below](#store-the-data)). Then the loop above.

```sh
curl -si localhost:5174/api/notes/2
# HTTP/1.1 404 Not Found
# {"title":"Not Found","status":404,"detail":"no note with that id"}
curl -s localhost:5174/api/notes/0
# {"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected number >= 1","location":"path.id","value":0}]}
```

## Errors a caller sees

| Case | Status | Body |
|---|---|---|
| A tag's rule is broken | 422 | `application/problem+json`; `errors[].location` is `path.`, `query.`, `header.` or `body.` and the field |
| A rule the tags cannot say | 422 | The input's `Resolve(huma.Context) []error` method returns `huma.ErrorDetail` values. `CreateInput` has one |
| The handler chooses | any | `huma.Error404NotFound("...")`, `huma.Error409Conflict("...")`, `huma.NewError(status, "...")` |
| Any other error | 500 | |
| An unknown path; a known path with another method | 404; 405 | |

## Rules the generated clients need

- **Every constrained request field has an `example` tag,** and every example is valid against its own schema. Fern invents the examples the contract lacks, and an invented value gets a 422. `TestExamplesAreThereAndValid` holds the contract to it.
- **`int32`, not `int`, for a number:** Fern's Go SDK then types it `int`.
- **A cursor is a string,** even when it is a number inside: the generated CLI's `--page-all` stops on numeric ones.
- **Every operation that would be an MCP tool can be one.** `mise run spec` fails otherwise, and says why ([MCP](mcp.md)).
- **A fixed segment wins over a `{name}`:** `/api/notes/watch` and `/api/notes/{id}` live side by side.

## Paginate a list

`listNotes` is the pattern: the input has `Cursor string` and `Limit int32`, the output `Data []Note` and `NextCursor string` with `omitempty`, and the operation says which is which:

```go
Extensions: sdk("notes", "list", map[string]any{
	"x-fern-pagination": map[string]any{"cursor": "$request.cursor", "next_cursor": "$response.next_cursor", "results": "$response.data"},
}),
```

The SDK then fetches the next page itself: `for await (const note of await client.notes.list())`.

## Store the data

A handler asks `env.Store()` for a `Store` and calls its methods. Add the method to the interface in `api/store.go` and to both implementations, as the methods beside it are written:

| Implementation | Runs | File |
|---|---|---|
| `D1Store` | On Cloudflare and under `mise run dev`: D1, through the library's `d1` package | `api/store_js.go` |
| `MemStore` | `mise run run` and `go test`: memory | `api/store.go` |

`d1.Query` decodes each row from JSON, so the struct's `json` tags name the columns ([Go packages](../reference/packages.md#d1)). `SQLStore` is the same statements through `database/sql`, for a native build with a SQLite driver; the Worker does not use it.

**To change the tables,** add the next numbered file to `migrations/`. Never edit one that was applied.

```sh
mise run migrate:local    # to the local D1 of a running mise run dev
mise run migrate          # REMOTE: to the deployed database (mise run deploy does this too)
```

