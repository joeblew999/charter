---
title: Deploy to Cloudflare
nav_order: 6
parent: Guides
---

# Deploy to Cloudflare

This page gets your API running on Cloudflare Workers with its database, and tested there. Read it for the first deploy, and when you set up deploys from GitHub.

You need a Cloudflare account ([which plan](#what-it-costs-to-run)), and the project checked: `mise run check`.

This page was written without deploying. The commands are the project's tasks as they are in `mise.toml`, and the results quoted are those recorded for orpc-api's own Go Worker, which is the same code ([findings](../findings.md)). What was run in a new project on 2026-10-01: the build, the local migration, the change of URL, and the remote migration command up to the point where it fails ([below](#migrations)).

## Log in

Either one:

```sh
api-go/node_modules/.bin/cf auth login    # once per machine: opens the browser
```

```sh
export CLOUDFLARE_API_TOKEN=...           # or an API token, for a machine without a browser
export CLOUDFLARE_ACCOUNT_ID=...
mise run cloudflare:token                 # fails, naming it, if one of the two is not set
```

`cf` is Cloudflare's command-line tool, installed by `mise run setup` into `api-go/node_modules/`. The token needs permission to edit Workers and D1; the exact set was not checked for this page.

## Deploy

```sh
mise run api-go:deploy      # builds the Wasm, deploys it, then applies the migrations
```

What it does, in order:

1. **Builds** `api-go/build/app.wasm` with TinyGo (`mise run api-go:build`) and fails if it is over 3 MB gzipped.
2. **Deploys** with `cf deploy`, from `api-go/cloudflare.config.ts`. That file is the whole description of what gets created:

   | What | Name | What it is for |
   |---|---|---|
   | The Worker | `billing-api` | Runs your Go code as Wasm |
   | A D1 database | `billing-api-db`, bound as `DB` | The notes. It is the log every stream reads from |
   | A Durable Object class | `NotesHub`, bound as `HUB` | The hub: it wakes the streams that are waiting when a note is created. It stores nothing (`api-go/worker/hub.mjs`) |
   | A variable | `APP_NAME` | The name in the greeting of `/api/hello` |

   A second deploy updates the Worker and keeps the database.
3. **Applies the migrations** that the database has not had yet.

## Migrations

The database's tables are the SQL files in `migrations/`, applied in the order of their names. A project starts with `migrations/0001_init.sql`. To change the schema, add a file with the next number: never edit one that has been applied.

```sh
mise run api-go:migrate          # REMOTE: apply what the deployed database has not had yet
mise run api-go:migrate:local    # the same for the local database of a running mise run api-go:dev
```

`api-go:migrate:local` works, and `mise run check` uses it. Each file is applied once: the local database remembers them in a table `_local_migrations`.

`api-go:deploy` runs the remote migration as its last step, so a first deploy leaves the tables in place.

## The Worker's URL

`cf deploy` prints the URL: `https://billing-api.<your-subdomain>.workers.dev`. There is no command to look it up afterwards.

The tasks take it from `API_GO_URL`, whose default is on the `API_GO_URL` line of `mise.toml`. A new project cannot know your subdomain, so setting it is part of the first deploy: put the URL there, write the specs again, and commit `mise.toml` and the two spec files.

```sh
sed -i.bak "s#default='https://billing-api[^']*'#default='https://billing-api.<your-subdomain>.workers.dev'#" mise.toml && rm mise.toml.bak
mise run api-go:spec         # the specs name the server, and the generated SDKs call it by default
mise run api-go:spec:check   # "specs match the Go contract": the committed specs and the URL agree
```

- **What uses `API_GO_URL`:** `api-go:spec` and `api-go:spec:check` (the server in the specs), `api-go:live-test`, `api-go:soak` and `api-go:bench` (the Worker they call).
- **`mise.local.toml` also works, on one machine only.** An `[env]` table there with `API_GO_URL = "https://..."` overrides the default, and the file is not committed. But the specs are committed with the URL in them, and `mise run check` compares them against `API_GO_URL`. On GitHub, and on a colleague's machine, the default applies, and specs written with your override fail the check there (`openapi.json is stale`). So use it to point the tests at another deployment, and do not run `mise run api-go:spec` while it is set.

## Test what you deployed

```sh
mise run api-go:live-test    # REMOTE: writes test notes into the deployed database
```

Run it after every deploy. It needs Docker the first time, to generate the TypeScript SDK it uses.

It runs three programs against `API_GO_URL`: an SSE stream and a WebSocket both receive a new note, and a stream resumes after a reconnect (`test/live-test.mjs`); the same through the generated TypeScript SDK (`test/sdk-live-test.mjs`); and the MCP endpoint with a real MCP client (`test/mcp-test.mjs`). Every line it prints starts with `PASS` or `FAIL`.

Why, when `mise run check` already ran the same programs locally: **some bugs exist only in production.** Go timers hung on Cloudflare and nowhere else. A stream asked to end after 2 seconds was still open after 40, because Cloudflare's clock only moves in whole milliseconds and the local runtime's clock is real. Every local check was green. The fix is in your project (`api-go/worker/tinygo-clock.mjs`), and the next such bug will again only show there.

Run it a few seconds after the deploy: a request right after it can still reach the previous version.

More tests against the deployed Worker, and what each proves: [Test locally and on Cloudflare](testing.md).

## Deploy from GitHub

`mise run dev:workflows` writes a workflow `api-deploy` ([CI and releases on GitHub](ci-releases.md)). It runs only when you start it:

```sh
gh workflow run api-deploy.yml -f api=api-go
```

It runs four tasks: `cloudflare:token`, `setup`, `api-go:deploy`, then `api-go:live-test` against what it just deployed. So a deploy from GitHub is always tested.

It needs two repository secrets, `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`. There is no browser login on GitHub. Set them once:

```sh
mise run cloudflare:secrets    # REMOTE: copies both from fnox into the repo's GitHub secrets, without printing them
```

- **That task reads the two values from [fnox](https://fnox.jdx.dev),** a secret store that `mise install` provides. Put them there first: `fnox set CLOUDFLARE_API_TOKEN` asks for the value without showing it. Which store fnox keeps them in (the system keychain, an encrypted file) is fnox's own configuration (`fnox init`).
- **Without fnox:** `gh secret set CLOUDFLARE_API_TOKEN` and `gh secret set CLOUDFLARE_ACCOUNT_ID` do the same, each asking for the value.
- **The workflow uses the default `API_GO_URL` from `mise.toml`.** It cannot see a `mise.local.toml`.
- **The migration step fails there too** ([above](#migrations)), so the workflow stops before the live test.

## What it costs to run

A read costs 2 to 7 ms of CPU and a write 10 to 13 ms; the slowest requests cost 9 to 20 ms. Workers Free allows 10 ms per request, so it is enough to try the project, and production wants Workers Paid. `mise run api-go:bench` measures your own API on Cloudflare; the figures and what they mean are on the docs home page ([what it costs](../README.md#before-you-choose-go-what-it-costs-to-run)) and in [benchmarks](../benchmarks.md).

Measure your own Worker as a client sees it:

```sh
mise run api-go:bench    # REMOTE, read-only: median and slowest of 20 requests, for four routes
```

That is wall time, network included. CPU time, which is what Cloudflare bills, is in the Worker's logs in the Cloudflare dashboard.

## Limits

- **The Wasm must stay under 3 MB gzipped.** `mise run api-go:build` fails over that. A new project is at 875 KB (measured 2026-10-01: `874803 B gzipped`), and each Go package you import adds to it.
- **It must build with TinyGo.** Not every Go package does, and `go test` cannot tell you. `mise run check` builds the Wasm and runs it locally for that reason.
- **One Worker, one database, no staging.** A second environment is a second project name, or a change to `api-go/cloudflare.config.ts` that this page does not cover.
- **Nothing here sets a custom domain.** The Worker is on its `workers.dev` address.
