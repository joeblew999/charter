---
title: Auth
nav_order: 5
parent: Guides
---

# Auth: who may call, and how they prove it

The contract says which scopes each operation needs; one middleware checks every request against it: `go/lib/auth` in Go, `"charter-lib/auth"` in TypeScript. In the notes examples, reading is public and writing needs `write`.

## The ways in

| Caller | Proves it with | Checked by | May do |
|---|---|---|---|
| A program you trust | a bearer token: a Worker secret (`READ_TOKEN`, `WRITE_TOKEN`) | the Worker | the token's scopes |
| A person | a GitHub login through Cloudflare Access | Access at the edge, then the Worker: the JWT Access sends (`Cf-Access-Jwt-Assertion`) against the team's keys and the application's AUD tag | `People` |
| A machine | its own Access service token (`CF-Access-Client-Id`, `CF-Access-Client-Secret`) | the same; the JWT names the token's Client ID | `Machines` |
| A user of an app | an access token from an OpenID Connect issuer (`Authorization: Bearer`) | the Worker, against the issuer's keys, found by its discovery document | its `scope` claim |
| A client of the API's own OAuth endpoint | a bearer token the API issued (client credentials) | the API's own check: `auth.Issued{Scheme, Check}` in Go, `issued: { scheme, check }` in TypeScript | what that check says |

The notes examples take the first four. The conformance projects take all five: Fern's OAuth client credentials beside charter's ways, each operation's security naming OAuth with the same scopes (`needs` in `go/conformance/api/contract.go` and `ts/conformance/src/contract.ts`).

No credentials, or credentials that do not verify: 401. Known, without the scope: 403. The handler finds the caller (email, Client ID, subject) with `auth.CallerOf(ctx)` in Go, `context.caller` in TypeScript. An unset setting trusts no one that way.

JWTs are verified with [go-jose](https://github.com/go-jose/go-jose) and [jose](https://github.com/panva/jose): RS256, ES256 or EdDSA only; the issuer, the audience and `exp` must be right, with a minute's leeway; keys are kept for an hour, and a token with an unknown key makes one new fetch, at most every 30 s.

## In the contract

```go
auth.Scheme(config.OpenAPI)              // bearer tokens
auth.AccessScheme(config.OpenAPI, Title) // the service token's headers; SDKs read <TITLE>_ACCESS_CLIENT_ID and _SECRET
auth.OIDCScheme(config.OpenAPI)          // openIdConnectUrl: the API's /.well-known/openid-configuration (auth.Discovery)
Security: auth.Needs("write")            // in an operation: write, by any of them
serve.API{..., Setting: env.Var, Trusted: []auth.Trusted{ // api/spec.go: who may call
	auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
	auth.Access{People: []string{"read", "write"}, Machines: []string{"read"}},
	auth.OIDC{},
}}
```

In TypeScript: `base: oidcScheme(accessScheme(info.title, scheme()))` in the document, `spec: needs(["write"])`, and `trusted` in `serve`'s options, whose `guard` runs `authorize` before each call (`ts/examples-notes/src/index.ts`). The specs then list every way in for each operation, and a public one says `security: []`.

## Bearer tokens

```sh
openssl rand -hex 32 | fnox set WRITE_TOKEN -k "<name> WRITE_TOKEN" -p keychain
openssl rand -hex 32 | fnox set READ_TOKEN -k "<name> READ_TOKEN" -p keychain
fnox exec -- sh -c 'gh secret set WRITE_TOKEN --body "$WRITE_TOKEN" && gh secret set READ_TOKEN --body "$READ_TOKEN"'   # for the deploy workflow
```

They are the Worker's secrets (`secrets` in `charter.config.ts`, `bindings.secret()` in `cloudflare.config.ts`): every `npm run deploy` sets them from the environment, `fnox exec --` here and GitHub's secrets in CI. Local runs use throwaway ones (`localSecrets`). Nothing prints a value.

## Cloudflare Access

In Cloudflare's dashboard (Zero Trust, Access, Applications), make a self-hosted application for the Worker's host: a policy for the people who may log in (with GitHub), and a Service Auth policy for each machine's service token. Then give the Worker the application's two values, in fnox as the tokens are, and deploy:

```sh
echo "https://<team>.cloudflareaccess.com" | fnox set ACCESS_TEAM_DOMAIN -k "<name> ACCESS_TEAM_DOMAIN" -p keychain
echo "<the application's audience tag>" | fnox set ACCESS_AUD -k "<name> ACCESS_AUD" -p keychain
fnox exec -- npm run deploy    # sets them: they are optionalSecrets in charter.config.ts
```

- A machine's service token is kept as `<WORKER>_ACCESS_CLIENT_ID` and `_SECRET` (in fnox), what the SDKs and the live tests read.
- Behind Access every request needs the token, even one the contract calls public. Without it, Access answers 302 to the login, and the Worker never sees the request.

## An OpenID Connect issuer

Put `OIDC_ISSUER` (its discovery document is at `<issuer>/.well-known/openid-configuration`, and must name the same issuer) and `OIDC_AUDIENCE` in fnox, as the tokens are: every deploy sets them (`optionalSecrets` in `charter.config.ts`).

## How a client finds out how to get in

A refusal says how to get in, as MCP's authorization asks: every 401 carries `WWW-Authenticate: Bearer resource_metadata="<the API>/.well-known/oauth-protected-resource"`, and a 403 adds `error="insufficient_scope", scope="write"`. That document (RFC 9728) names the API, its scopes and the authorization servers it trusts, in the order a client should try them: the API itself when it issues tokens, the OpenID Connect issuer, the Cloudflare Access team. An API that issues its own tokens (`auth.Issued{..., TokenPath: "/api/oauth/token"}` in Go, `issued: { ..., tokenPath }` in TypeScript) also says how, at `/.well-known/oauth-authorization-server` (RFC 8414): its token endpoint, the client-credentials grant, and Basic or form credentials.

`go/lib/serve` and `charter-lib/serve` serve all of it for an API that trusts anyone ([The standard API](../reference/standard.md)): the documents at `/.well-known/`, the challenge on every refusal, and at `/api/mcp` a tool call refused that way as that 401 or 403 with its challenge, not a tool error. Underneath, in Go: `auth.WellKnown`, `auth.Middleware`, `humamcp`; in TypeScript: `wellKnown`, `authorize` (its `challenge`, set by the guard as `WWW-Authenticate` and in the `ORPCError`'s `data`), `mcpHandler`.

The conformance projects check the whole chain against both servers, natively, under workerd and on Cloudflare (`ts/conformance/test/mcp-test.ts`, `auth-test.ts`): a tool call without credentials is a 401 whose challenge points to the metadata, which points to the API's own authorization server, whose token endpoint gives a token for the client's credentials, with which the call goes through. MCP's own TypeScript client cannot follow the last steps by itself yet ([Upstream issues](../upstream.md)).

## From the SDKs

| SDK | Bearer or OIDC token | Access service token |
|---|---|---|
| Go | `option.WithToken(token)` | `<WORKER>_ACCESS_CLIENT_ID` and `_SECRET`, or `option.WithAccessClientID`, `WithAccessClientSecret`. Every credential given goes on every request |
| TypeScript | `bearer: { token }`; an API with OAuth too: `oauth: { clientId, clientSecret }` beside it | both as `headers` (fern-api/fern#17775). A client with no credentials for the WebSocket channel passes `auth: false` |
| CLI | `NOTES_TOKEN` | the flags `--access-client-id`, `--access-client-secret` or the variables, but it sends only one of the two: it cannot pass Access yet |

## Rate limits

A limited operation takes at most `limit` calls per caller in each `period` (10 or 60 s); one more is 429 with `Retry-After`. The caller is who auth let in (service token, subject, email, or the token's secret), or the client's IP (`CF-Connecting-IP`) on a public operation; a call auth refuses is not counted. In the notes examples, creating a note takes 100 a minute.

```go
ratelimit.Declare(config.OpenAPI, ratelimit.Limit{Binding: "READ_LIMIT", Limit: 600, Period: 60, Scope: "read"}) // per scope; none: only operations' own
Extensions: ratelimit.On(ratelimit.Limit{Binding: "WRITE_LIMIT", Limit: 100, Period: 60}, nil)                   // in an operation: its own
```

In TypeScript (`"charter-lib/ratelimit"`): `spec: needs(["write"], limited(writeLimit))`, and `rateLimits` in the document for scopes; `charter-lib/serve`'s guard counts after auth, on the bindings `serve`'s `limits` gives (`ts/examples-notes/src/index.ts`).

- Each limit is a [Workers Rate Limiting](https://developers.cloudflare.com/workers/runtime-apis/bindings/rate-limit/) binding of that name, after auth (`serve` does both). `cloudflare.config.ts` declares one per `x-rate-limit` in `fern/openapi.json`, with its limit and period, so they are written once: `...rateLimits(spec, name, bindings.rateLimit)` from `"charter-ts/cloudflare"`, which names each counter (a number unique in the account) from the Worker's name.
- The binding counts per Cloudflare location and is eventually consistent: it stops abuse, it does not account. `cf dev` counts locally. Natively, Go counts in memory.
- A Worker without the binding lets every call through, and logs it once.

## Proven

- Locally (`npm run check`): the auth test serves a test issuer as Access and as an OIDC issuer, and checks 200, 401 and 403 for each way in, then 429 with `Retry-After` after the write limit for one caller while another still writes. The test is `ts/conformance/test/auth-test.ts`, with OAuth tokens beside the others; it runs against go/conformance natively and as TinyGo Wasm under workerd, and ts/conformance under workerd. The notes examples are served by the same `serve`. Rate limits were not tried on Cloudflare.
- On Cloudflare, 2026-10-03: a Go notes project behind Access, set up by the Access commands charter's Go tool had then. No token: 302. A wrong secret: 302. The machine's token: reads; writing is 403. With the write token: writes; with the read token or a JWT from an untrusted issuer: 403 and 401. The live, SDK (Go and TypeScript) and MCP tests passed through Access. An OIDC issuer was not tried there.

## Limits

- `People` and `Machines` are the same for every person and every machine; finer rules go in the handler, by `CallerOf`.
- Access's settings reach a Worker deployed from GitHub only as `access:setup` set them; a deploy keeps them.
