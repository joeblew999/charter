# charter

Go apps on Cloudflare Workers, one way: pages, an API from a contract, a live feed behind both, languages and search engines, in one Worker.

This site is a charter app. Its [notes](../) are the live feed, its [bindings page](../bindings) uses everything Cloudflare gives a Worker, and these docs are Markdown files in its `docs/` folder. Its source is [`example/`](https://github.com/joeblew999/charter/tree/main/example) in charter's repository, and it is what `charter create` copies.

## What charter is

A Go library and one command. It drives the tools that own each job and adds what they lack:

| Job | Tool |
|---|---|
| The Worker: run it, ship it, its bindings | [wrangler](https://developers.cloudflare.com/workers/wrangler/) |
| Go on Workers | [workers-go](https://github.com/syumai/workers-go) |
| Components and the dev loop | [gsx](https://gsxhq.github.io) and [gsxui](https://ui.gsxhq.dev) |
| Pages and their routes | [structpages](https://jackielii.github.io/structpages/) |
| The browser's files | [Vite](https://vite.dev) |
| SDKs | [Fern](https://buildwithfern.com) |

A charter app runs on Cloudflare and nowhere else. Locally that is `wrangler dev`, the same runtime; remotely it is Cloudflare.

## Where to go

- [Getting started](01-getting-started.md): a new app in three commands.
- [An app's layout](02-layout.md): what goes where, and what charter finds by itself.
- [Pages](03-pages.md), [the API](04-api.md), [the live feed](05-live.md), [the database](06-database.md).
- [Languages and search engines](07-languages.md).
- [Tests](08-tests.md) and [deploying](09-deploy.md).
