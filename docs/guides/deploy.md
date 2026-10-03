---
title: Deploy and CI
nav_order: 4
parent: Guides
---

# Deploy and CI

How to put a project on Cloudflare Workers, test it there, and run the same from GitHub. Local checks cannot see everything Cloudflare's runtime does, so every deploy is followed by the live test.

## Deploy

```sh
npx cf auth login     # once per machine; or set CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID
mise run deploy       # builds the Wasm, deploys the Worker with its secrets, its D1 database and the hub, applies migrations
mise run live-test    # REMOTE, writes test notes: a few seconds after every deploy
```

- **`cloudflare.config.ts`** declares the Worker, the database `<name>-db` (binding `DB`), the hub (`HUB`) and `APP_NAME`. A second deploy keeps the database.
- **Migrations** are `migrations/*.sql`, applied in name order, each once: `mise run migrate` on Cloudflare, `mise run migrate:local` for `mise run dev`. Never edit one that was applied.
- **The URL is `API_URL`** in `mise.toml`: the specs name it and the remote tests call it. After changing it, `mise run spec`. `mise.local.toml` overrides it on one machine; do not run `mise run spec` then.
- **The build fails over 3,000,000 bytes gzipped,** the Workers Free limit.

## Tokens

Writing a note needs a token with the `write` scope; reading is public. The contract says so (`Security: auth.Needs("write")` in Go, `spec: needs(["write"], ...)` in TypeScript), so the specs and the SDKs carry it, and one middleware enforces it: no token it knows is 401, a token without the scope 403. The libraries are `go/auth` and `@charter/ts/auth`.

The tokens are the Worker's secrets, named in `mise.toml` (`WORKER_SECRETS = "READ_TOKEN WRITE_TOKEN"`) and declared in `cloudflare.config.ts` (`bindings.secret()`). `fnox.toml` says where fnox keeps them; make them once:

```sh
openssl rand -hex 32 | fnox set WRITE_TOKEN -k "<name> WRITE_TOKEN" -p keychain
openssl rand -hex 32 | fnox set READ_TOKEN -k "<name> READ_TOKEN" -p keychain
mise run cloudflare:secrets    # copies them, and the Cloudflare credentials, to GitHub for CI
```

Every `mise run deploy` sets them on the Worker, so the first deploy and a changed token are the same step. The tasks that need them run through `charter exec -secrets`, which takes them from fnox unless they are already set, as in CI. Local runs and tests use throwaway tokens. No task prints a value.

## Test

| Task | Where | What it proves |
|---|---|---|
| `mise run check` | Local | Lint, Go tests, fresh specs, then the live and MCP tests natively (`test:native`) and as Wasm under workerd (`test:workerd`) |
| `mise run live-test` | Cloudflare | The same programs, and the TypeScript SDK, on the deployed Worker |
| `mise run soak` | Cloudflare, redeploys | No stream loses, repeats or reorders an item ([Streaming](streaming.md)) |

Add a Go test in `api/` with the helpers in `api/api_test.go` (`server(t)`, `do(t, method, url, body)`). Add a Node program that takes a URL and exits 1 on a failure to `test:native`, `test:workerd` and `live-test` in `mise.toml`.

## CI on GitHub

`charter new` writes the workflows; every step is a mise task, so a failing step runs the same on your machine. Never edit them: `mise run workflows` writes them again.

| Workflow | When | Runs |
|---|---|---|
| `check` | Every push to main and pull request | `setup`, `check`, on Linux and Windows |
| `sdk-check` | The same | `sdk:gen`, `sdk:check`, `sdk:publish:check` |
| `deploy` | `gh workflow run deploy.yml` | `deploy`, then `live-test` |
| `release` | A version tag | [Release](sdks.md#release) |

```sh
mise run cloudflare:secrets    # once: the deploy workflow's two secrets, from fnox (or: gh secret set)
```

Limits: one Worker and database, no staging, no custom domain. The deploy workflow has not yet run from GitHub.

## Keep the repo in shape

`charter.toml` at the repo's root says what the repo is; `charter new` writes one.

```toml
description = "billing-api: a contract-first API on Cloudflare Workers"
topics = ["api", "cloudflare-workers"]
# homepage = "https://..."     default: the GitHub Pages URL
# docs = "docs"                the docs folder: "docs" or "."
# projects = ["api"]           folders that are projects, in a repo that holds several
```

```sh
mise run repo          # REMOTE: brings the repo in line, printing ok or changed per item
mise run repo:check    # REMOTE, read-only: fails on drift, for CI
```

It runs at the repo's root, from any folder in it, and keeps: the docs site's config and `docs/writing.md`; the issue forms and `labels.tsv`; the workflows, for a repo that is a project or lists `projects`; the GitHub labels, removing GitHub's unused defaults; the description, homepage and topics; GitHub Pages for the docs folder. Running it again changes nothing. A project in a subfolder keeps its own tasks.
