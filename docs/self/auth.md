---
title: Auth
nav_order: 6
---

# Auth: who may call, and how they prove it

The contract says which scopes each operation needs; one middleware checks every request against it: `charter/auth` in Go, `charter-lib/auth` in TypeScript. In the notes examples, reading is public and writing needs `write`.

## The ways in

| Caller | Proves it with | May do |
|---|---|---|
| A program you trust | a bearer token: a Worker secret (`READ_TOKEN`, `WRITE_TOKEN`) | the token's scopes |
| A person | a GitHub login through Cloudflare Access: the Worker checks the JWT Access sends (`Cf-Access-Jwt-Assertion`) | `People` |
| A machine | its own Access service token (`CF-Access-Client-Id`, `CF-Access-Client-Secret`) | `Machines` |
| A user of an app | an access token from an OpenID Connect issuer (`Authorization: Bearer`), checked against the issuer's keys | its `scope` claim |
| A client of the API's own OAuth endpoint | a bearer token the API issued (client credentials): `auth.Issued` in Go, `issued` in TypeScript | what its check says |

No credentials, or ones that do not verify: 401. Known, without the scope: 403. A handler finds the caller with `auth.CallerOf(ctx)` in Go, `context.caller` in TypeScript. An unset setting trusts no one that way. JWTs must be RS256, ES256 or EdDSA, with the right issuer, audience and `exp` (a minute's leeway).

## In the contract

```go
auth.Scheme(config.OpenAPI)              // bearer tokens
auth.AccessScheme(config.OpenAPI, Title) // the service token's headers
auth.OIDCScheme(config.OpenAPI)          // an OpenID Connect issuer
Security: auth.Needs("write")            // in an operation: write, by any of them
var Trusted = []auth.Trusted{            // api/handlers.go: who may call
	auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
	auth.Access{People: []string{"read", "write"}, Machines: []string{"read"}},
	auth.OIDC{},
}
```

In TypeScript: `base: oidcScheme(accessScheme(info.title, scheme()))` in the document, `spec: needs(["write"])`, and `trusted` in `serve`'s options (`examples/notes-ts/api/handlers.ts`). The specs then list every way in for each operation, and a public one says `security: []`.

## Bearer tokens

```sh
openssl rand -hex 32 | fnox set WRITE_TOKEN -k "<name> WRITE_TOKEN" -p keychain
openssl rand -hex 32 | fnox set READ_TOKEN -k "<name> READ_TOKEN" -p keychain
```

They are the Worker's secrets (`[secrets]` in `charter.toml`): every `fnox exec -- charter deploy production` sets them from the environment. Local runs use throwaway ones (`[local] vars`). Nothing prints a value. A repo whose CI deploys also keeps them as GitHub secrets ([Deploy and CI](deploy.md#ci-on-github)).

## Cloudflare Access

In Cloudflare's dashboard (Zero Trust, Access, Applications), make a self-hosted application for the Worker's host: a policy for the people who may log in, and a Service Auth policy for each machine's service token (made in the same place: Access, Service credentials). Then give the Worker the application's two values, and deploy:

```sh
echo "https://<team>.cloudflareaccess.com" | fnox set ACCESS_TEAM_DOMAIN -k "<name> ACCESS_TEAM_DOMAIN" -p keychain
echo "<the application's audience tag>" | fnox set ACCESS_AUD -k "<name> ACCESS_AUD" -p keychain
fnox exec -- charter deploy production   # sets them: they are [secrets] optional
```

A machine keeps its service token as `<WORKER>_ACCESS_CLIENT_ID` and `_SECRET`, which the SDKs and the live tests read. Behind Access every request needs it, even a public one: without it Access answers with a redirect to its login, and the Worker never sees the request. Access's own settings live in Cloudflare; a deploy does not change them.

## An OpenID Connect issuer

Set `OIDC_ISSUER` (its discovery document, `<issuer>/.well-known/openid-configuration`, must name the same issuer) and `OIDC_AUDIENCE` in fnox: every deploy sets them.

## How a client finds out how to get in

Every 401 carries `WWW-Authenticate: Bearer resource_metadata="<the API>/.well-known/oauth-protected-resource"` (a 403 adds `error="insufficient_scope"` and the scope). That document (RFC 9728) names the API, its scopes and the servers it trusts. An API that issues its own tokens also says how at `/.well-known/oauth-authorization-server` (RFC 8414). The libraries serve all of it ([The standard API](standard.md)), and at `/api/mcp` a tool call refused this way answers with that 401 or 403.

## From the SDKs

| SDK | Bearer or OIDC token | Access service token |
|---|---|---|
| Go | `option.WithToken(token)` | `<WORKER>_ACCESS_CLIENT_ID` and `_SECRET`, or `option.WithAccessClientID`, `WithAccessClientSecret` |
| TypeScript | `bearer: { token }`; with OAuth, `oauth: { clientId, clientSecret }` | both as `headers` ([fern#17775](upstream.md)); for a public WebSocket channel, `auth: false` |
| CLI | the flag or variable its `--help` names | it sends only one of the two, so it cannot pass Access ([fern#17996](upstream.md)) |

## Rate limits

How often each caller may call: [Rate limits](ratelimits.md).

## Proven

- `npm run check` (2026-10-15): `examples/conformance-ts/test/auth.ts` serves a test issuer as Access and as an OIDC issuer, and checks 200, 401 and 403 for each way in, then 429 with `Retry-After`, against examples/conformance-go natively and under workerd, and examples/conformance-ts under workerd. It does not run on Cloudflare, where an OIDC issuer has not been tried; rate limits have ([Rate limits](ratelimits.md)).
- On Cloudflare, 2026-10-03: a Go notes project behind Access. No token, or a wrong secret: a redirect to the login. The machine's token reads; writing is 403. The write token writes; the read token, 403; a JWT from an untrusted issuer, 401.

## Limits

`People` and `Machines` are the same for every person and every machine; finer rules go in the handler, by its caller.
