# charter

Go apps on Cloudflare Workers, one way: pages, an API from a contract, a live feed behind both, languages and search engines, in one Worker.

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

## See it running

[The example](https://charter-example.gedw99.workers.dev) is one app that uses all of charter: its notes are the live feed, people sign in with a passkey and have roles, and each thing Cloudflare gives a Worker does one job in it, on a page of its own. Its source is [`example/`](https://github.com/joeblew999/charter/tree/main/example) in charter's repository, and it is what `charter create` copies.

This site is a charter app too: [`site/`](https://github.com/joeblew999/charter/tree/main/site), a front page and these docs, which are Markdown files in its `docs/` folder. The pages that show them are charter's own, the same ones [your app's docs](11-docs.md) get.

## Where to go

The docs follow what a developer does, in order. Each page opens with the commands that matter for it, what is checked before the app runs, and what is not.

**Start**

- [Getting started](01-getting-started.md): a new app in three commands.
- [The commands, and what each one proves](02-commands.md): the whole day, command by command. Read this one first.
- [An app's layout](03-layout.md): what goes where, and what charter finds by itself.

**Write, beside `charter dev`**

- [Pages](04-pages.md), [the API](05-api.md), [the live feed](06-live.md), [the database](07-database.md).
- [Signing in, and who may do what](14-sign-in.md): one caller and one rule for pages and API; passkeys.
- [Languages and search engines](08-languages.md).
- [Screens and themes](13-screens-and-themes.md): what charter gives every app's interface, in the app's own look.
- [Docs](11-docs.md): your app's own, from Markdown and from the app itself.

**Prove, with `charter check` and `charter test`**

- [Tests](09-tests.md): what to test, what not to, and how.

**Ship and watch, with `charter deploy` and `charter run`**

- [Deploying](10-deploy.md), [the admin screen](15-admin.md), [watching a deployed Worker](16-watching.md).

**With an agent**

- [For your coding agent](12-agents.md): the Claude Code plugins, and what a language model reads.

## What an app made with charter looks like

These pages explain charter. For what an app has, look at the example, whose reference pages nobody wrote: charter writes them from the app itself.

- [Its API](https://charter-example.gedw99.workers.dev/docs/reference-api), from its contract.
- [Its database](https://charter-example.gedw99.workers.dev/docs/reference-database), from its schema and queries.
- [What its Worker is bound to](https://charter-example.gedw99.workers.dev/docs/reference-bindings), from `wrangler.jsonc`.
