# The API

The API is written once, as a contract in Go (`api/contract.go`). Its OpenAPI and AsyncAPI documents, its MCP tools and its SDKs come from that.

## An operation

```go
{Method: http.MethodPost, Path: "/api/notes", OperationID: "createNote", Register: func(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "createNote", Method: http.MethodPost, Path: "/api/notes",
		Summary:  "Create a note", Tags: []string{"notes"},
		Security: charter.Needs("write"),
	}, env.create)
}},
```

`charter check --fix` writes `fern/openapi.json` and `fern/asyncapi.json` from the contract; `charter check` fails when they are behind it.

This site's API is live: [the OpenAPI document](/api/openapi.json), and [the notes](/api/notes).

## Who may call

Reading is public. Writing needs a scope, which a bearer token, Cloudflare Access or an OpenID Connect issuer gives. An unset secret trusts no one.

## Limits

An operation says how often one caller may call it. The Worker's Rate Limiting binding enforces it, and `charter check` holds `wrangler.jsonc` to the contract.

## SDKs

```sh
go tool charter pack -prove
```

Fern generates a client for each language `charter.toml` names, and each is proved in its own language. It all runs in containers, so nothing of Rust or Python is on your machine. The Go client is committed in `sdk/go` (`charter pack -publish`), and this site's own tests use it as the client.

## MCP

Every operation is also an MCP tool, at `/api/mcp`.
