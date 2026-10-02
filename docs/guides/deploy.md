---
title: Deploy
nav_order: 7
parent: Guides
---

# Deploy to Cloudflare

How to put a project on Cloudflare Workers with its database, and test it there: from your machine, and from GitHub. You need a Cloudflare account and a project that passes `mise run check`.

What is recorded as run is the notes example's own deploy ([Findings](../findings.md)), not a deploy of a freshly made project.

## Log in

```sh
./node_modules/.bin/cf auth login    # once per machine: opens the browser
```

Or an API token, for a machine without a browser:

```sh
export CLOUDFLARE_API_TOKEN=... CLOUDFLARE_ACCOUNT_ID=...
mise run cloudflare:token            # fails, naming it, if one is not set
```

`cf` is Cloudflare's CLI, installed by `mise run setup`. The token must be able to edit Workers and D1; the exact permissions were not checked.

## Deploy

```sh
mise run deploy       # builds the Wasm, deploys, applies pending migrations
mise run live-test    # REMOTE, writes test notes: run it after every deploy
```

`cloudflare.config.ts` is the whole description of what is created:

| What | Name | What it is for |
|---|---|---|
| The Worker | the project's name | Runs your Go code as Wasm |
| A D1 database | `<name>-db`, bound as `DB` | Your data. It is the log every stream reads |
| A Durable Object class | `Hub`, bound as `HUB` | The hub: wakes open streams. It stores nothing |
| A variable | `APP_NAME` | The name in the greeting of `/api/hello` |

- **The build fails over 3,000,000 bytes gzipped,** the Workers Free limit. The notes example is about 875 KB (2026-10-01); every package you import is compiled in.
- **A second deploy** updates the Worker and keeps the database.
- **Wait a few seconds before the live test:** a request right after a deploy can still reach the previous version.

## Migrations

The tables are the SQL files in `migrations/`, applied in the order of their names, each once. To change the schema, add a file with the next number. Never edit one that was applied.

```sh
mise run migrate          # REMOTE: what the deployed database has not had yet (mise run deploy does this too)
mise run migrate:local    # the same for the local database of a running mise run dev
```

## The Worker's URL

`cf deploy` prints it: `https://<name>.<your-subdomain>.workers.dev`. The tasks take it from `API_URL`, whose default is in `mise.toml`: the specs name it as their server, the remote tests call it, and its first label is the Worker's name ([who reads it](../reference/config.md#environment-variables)).

- **To change it for everyone:** change the default in `mise.toml`, run `mise run spec`, and commit both.
- **`mise.local.toml`** overrides it on one machine, and is not committed. Use it to point the tests at another deployment. Do not run `mise run spec` while it is set: the committed specs would then fail `mise run check` everywhere else.

## Deploy from GitHub

The `deploy` workflow runs only when you start it:

```sh
gh workflow run deploy.yml
```

It runs `cloudflare:token`, `setup`, `deploy`, then `live-test`, so a deploy from GitHub is always tested. It needs two repository secrets:

```sh
mise run cloudflare:secrets    # REMOTE, once: copies both from fnox into the repo's GitHub secrets, without printing them
```

- **[fnox](https://fnox.jdx.dev)** is a secret store that `mise install` provides: `fnox set CLOUDFLARE_API_TOKEN` asks for the value.
- **Without fnox:** `gh secret set CLOUDFLARE_API_TOKEN` and `gh secret set CLOUDFLARE_ACCOUNT_ID`.
- **The workflow reads the default `API_URL`** from `mise.toml`. It cannot see a `mise.local.toml`.

Whether the workflow has run on GitHub was not checked: the recorded deploys were made from a machine.

## Limits

- **One Worker, one database, no staging.** A second environment is a second project name, or a change to `cloudflare.config.ts` that no page covers.
- **No custom domain** is set: the Worker is on its `workers.dev` address.
- **What a request costs:** [Benchmarks](../benchmarks.md).
