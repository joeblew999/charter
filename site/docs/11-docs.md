# Docs

An app's docs are Markdown files in its `docs/` folder. charter shows them, lists them for search engines, and gives them to language models as Markdown.

```
docs/
  index.md               /docs/
  01-getting-started.md  /docs/getting-started
  02-how-it-works.md     /docs/how-it-works
```

A page's first heading is its title and its first paragraph its description. The number in a file's name is its place in the list, and is not in its address. A link to another page is written to its file (`[the start](index.md)`), so it works on GitHub too.

## Showing them

```go
// main.go
Docs: &app.Docs{
	Name:    "My App",
	Summary: "What it is, in a sentence.",
	Pages:   docs.Pages, // built from docs/ by charter build
	Repo:    "https://github.com/me/my-app",
},

// pages/pages.go
type Pages struct {
	…
	docs screens.Docs `route:"/docs Docs"` // go tool charter add screen:docs
}
```

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

- The list of pages at the side, and in a menu where there is no room for a side.
- Search by the titles of pages and their sections. Cmd-K (Ctrl-K) opens it.
- Code marked up by what each word is, in the colours of your app's theme, light and dark.
- The theme switch, the repository, and "edit this page".

## Code taken from a real file

A sample that names a file is that file's code, and `check` keeps it so:

> Open the sample's fence with the language and the file: three backticks, then `go file=../example/pages/pages.go#Home.Props`.

`file` is a path from the app's folder. After `#` is one declaration by its name: a type, a function, a method as `Type.Method`, a gsx component. Without `#` it is the whole file. `go tool charter check --fix` writes the code into the Markdown, so it reads on GitHub too; `check` fails when the two differ. Most samples of these docs are the example's own code, taken this way.
