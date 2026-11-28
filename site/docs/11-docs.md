# Docs

An app's docs are Markdown files in its `docs/` folder. charter shows them, lists them for search engines, and gives them to language models as Markdown.

```
docs/
  index.md               /docs/
  01-getting-started.md  /docs/getting-started
  02-how-it-works.md     /docs/how-it-works
```

A page's first heading is its title and its first paragraph its description. The number in a file's name is its place in the list, and is not in its address. A link to another page is written to its file (`[the start](index.md)`), so it works on GitHub too.

## With the commands

| When | Command | What it does for your docs |
|---|---|---|
| You want docs | `add screen:docs` | the pages that show them, in your app's own components |
| You save `docs/*.md` | `dev`, running | the page is rebuilt and shown in about 1.3 s; no Go is compiled |
| You save the API's spec, `db/*.sql` or `wrangler.jsonc` | `dev`, running | the pages charter writes of the app follow |
| Before a push | `check` | a code sample taken from a file is what the file says (`--fix` writes it again) |
| Before a push | `test` | every link a page has to the app is followed, and one that leads nowhere fails |

**Checked:** code that names its file; links; and the pages charter writes, which are read from the source and cannot differ from it.

**Not checked by anything:** what a page says in words. When the code a paragraph describes changes, nothing can tell. So keep prose for what does not change with every commit (what the app is for, why it is built as it is), and take facts from files: a sample by its file's name, a table or an operation from the page charter writes.

## Groups, and the order

The front page, `docs/index.md`, says both, where a reader finds them too. A line that is only bold names a group, and the pages the list under it links to are in it, in that order:

```
**Start**

- [Getting started](01-getting-started.md): a new app in three commands.
- [An app's layout](03-layout.md)

**Write**

- [Pages](04-pages.md), [the API](05-api.md)
```

The list at the side of every page is then those groups, and "next" at the foot of a page is the next one listed. There is nothing else to keep: no file of settings, and no second list. A page the front page lists in no group comes after them, and what charter writes of the app is the group Reference. A front page with no such lines groups nothing, and the pages are in the order of their files' numbers.

## What you write, and what charter writes

You write what only you know: what the app is for, and how to use it.

What the app *has* is already written down once, in the files that make it. charter reads those and writes a page of the docs for each, after yours:

| Page | Address | Written from | Says |
|---|---|---|---|
| The API | `/docs/reference-api` | `fern/openapi.json`, the contract `charter check` holds to your Go types | every operation, who may call it and how often, what it takes and answers, every type |
| The database | `/docs/reference-database` | `db/schema.sql`, `db/queries.sql` | every table and column, every statement the app runs |
| What the Worker is bound to | `/docs/reference-bindings` | `wrangler.jsonc`, `.dev.vars.example`, `fnox.toml` | each database, bucket, queue and variable, and the names of the secrets |

A page is there when its source is: an app with no API has no API page. Nobody keeps them in step, because there is nothing to keep: rename a column and the database's page has the new name at the next save. They are pages like any other: listed, searched, given to language models as Markdown, and their "edit" link goes to the file they are written from. Link to one by its address: `[the API](reference-api)`.

[The example's](https://github.com/joeblew999/charter/tree/main/example) are live: [its API](https://charter-example.gedw99.workers.dev/docs/reference-api), [its database](https://charter-example.gedw99.workers.dev/docs/reference-database), [what it is bound to](https://charter-example.gedw99.workers.dev/docs/reference-bindings).

charter's own site has none of the first two, and says so: it has no API and no database. Its pages explain how charter works; for what an app made with it looks like, they point at the example.

## Links are checked

`charter test` follows every link a page of the docs has to the app itself, as it does for every other page, and fails on one that leads nowhere: an address that answers 404, or a `#part` the page it names does not have. A link to another page's file (`03-pages.md`) or to a written page's address is such a link.

## Showing them

```go
// main.go
Docs: &app.Docs{
	Name:    "charter-example",
	Pages:   docs.Pages,
	Repo:    "https://github.com/joeblew999/charter",
	Dir:     "example",
},

// pages/pages.go
docs  screens.Docs `route:"/docs Docs"`
```

That is the example's: its name, the pages charter build wrote from `docs/`, and where its repository is, for the links to a page's own file. `Summary` is a line about the app. The route is the docs screen's (`go tool charter add screen:docs`).

Everything around a page is a gsxui component, so it has your app's theme and gsxui's own behaviour, and none of it is charter's own markup. The page itself is the one thing that is not: it is the HTML charter build makes of your Markdown, styled to match.

The pages are one of [charter's screens](13-screens-and-themes.md): the list at the side, the page, its sections, the theme switch, a link to the repository and one to the page's own file on GitHub. They are built from your app's own gsxui components, so they have its look. This site's docs are shown by the same screen.

## What comes with them

| Address | What it is |
|---|---|
| `/docs/<page>` | The page, in the sitemap, with a title and a description of its own |
| `/docs/<page>.md` | The same page as Markdown |
| `/llms.txt` | The app in a few lines, and every page of the docs as a link to its Markdown |
| `/llms-full.txt` | Every page, as one Markdown document |

While `charter dev` runs, saving a Markdown file reloads the page.

## What the pages have

- The list of pages at the side, in groups: gsxui's `Sidebar`. Its button, or Cmd-B (Ctrl-B), folds it away; on a phone it slides in over the page.
- Where the page is among the others, at the top, which stays as the page is read: gsxui's `Breadcrumb`.
- The page's own sections at the other side (a second `Sidebar`), and on a narrow window under "On this page", which comes down from the top and goes when a section is chosen (a `Sheet`).
- At the foot of each page, the page before it and the page after it, each a gsxui `Item`: the docs read from one end to the other.
- Search by the titles of pages and their sections. Cmd-K (Ctrl-K) opens it.
- Code marked up by what each word is, in the colours of your app's theme, light and dark.
- The theme switch, the repository, and "edit this page".

## Code taken from a real file

A sample that names a file is that file's code, and `check` keeps it so:

> Open the sample's fence with the language and the file: three backticks, then `go file=../example/pages/pages.go#Home.Props`.

`file` is a path from the app's folder. After `#` is one declaration by its name: a type, a function, a method as `Type.Method`, a gsx component. Without `#` it is the whole file. `go tool charter check --fix` writes the code into the Markdown, so it reads on GitHub too; `check` fails when the two differ. Most samples of these docs are the example's own code, taken this way.
