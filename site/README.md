# site: server-rendered sites

Package `site` (this folder) wires the rest together in one call: see [the README](../README.md#a-site-in-one-call)
and [`examples/site`](../examples/site).

| Package | What it is |
|---|---|
| `site` | `Site.Handler`, and the `Head` component: Vite, languages and search engines for a gsx site |
| `intl`, `intl/config`, `intl/intltest`, `intl/workerd` | Languages through the runtime's own Intl: messages, plurals, numbers, dates, a URL per language. `charter intl` is its command |
| `gsxintl`, `gsxintl/filters` | Its gsx side: `{ t \|> date }`, `{ n \|> number }`, `{ m \|> text }` |
| `seo`, `seo/audit`, `seo/config` | `robots.txt`, the sitemap, `noindex`, one address. `charter seo` is its command |
| `gsxseo` | The `<head>` search engines and link previews read |
| `web` | `Render`, `Origin` and small net/http helpers for pages |
| `fragments` | A live feed to the browser as HTML fragments over SSE |
