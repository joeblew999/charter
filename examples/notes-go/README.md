# charter-notes-go

The notes API in Go on Cloudflare Workers, from a Huma contract (`api/contract.go`), built with TinyGo, made with [charter](https://joeblew999.github.io/charter/): auth (tokens, Cloudflare Access, OpenID Connect), SSE and WebSocket streams that lose nothing across deploys, and rate limits. `charter check` is every check that needs no Cloudflare account (`mise run charter:check`; in charter's repo, `go tool charter check`), and `charter.toml` says everything charter knows about it.
