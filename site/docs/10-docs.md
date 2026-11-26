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

The pages are one of [charter's screens](12-screens-and-themes.md): the list at the side, the page, its sections, the theme switch, a link to the repository and one to the page's own file on GitHub. They are built from your app's own gsxui components, so they have its look. This site's docs are shown by the same screen.

## What comes with them

| Address | What it is |
|---|---|
| `/docs/<page>` | The page, in the sitemap, with a title and a description of its own |
| `/docs/<page>.md` | The same page as Markdown |
| `/llms.txt` | The app in a few lines, and every page of the docs as a link to its Markdown |
| `/llms-full.txt` | Every page, as one Markdown document |

While `charter dev` runs, saving a Markdown file reloads the page.
