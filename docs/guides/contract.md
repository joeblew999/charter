---
title: Define your API
nav_order: 1
parent: Guides
---

# Define your API

This page gets you a new operation in your Go API, from the first line to a passing `mise run check`: its route, its inputs and outputs, validation, errors, the names the generated SDKs give it, pagination and storage. It uses one worked example, `GET /api/notes/{id}`, added to the project that `dev new -name billing-api` made ([Getting started](../getting-started.md)). Every command and every output below was run on that project.

## The shape of an operation

The whole API is two files in your project:

| File | What it holds |
|---|---|
| `api-go/api/contract.go` | The contract: one entry in `Routes` per operation, and the input and output structs |
| `api-go/api/handlers.go` | The code that runs when the operation is called |

The contract is [Huma](https://huma.rocks) on top of Go's `net/http`. An operation is a Huma operation plus two structs. The struct tags are the validation rules and the OpenAPI schema, so they cannot drift apart. From this one definition come the validation, the OpenAPI and AsyncAPI specs, and the SDKs, the CLI and the docs that Fern makes from the specs.

Each entry in `Routes` has this shape:

```go
{Method: http.MethodGet, Path: "/api/notes/{id}", OperationID: "getNote", Register: func(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getNote", Method: http.MethodGet, Path: "/api/notes/{id}",
		Summary: "Get one note by id", Tags: []string{"notes"},
		Extensions: sdk("notes", "get", nil),
	}, env.get)
}},
```

- **`OperationID`** is the operation's name. Say it in both places (the route and the operation). It becomes the tool name for AI agents ([MCP](mcp.md)) and the default SDK method name.
- **`Tags`** group operations in the docs.
- **`Summary`** is the one line shown in the docs and given to AI agents as the tool's description.
- **`Extensions`** carry Fern's `x-fern-*` settings. `sdk(group, method, extra)` is a small helper already in `api-go/api/contract.go` that sets the SDK names (below).

## Add an operation: GET /api/notes/{id}

Four edits. Each is shown as it was made.

### 1. The input and output structs

In `api-go/api/contract.go`:

```go
type GetInput struct {
	ID int64 `path:"id" minimum:"1" doc:"The note's id"`
}
```

The tag on each field says where the value comes from:

| Tag | Where the value comes from |
|---|---|
| `path:"id"` | The `{id}` in the path |
| `query:"cursor"` | The query string, `?cursor=...` |
| `header:"X-Trace-Id"` | A request header |
| a `Body` field | The JSON body. `Body` is a struct, and its fields use `json:"..."` tags |

The output is the note, which the file already has as `NoteOutput` (a `Body` of type `Note`), so there is nothing to add.

### 2. The route

Add this entry to the list `Routes` returns, in the same file:

```go
{Method: http.MethodGet, Path: "/api/notes/{id}", OperationID: "getNote", Register: func(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getNote", Method: http.MethodGet, Path: "/api/notes/{id}",
		Summary: "Get one note by id", Tags: []string{"notes"},
		Extensions: sdk("notes", "get", nil),
	}, env.get)
}},
```

### 3. The handler

In `api-go/api/handlers.go` (add `"errors"` to its imports):

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

By the time the handler runs, Huma has already read and validated `in`. The handler returns an output, or an error.

### 4. The storage method

The handler needs `Get` on the store. That is in the next section, in full: add the three pieces there, then come back.

### 5. Regenerate the specs and check

```sh
mise run api-go:spec    # rewrites sdk/fern/apis/api-go/openapi.json and asyncapi.json from the contract
mise run api-go:lint    # gofmt and go vet, natively and for Wasm
mise run api-go:test    # the Go tests
```

After this change `api-go:test` fails in one place, and it is expected: `api-go/api/mcp_test.go` lists the operations that are tools for AI agents, and `getNote` is a new one. Run it, read the `got` it prints and put the new tool into `want` ([Expose the API to AI agents](mcp.md)). Then `mise run check` runs everything, including the spec drift check that fails if you forget `api-go:spec`.

### 6. Call it

```sh
mise run api-go:run                                    # in one shell: http://localhost:5174
```

In another shell (the outputs are from a run on another port):

```sh
curl -s -X POST localhost:5174/api/notes -H 'content-type: application/json' -d '{"body":"first"}'
# {"id":1,"body":"first","created_at":"2026-10-01 10:37:00"}

curl -si localhost:5174/api/notes/1
# HTTP/1.1 200 OK
# Content-Type: application/json
# {"id":1,"body":"first","created_at":"2026-10-01 10:37:00"}

curl -si localhost:5174/api/notes/2
# HTTP/1.1 404 Not Found
# Content-Type: application/problem+json
# {"title":"Not Found","status":404,"detail":"no note with that id"}
```

## Validation: what the tags say, and what a caller sees

Put the rule on the field and Huma enforces it before your handler runs.

| Tag | Rule |
|---|---|
| `minimum:"1"`, `maximum:"100"` | A number's range |
| `minLength:"1"`, `maxLength:"200"` | A string's length |
| `pattern:"^[a-z0-9-]+$"` | A string must match this regular expression |
| `enum:"short,full"` | One of these values |
| `default:"20"` | The value when the caller sends none |
| `doc:"..."` | The description in the spec and the docs |
| `required:"false"` | A header or query value the caller may leave out |

These are in your project already: `ListInput` has `minimum`, `maximum` and `default` on `Limit`, `CreateInput` has `minLength` on the body, and `WatchInput` has `pattern`. Use `int32`, not `int`, for a number: Huma then writes `format: int32` and the generated Go SDK types it as `int`.

A caller who breaks a rule gets HTTP 422 with `application/problem+json`, and `errors` says where. These are real answers from the example above (with `format` as an `enum` and a `pattern` on a header, added to `GetInput` for the test):

```sh
curl -s localhost:5174/api/notes/0
# {"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected number >= 1","location":"path.id","value":0}]}

curl -s 'localhost:5174/api/notes?limit=500'
# {"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected number <= 100","location":"query.limit","value":500}]}

curl -s 'localhost:5174/api/notes/1?format=long'
# {"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected value to be one of \"short, full\"","location":"query.format","value":"long"}]}

curl -s localhost:5174/api/notes/1 -H 'X-Trace-Id: BAD ID'
# {"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected string to match pattern ^[a-z0-9-]+$","location":"header.X-Trace-Id","value":"BAD ID"}]}
```

`location` is `path.`, `query.`, `header.` or `body.` and then the field. A person or an agent can correct the call from it.

### A rule the tags cannot say

Give the input a `Resolve` method. It runs after the tags pass. The notes API uses it to refuse a body that contains the stream's end marker (`api-go/api/handlers.go`):

```go
func (in *CreateInput) Resolve(huma.Context) []error {
	if strings.Contains(in.Body.Body, END) {
		return []error{&huma.ErrorDetail{Location: "body.body", Message: "must not contain " + END, Value: in.Body.Body}}
	}
	return nil
}
```

The caller gets the same 422, with your `Location` and `Message`:

```sh
curl -s -X POST localhost:5174/api/notes -H 'content-type: application/json' -d '{"body":"a [end-of-stream] b"}'
# {"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"must not contain [end-of-stream]","location":"body.body","value":"a [end-of-stream] b"}]}
```

### Return your own error status

A handler returns a Huma error to choose the status. `huma.Error404NotFound("...")` is the one used above. There is one per status (`huma.Error409Conflict`, `huma.Error401Unauthorized`, and so on), and `huma.NewError(status, "message")` takes any status. To report a problem with a field, give it details, as `list` does for a bad cursor:

```go
return nil, huma.Error422UnprocessableEntity("validation failed",
	&huma.ErrorDetail{Location: "query.cursor", Message: "not a cursor from next_cursor", Value: in.Cursor})
```

Any other error (the `return nil, err` lines) becomes a 500. Every answer is `application/problem+json`, and the SDKs get that error type.

## Name the SDK methods

Without names, the SDKs call the method after the operation (`client.getNote(...)`). Two extensions choose them:

```go
Extensions: sdk("notes", "get", nil),    // x-fern-sdk-group-name and x-fern-sdk-method-name
```

That is `client.notes.get(...)` in TypeScript, `c.Notes.Get(...)` in Go, and `billing-api notes get` in the CLI. Give every operation of one resource the same group.

## Paginate a list with a cursor

`listNotes` in `api-go/api/contract.go` is the pattern to copy:

```go
Extensions: sdk("notes", "list", map[string]any{
	"x-fern-pagination": map[string]any{"cursor": "$request.cursor", "next_cursor": "$response.next_cursor", "results": "$response.data"},
}),
```

The input has `Cursor string` (a `query` field) and `Limit int32`; the output has `Data []Note` and `NextCursor string` with `omitempty`. The generated SDK then returns something you loop over (`for await (const note of await client.notes.list())`) and fetches the next page itself.

Two rules, both from the code's comments. Make the cursor a string, even when it is a number inside: the generated CLI's `--page-all` stops on numeric cursors. And make `limit` an `int32`.

## Store the data

A handler asks `env.Store()` for a `Store` (`api-go/api/store.go`) and calls its methods. `Store` is an interface with two implementations:

| Implementation | Used when | Where |
|---|---|---|
| `MemStore` | `mise run api-go:run` and the Go tests | Memory. It is gone when the process stops |
| `SQLStore` | On Cloudflare, and under `mise run api-go:dev` | Cloudflare D1, through Go's `database/sql` |

Adding the `Get` method to both finishes step 4. In `api-go/api/store.go`, add to the interface and define the error:

```go
type Store interface {
	// Get returns the note with this id, or ErrNotFound.
	Get(ctx context.Context, id int64) (Note, error)
	// ... the methods already there
}

// ErrNotFound is what Get returns for an id that is not there.
var ErrNotFound = errors.New("not found")
```

The D1 version is plain SQL, using the helper `query` the file already has:

```go
func (s SQLStore) Get(ctx context.Context, id int64) (Note, error) {
	notes, err := s.query(ctx, "SELECT id, body, created_at FROM notes WHERE id = ?", id)
	if err != nil {
		return Note{}, err
	}
	if len(notes) == 0 {
		return Note{}, ErrNotFound
	}
	return notes[0], nil
}
```

The in-memory version:

```go
func (m *MemStore) Get(_ context.Context, id int64) (Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id < 1 || id > int64(len(m.notes)) {
		return Note{}, ErrNotFound
	}
	return m.notes[id-1], nil
}
```

Add `"errors"` to the file's imports.

### Change the tables: a migration

The tables are SQL files in `migrations/` at the root of your project (`migrations/0001_init.sql` holds the `notes` table). To change them, add the next file, for example `migrations/0002_note_title.sql`:

```sql
ALTER TABLE notes ADD COLUMN title TEXT;
```

Then apply it:

```sh
mise run api-go:migrate:local   # to the local D1 of a running api-go:dev
mise run api-go:migrate         # REMOTE: to your Cloudflare database (api-go:deploy does this too)
```

`MemStore` has no tables, so a new column there is a field in `Note`.

## Give every field an example

If the contract has no examples, Fern invents them when it writes the SDK docs, and the invented values can be ones your server rejects with a 422 (a number below a `minimum`, for instance). Say a real example on each field with Huma's `example` tag:

```go
type Note struct {
	ID        int64  `json:"id" example:"42"`
	Body      string `json:"body" example:"Buy milk"`
	CreatedAt string `json:"created_at" example:"2026-10-01 09:30:00"`
}

type GetInput struct {
	ID int64 `path:"id" minimum:"1" example:"42" doc:"The note's id"`
}
```

After `mise run api-go:spec`, the schema in `sdk/fern/apis/api-go/openapi.json` carries them (a real run):

```json
"Note": {
  "properties": {
    "body": { "examples": ["Buy milk"], "type": "string" },
    "id": { "examples": [42], "format": "int64", "type": "integer" }
  }
}
```

and the path parameter:

```json
{ "in": "path", "name": "id", "required": true,
  "schema": { "examples": [42], "format": "int64", "minimum": 1, "type": "integer" } }
```

Choose values that pass your own rules.

To check what the SDK documentation shows, generate the SDK (Docker is needed) and read the examples in the files it writes:

```sh
mise run sdk:gen api-go typescript   # into sdk/out/api-go/typescript
```

Open `sdk/out/api-go/typescript/README.md` and `reference.md` there, and look at the values in the code samples. I did not run Fern for this page, so what it prints for these fields is not shown here.

Adding an example to `Note` changes the schema AI agents get for the tools, so `api-go/api/mcp_test.go` needs its `want` updated again (see step 5).

## A stream route beside a get-by-id

`GET /api/notes/watch` (the SSE stream, [Real-time](streaming.md)) and `GET /api/notes/{id}` can live side by side, and your project has both in the example above. Fern prints a warning that the two paths conflict; it can be ignored. What was run: with both in place, `curl 'localhost:5174/api/notes/watch?after=0&seconds=1'` answered with the event stream, and `curl localhost:5174/api/notes/1` with the note. That is Go's router: a fixed segment (`watch`) wins over `{id}`. I did not run the Fern warning itself.

Because `id` is a number, `/api/notes/watch` is never a valid note id anyway.

## What to read next

- [Real-time: SSE and WebSocket](streaming.md): add a stream of your own.
- [Expose the API to AI agents](mcp.md): your new operation is already a tool.
- [Generate SDKs and a CLI](sdks.md): give the contract to other developers.
