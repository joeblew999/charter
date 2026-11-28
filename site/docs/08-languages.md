# Languages and search engines

## With the commands

| When | Command | What it does for your languages |
|---|---|---|
| You want a language | `add locale:de` | its catalog, `locales/de.toml` |
| You save a catalog | `dev`, running | the messages are written as Go names and the open page has the new text in about 1.3 s |
| You want it translated | `run intl:status`, `intl:translate`, `intl:todo` | what is missing or stale in each language, and a translation of it |
| Before a push | `check` | every language complete, plural forms right, no message unused |
| Before a push | `test` | the audit: each page's language, its links to its other languages, the sitemap |

**Checked before the app runs:** that a message a page shows exists and is given its arguments (it is a Go name); that every language has every message, with the plural forms that language has.

**Not checked by anything:** whether a translation says the right thing. A message whose source changed is marked stale until someone confirms or retranslates it (`run intl:confirm`).

## Languages

The languages are the catalogs in `locales/`: `en.toml` is the source, and each other file is a translation.

```sh
go tool charter add locale:de
go tool charter run intl:translate
```

Every page has a URL per language (`/built/`, `/de/built/`). Text in a page is a message from the catalog, never a literal:

```gsx
<ui.Button variant="link" class="px-0" href={screens.SignInPage{} |> url}>{ locales.BuiltSignIn }</ui.Button>
```

Numbers, dates and ages go through the runtime's Intl, in the reader's language and time zone:

```gsx
<span class="text-xs text-muted-foreground">{ note.Created() |> since }</span>
{ note.Created() |> date("dateStyle", "full", "timeStyle", "long") }
<dd>{ f.Timezone |> zone }</dd>
```

An age ("3 minutes ago"), a full date and time, and a time zone by its name, as the example's pages write them. `number` takes Intl's own options the same way.

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
