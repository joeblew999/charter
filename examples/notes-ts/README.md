# charter-notes-ts

The notes API in TypeScript on Cloudflare Workers, from an oRPC contract (`api/contract.ts`), made with [charter-ts](https://joeblew999.github.io/charter/): auth (tokens, Cloudflare Access, OpenID Connect), SSE and WebSocket streams that lose nothing across deploys, and rate limits. `npm run check` is every check that needs no Cloudflare account; the commands are the scripts in `package.json`, and `charter.config.ts` says what they need.
