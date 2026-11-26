# charter

Go apps on Cloudflare Workers, one way: pages, an API from a contract, a live feed behind both, languages and search
engines, in one Worker.

**The docs are a charter app:** https://charter-example.gedw99.workers.dev/docs/

That site is [`example/`](example) in this repository: its notes are the live feed, its bindings page uses everything
Cloudflare gives a Worker, and its docs are the Markdown in [`example/docs/`](example/docs), which is where to read
them on GitHub. It is also what `charter create` copies.

## Start

You need Go and Node. charter brings everything else itself.

```sh
go run github.com/joeblew999/charter/cmd/charter@latest create my-app
cd my-app
go tool charter dev
```

Before a push: `go tool charter check && go tool charter test`. To ship: `go tool charter deploy`.

## For your coding agent

This repository is a Claude Code plugin and a marketplace, as gsx's, gsxui's and structpages' are. It lists charter's
own skill and theirs, each from its owner's repository:

```
/plugin install charter --marketplace joeblew999/charter
```

An app made with charter enables all four in its `.claude/settings.json`, and `go tool charter install` installs them.
A language model reads the site at [`/llms.txt`](https://charter-example.gedw99.workers.dev/llms.txt).

## The repository

| Folder | What is in it |
|---|---|
| `app`, `api`, `live`, `cloud`, `intl`, `seo` | The library: `github.com/joeblew999/charter/<name>` |
| `cmd`, `internal` | The command, `go tool charter` |
| `example` | The one app that uses all of charter: the docs site, and the template |
| `skills` | The skill the plugin ships |

Rules for working on charter itself: [AGENTS.md](AGENTS.md).
