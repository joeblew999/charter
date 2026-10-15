# charter-notes-go

The notes API in Go on Cloudflare Workers, from a Huma contract (`api/contract.go`), built with TinyGo, made with [charter-ts](https://joeblew999.github.io/charter/): auth (tokens, Cloudflare Access, OpenID Connect), SSE and WebSocket streams that lose nothing across deploys, and rate limits. `npm run check` is every local check, with no Cloudflare account; the steps are the scripts in `package.json`, and `charter.config.ts` says what they need.
