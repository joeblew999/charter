# Languages and search engines

## Languages

The languages are the catalogs in `locales/`: `en.toml` is the source, and each other file is a translation.

```sh
go tool charter add locale:de
go tool charter run intl:translate
```

Every page has a URL per language (`/built/`, `/de/built/`). Text in a page is a message from the catalog, never a literal:

```gsx
<h1>{ locales.HomeHeading }</h1>
```

Numbers, dates and ages go through the runtime's Intl, in the reader's language and time zone:

```gsx
{ s.Visits |> number }
{ note.Created() |> since }
{ t |> date("dateStyle", "medium", "timeStyle", "short") }
```

`charter check` fails while a language is incomplete. A right-to-left language is laid out right to left; `check --fix` rewrites gsxui's components for it once.

These docs are the exception: they are written in English.

## Search engines

charter serves `robots.txt`, a sitemap in every language, the canonical address and every alternate in each page's head, and `noindex` on whatever is no page.

```sh
go tool charter run seo:audit
```

The audit reads the live site as a search engine does and fails on anything that would keep a page out of the index or look wrong in a result. It also follows every link a page has to the site itself, from the docs' pages as from any other, and fails on one that leads nowhere: an address that answers 404, or a `#part` the page it names does not have. `charter test` runs it on every push.

## Agents

[`/llms.txt`](/llms.txt) lists the site's pages and these docs for a language model, as [llmstxt.org](https://llmstxt.org) lays it out.
