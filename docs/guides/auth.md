---
title: Auth
nav_order: 5
parent: Guides
---

# Auth: who may call, and how they prove it

The contract says which scopes each operation needs; one middleware checks every request against it: `go/lib/auth` in Go, `"charter-lib/auth"` in TypeScript. In the notes examples, reading is public and writing needs `write`.

## Three ways in

| Caller | Proves it with | Checked by | May do |
|---|---|---|---|
| A program you trust | a bearer token: a Worker secret (`READ_TOKEN`, `WRITE_TOKEN`) | the Worker | the token's scopes |
| A person | a GitHub login through Cloudflare Access | Access at the edge, then the Worker: the JWT Access sends (`Cf-Access-Jwt-Assertion`) against the team's keys and the application's AUD tag | `People` |
| A machine | its own Access service token (`CF-Access-Client-Id`, `CF-Access-Client-Secret`) | the same; the JWT names the token's Client ID | `Machines` |
| A user of an app | an access token from an OpenID Connect issuer (`Authorization: Bearer`) | the Worker, against the issuer's keys, found by its discovery document | its `scope` claim |

No credentials, or credentials that do not verify: 401. Known, without the scope: 403. The handler finds the caller (email, Client ID, subject) with `auth.CallerOf(ctx)` in Go, `context.caller` in TypeScript. An unset setting trusts no one that way.

JWTs are verified with [go-jose](https://github.com/go-jose/go-jose) and [jose](https://github.com/panva/jose): RS256, ES256 or EdDSA only; the issuer, the audience and `exp` must be right, with a minute's leeway; keys are kept for an hour, and a token with an unknown key makes one new fetch, at most every 30 s.

## In the contract

```go
auth.Scheme(config.OpenAPI)              // bearer tokens
auth.AccessScheme(config.OpenAPI, Title) // the service token's headers; SDKs read <TITLE>_ACCESS_CLIENT_ID and _SECRET
auth.OIDCScheme(config.OpenAPI)          // openIdConnectUrl: the API's /.well-known/openid-configuration (auth.Discovery)
Security: auth.Needs("write")            // in an operation: write, by any of them
routes.UseMiddleware(auth.Middleware(routes, env.Var,
	auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
	auth.Access{People: []string{"read", "write"}, Machines: []string{"read"}},
	auth.OIDC{}))
```

In TypeScript: `base: oidcScheme(accessScheme(info.title, scheme()))`, `spec: needs(["write"])`, and `await authorize(headers, securityOf(procedure), trusted)` in an oRPC middleware (`ts/notes/src/index.ts`). The specs then list every way in for each operation, and a public one says `security: []`.

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

## From the SDKs

| SDK | Bearer or OIDC token | Access service token |
|---|---|---|
| Go | `option.WithToken(token)` | `<WORKER>_ACCESS_CLIENT_ID` and `_SECRET`, or `option.WithAccessClientID`, `WithAccessClientSecret`. Every credential given goes on every request |
| TypeScript | `bearer: { token }` | both as `headers` (fern-api/fern#17775). A client with no credentials for the WebSocket channel passes `auth: false` |
| CLI | `NOTES_TOKEN` | the flags `--access-client-id`, `--access-client-secret` or the variables, but it sends only one of the two: it cannot pass Access yet |

## Rate limits

A limited operation takes at most `limit` calls per caller in each `period` (10 or 60 s); one more is 429 with `Retry-After`. The caller is who auth let in (service token, subject, email, or the token's secret), or the client's IP (`CF-Connecting-IP`) on a public operation; a call auth refuses is not counted. In the notes examples, creating a note takes 100 a minute.

```go
ratelimit.Declare(config.OpenAPI, ratelimit.Limit{Binding: "READ_LIMIT", Limit: 600, Period: 60, Scope: "read"}) // per scope; none: only operations' own
Extensions: ratelimit.On(ratelimit.Limit{Binding: "WRITE_LIMIT", Limit: 100, Period: 60}, nil)                   // in an operation: its own
routes.UseMiddleware(ratelimit.Middleware(routes, nil))                                                        // after auth.Middleware
```

In TypeScript (`"charter-lib/ratelimit"`): `spec: needs(["write"], limited(writeLimit))`, `rateLimits` in openapiSpec's options for scopes, and `await rateLimit(limitOf(procedure), keyOf(caller, headers), env)` after `authorize` (`ts/notes/src/index.ts`).

- Each limit is a [Workers Rate Limiting](https://developers.cloudflare.com/workers/runtime-apis/bindings/rate-limit/) binding of that name. `cloudflare.config.ts` declares one per `x-rate-limit` in `fern/openapi.json`, with its limit and period, so they are written once; it holds each binding's namespace, a number unique in the account.
- The binding counts per Cloudflare location and is eventually consistent: it stops abuse, it does not account. `cf dev` counts locally. Natively, Go counts in memory.
- A Worker without the binding lets every call through, and logs it once.

## Proven

- Locally (`npm run check`): the auth test (`test/auth-test.mjs` in Go, `test/auth-test.ts` in TypeScript) serves a test issuer as Access and as an OIDC issuer, and checks 200, 401 and 403 for each way in, then 429 with `Retry-After` after the write limit for one caller while another still writes: the Go Worker natively and as TinyGo Wasm under workerd, the TypeScript one under workerd. Rate limits were not tried on Cloudflare.
- On Cloudflare, 2026-10-03: a Go notes project behind Access, set up by the Access commands charter's Go tool had then. No token: 302. A wrong secret: 302. The machine's token: reads; writing is 403. With the write token: writes; with the read token or a JWT from an untrusted issuer: 403 and 401. The live, SDK (Go and TypeScript) and MCP tests passed through Access. An OIDC issuer was not tried there.

## Limits

- `People` and `Machines` are the same for every person and every machine; finer rules go in the handler, by `CallerOf`.
- Access's settings reach a Worker deployed from GitHub only as `access:setup` set them; a deploy keeps them.
