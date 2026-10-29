# charter-notes-ts

The notes API in TypeScript on Cloudflare Workers, from an oRPC contract (`api/contract.ts`), made with [charter](https://joeblew999.github.io/charter/): auth (tokens, Cloudflare Access, OpenID Connect), SSE and WebSocket streams that lose nothing across deploys, and rate limits. `charter check` is every check that needs no Cloudflare account (`mise run charter:check`; in charter's repo, `go tool charter check`), and `charter.toml` says everything charter knows about it.
