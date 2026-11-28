# The API

The API is written once, as a contract in Go (`api/contract.go`). Its OpenAPI and AsyncAPI documents, its MCP tools and its SDKs come from that.

## With the commands

| When | Command | What it does for your API |
|---|---|---|
| You change `api/contract.go` | `dev`, running | it compiles, and the running Worker serves the operation |
| Before a push | `check --fix` | writes `fern/openapi.json` and `fern/asyncapi.json` from the Go types; `check` fails while the committed ones are behind |
| Before a push | `test` | the API called through its own generated Go SDK, on a real Worker |
| Before a release | `pack --prove` | a client in each language, written by Fern from the spec, proven by calling the Worker |
| Nothing | | the docs' page of the API is written from the spec: [the example's](https://charter-example.gedw99.workers.dev/docs/reference-api) |

**Checked before the app runs:** what an operation takes and answers (Huma writes the spec from the Go types, never the other way); the scopes it needs (values); its limit, whose binding, calls and seconds are `wrangler.jsonc`'s, said once.

**Not checked until it runs:** what a handler does with a request. And nothing checks that a change is safe for clients already out there: the API's row is the table's row, so a column you rename is a field renamed for every client. `check` shows the spec changed; whether that is acceptable is yours to decide.

## An operation

```go
{Method: http.MethodPost, Path: "/api/notes", OperationID: "createNote", Register: func(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "createNote", Method: http.MethodPost, Path: "/api/notes",
		Summary:  "Create a note", Tags: []string{"notes"},
		Security: charter.Needs(Write),
	}, env.create)
}},
```

`charter check --fix` writes `fern/openapi.json` and `fern/asyncapi.json` from the contract; `charter check` fails when they are behind it.

This site has no API of its own. The example's is live: [every operation of it, written from its contract](https://charter-example.gedw99.workers.dev/docs/reference-api), [its OpenAPI document](https://charter-example.gedw99.workers.dev/api/openapi.json), and [its notes](https://charter-example.gedw99.workers.dev/api/notes).

An app's docs get that page without anyone writing it: [what charter writes](11-docs.md#what-you-write-and-what-charter-writes).

## Who may call

Reading is public. Writing needs a scope, which a bearer token, Cloudflare Access or an OpenID Connect issuer gives. An unset secret trusts no one.

## Limits

An operation says how often one caller may call it. The Worker's Rate Limiting binding enforces it, and `charter check` holds `wrangler.jsonc` to the contract.

## SDKs

```sh
go tool charter pack -prove
```

Fern generates a client for each language `charter.toml` names. Each is proved in its own language, and then against the Worker itself: the program in `test/sdk/<language>/` is refused and let through as the contract says ([who may do what](14-sign-in.md)). It all runs in containers, so nothing of Rust or Python is on your machine. The Go client is committed in `sdk/go` (`charter pack -publish`), and this site's own tests use it as the client.

## MCP

Every operation is also an MCP tool, at `/api/mcp`.
