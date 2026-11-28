# Getting started

You need [mise](https://mise.jdx.dev), once. It brings everything else, at the versions each app pins.

```sh
brew install mise      # once. Linux: curl https://mise.run | sh   Windows: winget install jdx.mise
mise exec go@latest -- go run github.com/joeblew999/charter/cmd/charter@latest create my-app
cd my-app
go tool charter dev
```

Put `mise activate` in your shell's profile, once ([mise's guide](https://mise.jdx.dev/getting-started.html)): then in an app's folder `go` and `node` are the app's.

`create` copies [the example](https://github.com/joeblew999/charter/tree/main/example) under your name, gives it the tools charter is built with (its own `mise.toml`), installs what it pins and writes its specs.

## Working on it

`dev` runs the app on the real runtime: workers-go on wrangler dev, with the bindings of `wrangler.jsonc`. The first time, it makes the local database and fills it with the app's [seed](07-database.md). Leave it running and run everything else beside it.

| You save | What happens | On the page after about |
|---|---|---|
| A `.gsx` or `.go` file | gsx regenerates, the Wasm is built, the running Worker takes it | 1.5 s |
| A catalog (`locales/*.toml`) | the messages are compiled and handed to the running Worker: no Wasm is built | 1.3 s |
| A page of the docs (`docs/*.md`) | the same: the pages are data beside the Wasm | 1.3 s |
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

`check` says everything that can be said without running the app: formatting, go vet and structpages' linter (for the machine and for the Worker's build), the API's contract, the database's schema, the languages, and charter's screens. `check --fix` writes what it can. `check --json` is the same as one JSON document, each part with what it said, for a script or a coding agent. `test` runs the Worker on wrangler dev and asks it: the pages in a real browser, the API through its own SDK, and the search-engine audit.

## Shipping it

```sh
go tool charter deploy
go tool charter test --deployed
```

The first deploy makes the database, the KV namespace, the bucket and the queue that `wrangler.jsonc` names. The same tests then ask the deployed Worker.

## Tools

| For | Tool | Pinned by | Brought by |
|---|---|---|---|
| Everything below | mise | | you, once |
| Building | Go | `mise.toml` | mise |
| Vite and wrangler | Node | `mise.toml` | mise |
| Secrets | fnox | `mise.toml` | mise |
| Vite, wrangler, gsx's plugin | npm packages | `package-lock.json` | `charter install` |
| Containers | Kiri | | charter, the first time `charter pack` generates SDKs |
| Skills for your coding agent | Claude Code plugins for charter, gsx, gsxui and structpages | | `charter install` |

`mise.toml` is the app's, and so is `mise.lock`, which says what each download must be: commit both. Every machine and CI then build with the same Go and the same Node.

```toml
[tools]
go = "1.27.4"
node = "26.13.1"
"github:jdx/fnox" = "1.50.0"
```

`go tool charter check` holds the app to it:

- **What runs is what is pinned.** A shell whose PATH mise has not set for this folder runs another Go or Node, and what it builds is not what CI builds: check fails and says what runs and what is pinned. An interactive shell with `mise activate` follows the folder by itself. A shell that does not run its hook (an editor's task, an agent's) gets it with `eval "$(mise hook-env)"`.
- **At least what charter was tested with.** An app pins no less than charter's own `mise.toml`. After `go tool charter upgrade`, `check --fix` raises the app's pins and installs them; it never lowers one. What a release changed in how an app is written is on [the upgrading page](17-upgrading.md).

go.mod's `go` line is something else: the oldest Go the module may be built with. Leave it where it is until you need something newer.

In CI, [mise's action](https://github.com/jdx/mise-action) installs the tools from `mise.lock` (charter's own workflows use it through `.github/actions/tools`). [Renovate](https://docs.renovatebot.com/modules/manager/mise/) reads `mise.toml` and `mise.lock` and keeps them current; Dependabot does not read them.

The first command that talks to Cloudflare says, in three steps, how to put your token and account id into fnox, once. `go tool charter env` says whether they are there.

## Every command

The commands are [Vite+](https://viteplus.dev/guide/)'s verbs. [The commands, and what each one proves](02-commands.md) goes through them in the order you meet them: what each does, what it checks of your code, and what it leaves for the next one. `go tool charter` lists them, and `go tool charter run` lists the tasks that are no verb: `intl:translate`, `seo:audit`, `db:status`, `ui:list` and the rest.
