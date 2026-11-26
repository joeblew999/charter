# charter's site

A front page and the docs, as a charter app. Deployed at https://charter.gedw99.workers.dev.

The docs are the Markdown in [`docs/`](docs): edit a page there. The pages that show them are charter's
docs screen (`pages/screens`, from `charter add screen:docs`), the same the example's docs and every app's docs get.

`npm install` at the repo's top, then here:

```sh
go tool charter dev                              # leave it running: a saved Markdown file reloads the page
go tool charter check && go tool charter test    # before a push
go tool charter deploy                           # then: go tool charter test --deployed
```

## Scores

Written by `go tool charter run seo:badges`: Lighthouse on the deployed site, and the search-engine audit.

<!-- seo scores -->

| Page | Performance | Accessibility | Best practices | SEO |
|---|---:|---:|---:|---:|
| [/](https://charter.gedw99.workers.dev/) | 100 | 100 | 100 | 100 |
| [/docs/](https://charter.gedw99.workers.dev/docs/) | 100 | 100 | 100 | 100 |
| [/docs/getting-started](https://charter.gedw99.workers.dev/docs/getting-started) | 98 | 100 | 100 | 100 |
| [/docs/layout](https://charter.gedw99.workers.dev/docs/layout) | 100 | 100 | 100 | 100 |
| [/docs/pages](https://charter.gedw99.workers.dev/docs/pages) | 84 | 100 | 100 | 100 |
| [/docs/api](https://charter.gedw99.workers.dev/docs/api) | 100 | 100 | 100 | 100 |
| [/docs/live](https://charter.gedw99.workers.dev/docs/live) | 100 | 100 | 100 | 100 |
| [/docs/database](https://charter.gedw99.workers.dev/docs/database) | 100 | 100 | 100 | 100 |
| [/docs/languages](https://charter.gedw99.workers.dev/docs/languages) | 100 | 100 | 100 | 100 |
| [/docs/tests](https://charter.gedw99.workers.dev/docs/tests) | 100 | 100 | 100 | 100 |
| [/docs/deploy](https://charter.gedw99.workers.dev/docs/deploy) | 100 | 100 | 100 | 100 |
| [/docs/docs](https://charter.gedw99.workers.dev/docs/docs) | 85 | 100 | 100 | 100 |
| [/docs/agents](https://charter.gedw99.workers.dev/docs/agents) | 100 | 100 | 100 | 100 |
| [/docs/screens-and-themes](https://charter.gedw99.workers.dev/docs/screens-and-themes) | 100 | 100 | 100 | 100 |

Lighthouse mobile on Google's servers (PageSpeed Insights), 2026-11-26: `charter run seo:badges` writes this table and the badges.

<!-- /seo scores -->
