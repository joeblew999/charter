# The example

This app is charter's example: the one app that uses all of charter, and what `go tool charter create` copies. These pages are its own docs, Markdown files in its `docs/` folder, shown by the same pages charter's site uses for its docs.

charter itself is documented at [charter's site](https://charter.gedw99.workers.dev/docs/).

## What is in it

| Where | What |
|---|---|
| [The home page](../) | Notes, live: write one and it shows at once on every open page |
| [How it's built](../built/) | Each thing Cloudflare gives a Worker doing one job, on a page of its own |
| [The API](reference-api) | The notes API, from a contract, with tokens, limits, a stream and SDKs |
| These pages | Docs from Markdown, and from the app itself |

## What it has, exactly

Three pages here are not written by anyone. charter writes them from where the app is declared, so they are what the app has now:

| Page | Written from |
|---|---|
| [The API](reference-api): every operation, who may call it and how often, what it takes and answers | the API's contract, `fern/openapi.json` |
| [The database](reference-database): every table and column, and every statement the app runs | `db/schema.sql` and `db/queries.sql` |
| [What the Worker is bound to](reference-bindings): its database, bucket, queue, variables and the names of its secrets | `wrangler.jsonc`, `.dev.vars.example`, `fnox.toml` |

Change a column, an operation or a binding and its page changes with it. There is nothing to keep in step.

## Making it yours

Replace the Markdown files with your own docs: what your app is for, and how to use it. A page is a Markdown file; its first heading is its title, and its file's name is its address. The three pages above stay, and follow your app. See [how it is built](01-how-it-is-built.md).
