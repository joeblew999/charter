# Getting started

You need Go and Node. charter brings everything else itself.

## A new app

```sh
go run github.com/joeblew999/charter/cmd/charter@latest create my-app
cd my-app
go tool charter dev
```

`create` copies this site's source under your name, installs what it pins and writes its specs. `dev` runs it on the real runtime and opens it; a save rebuilds and reloads. Leave it running and run everything else beside it.

## Before a push

```sh
go tool charter check
go tool charter test
```

`check` says everything that can be said without running the app: formatting, lint, the API's contract, the database's schema, the languages. `check --fix` writes what it can. `test` runs the Worker on wrangler dev and asks it: the pages in a real browser, the API through its own SDK, and the search-engine audit.

## Shipping it

```sh
go tool charter deploy
go tool charter test --deployed
```

The first deploy makes the database, the KV namespace, the bucket and the queue that `wrangler.jsonc` names. The same tests then ask the deployed Worker.

## What charter brings

| For | Tool | When |
|---|---|---|
| Secrets | fnox, a pinned release checked against its SHA-256 | the first time a command talks to Cloudflare |
| Containers | Kiri | the first time `charter pack` generates SDKs |
| Skills for your coding agent | Claude Code plugins for charter, gsx, gsxui and structpages | `charter install` |

Put your Cloudflare token into fnox once: `fnox set CLOUDFLARE_API_TOKEN` and `fnox set CLOUDFLARE_ACCOUNT_ID`.

## Every command

The commands are [Vite+](https://viteplus.dev/guide/)'s verbs. `go tool charter` lists them, and `go tool charter run` lists the tasks that are no verb: `intl:translate`, `seo:audit`, `db:status`, `ui:list` and the rest.
