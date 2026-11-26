# Getting started

You need Go and Node. charter brings everything else itself.

## A new app

```sh
go run github.com/joeblew999/charter/cmd/charter@latest create my-app
cd my-app
go tool charter dev
```

`create` copies [the example](https://github.com/joeblew999/charter/tree/main/example) under your name, installs what it pins and writes its specs.

## Working on it

`dev` runs the app on the real runtime: workers-go on wrangler dev, with the bindings of `wrangler.jsonc`. Leave it running and run everything else beside it.

| You save | What happens | On the page after about |
|---|---|---|
| A `.gsx` or `.go` file | gsx regenerates, the Wasm is built, the running Worker takes it | 1.5 s |
| A catalog (`locales/*.toml`) | the messages are compiled, then the same | 1.6 s |
| A page of the docs (`docs/*.md`) | it is written as Go, then the same | 2 s |
| `db/schema.sql` | the local database follows, its migration and the queries' Go are written | 2 s |
| `wrangler.jsonc` | wrangler reloads the Worker | 0.6 s |
| A file of `web/`, CSS | Vite replaces it in the open page | at once |

The Worker is started once and never restarted: each build is handed to it. A mistake, in any of these files, shows in the browser in gsx's overlay, and the last page that worked stays up. Cmd-D (Ctrl-D) opens gsx's dev panel, with the Worker's log.

`go tool charter env` says where the loop is running and where its log is, for a second terminal or a coding agent.

`go tool charter install` puts gsx's language server where your editor's gsx extension finds it. The app's `.vscode/` recommends the extension and formats a gsx file when it is saved.

## Before a push

```sh
go tool charter check
go tool charter test
```

`check` says everything that can be said without running the app: formatting, go vet and structpages' linter (for the machine and for the Worker's build), the API's contract, the database's schema, the languages, and charter's screens. `check --fix` writes what it can. `test` runs the Worker on wrangler dev and asks it: the pages in a real browser, the API through its own SDK, and the search-engine audit.

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
