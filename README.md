# charter

Go apps on Cloudflare Workers, one way: pages, an API from a contract, a live feed behind both, languages and search
engines, in one Worker.

![Lighthouse performance](site/docs/badges/lighthouse-performance.svg) ![Lighthouse accessibility](site/docs/badges/lighthouse-accessibility.svg) ![Lighthouse best practices](site/docs/badges/lighthouse-best-practices.svg) ![Lighthouse SEO](site/docs/badges/lighthouse-seo.svg) ![Search-engine audit](site/docs/badges/seo-audit.svg)

**The docs:** https://charter.gedw99.workers.dev/docs/

**The example, running:** https://charter-example.gedw99.workers.dev

Both are charter apps in this repository. [`site/`](site) is the docs: the Markdown in [`site/docs/`](site/docs),
which is where to read them on GitHub. [`example/`](example) is the one app that uses all of charter: its notes are
the live feed, people sign in and have roles, each thing Cloudflare gives a Worker does one job in it, and it is what
`charter create` copies.

## Start

You need [mise](https://mise.jdx.dev), once (macOS: `brew install mise`; Linux: `curl https://mise.run | sh`;
Windows: `winget install jdx.mise`), with `mise activate` in your shell. It brings the Go, Node and fnox each app pins.

```sh
mise exec go@latest -- go run github.com/joeblew999/charter/cmd/charter@latest create my-app
cd my-app
go tool charter dev
```

Before a push: `go tool charter check && go tool charter test`. To ship: `go tool charter deploy`.

## For your coding agent

This repository is a Claude Code plugin and a marketplace, as gsx's, gsxui's and structpages' are. It lists charter's
own skill and theirs, each from its owner's repository:

```
/plugin marketplace add joeblew999/charter
/plugin install charter@charter
```

An app made with charter enables all four in its `.claude/settings.json`, and `go tool charter install` installs them:
[For your coding agent](https://charter.gedw99.workers.dev/docs/agents). A language model reads the docs at
[`/llms.txt`](https://charter.gedw99.workers.dev/llms.txt).

## The repository

| Folder | What is in it |
|---|---|
| `app`, `api`, `live`, `cloud`, `intl`, `seo` | The library: `github.com/joeblew999/charter/<name>` |
| `cmd`, `internal` | The command, `go tool charter` |
| `example` | The one app that uses all of charter, and the template `charter create` copies |
| `site` | charter's own site: a front page and the docs, as a charter app |
| `skills` | The skill the plugin ships |

Rules for working on charter itself: [AGENTS.md](AGENTS.md).
