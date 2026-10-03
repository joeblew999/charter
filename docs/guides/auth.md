---
title: Auth
nav_order: 5
parent: Guides
---

# Auth: who may call, and how they prove it

The contract says which scopes each operation needs; one middleware checks every request against it: `go/auth` in Go, `@charter/ts/auth` in TypeScript. In the notes examples, reading is public and writing needs `write`.

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

In TypeScript: `base: oidcScheme(accessScheme(info.title, scheme()))`, `spec: needs(["write"])`, and `await authorize(headers, securityOf(procedure), trusted)` in an oRPC middleware (`examples/notes-ts/src/index.ts`). The specs then list every way in for each operation, and a public one says `security: []`.

## Bearer tokens

```sh
openssl rand -hex 32 | fnox set WRITE_TOKEN -k "<name> WRITE_TOKEN" -p keychain
openssl rand -hex 32 | fnox set READ_TOKEN -k "<name> READ_TOKEN" -p keychain
mise run cloudflare:secrets    # copies them, and the Cloudflare credentials, to GitHub for CI
```

They are the Worker's secrets (`WORKER_SECRETS` in `mise.toml`, `bindings.secret()` in `cloudflare.config.ts`): every `mise run deploy` sets them, through `charter exec -secrets`, from fnox unless already set, as in CI. Local runs use throwaway ones. No task prints a value.

## Cloudflare Access

fnox needs `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `CF_ACCESS_TEAM_DOMAIN` and `CF_ACCESS_GITHUB_IDP_ID`.

```sh
mise run deploy                                  # the Worker must exist
mise run access:setup -- you@example.com         # who may log in with GitHub; again to change it
mise run access:token -- create <machine> fnox   # or a file instead of fnox, readable by you alone
mise run access:token -- list
mise run access:token -- revoke <machine>        # Access refuses it from then on; the others are untouched
mise run access:delete                           # the application and every token
```

- `access:setup` makes the Access application for `API_URL`'s host: each machine's token passes (Service Auth), the people named log in with GitHub. It sets the Worker's `ACCESS_TEAM_DOMAIN` and `ACCESS_AUD` at once, and keeps them in fnox with `ACCESS_APP_ID`, so every deploy sets them too (`WORKER_OPTIONAL_SECRETS`).
- A token is named `<worker>:<machine>` and lasts a year. With `fnox`, it is kept as `<WORKER>_ACCESS_CLIENT_ID` and `_SECRET`, what the SDKs and `mise run live-test` read.
- Behind Access every request needs the token, even one the contract calls public. Without it, Access answers 302 to the login, and the Worker never sees the request.

## An OpenID Connect issuer

Put `OIDC_ISSUER` (its discovery document is at `<issuer>/.well-known/openid-configuration`, and must name the same issuer) and `OIDC_AUDIENCE` in fnox, as the tokens are: every deploy sets them (`WORKER_OPTIONAL_SECRETS`).

## From the SDKs

| SDK | Bearer or OIDC token | Access service token |
|---|---|---|
| Go | `option.WithToken(token)` | `<WORKER>_ACCESS_CLIENT_ID` and `_SECRET`, or `option.WithAccessClientID`, `WithAccessClientSecret`. Every credential given goes on every request |
| TypeScript | `bearer: { token }` | both as `headers` (fern-api/fern#17775). A client with no credentials for the WebSocket channel passes `auth: false` |
| CLI | `NOTES_TOKEN` | the flags `--access-client-id`, `--access-client-secret` or the variables, but it sends only one of the two: it cannot pass Access yet |

## Proven

- Locally (`mise run check`): `test/auth-test.mjs` serves a test issuer as Access and as an OIDC issuer, and checks 200, 401 and 403 for each way in: the Go Worker natively and as TinyGo Wasm under workerd, the TypeScript one under workerd.
- On Cloudflare, 2026-10-03: a project made by `charter new` behind Access, through these tasks. No token: 302. A wrong secret: 302. The machine's token: reads; writing is 403. With the write token: writes; with the read token or a JWT from an untrusted issuer: 403 and 401. The live, SDK (Go and TypeScript) and MCP tests passed through Access. An OIDC issuer was not tried there.

## Limits

- `People` and `Machines` are the same for every person and every machine; finer rules go in the handler, by `CallerOf`.
- Access's settings reach a Worker deployed from GitHub only as `access:setup` set them; a deploy keeps them.
