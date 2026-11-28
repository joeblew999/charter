# Screens and themes

## With the commands

| When | Command | What it does |
|---|---|---|
| You want a screen | `add screen:sign-in`, `screen:docs`, `screen:admin`, … | copies it into `pages/screens/`, with the gsxui components it is made of |
| You want a component | `add ui:dialog`; `run ui:list` | gsxui's component, in `ui/` |
| You want another look | `run ui:theme --style lyra`, `run ui:theme <preset>` | another of gsxui's 8 styles, or a style and theme from its editor: `gsxui apply` |
| Readers choose a theme | `add screen:theme` | gsxui's own theme editor, in the app, with the app as its preview: style, mode, base color, theme, chart color, menu accent, radius, fonts |
| Before a push | `check` | fails when charter's screens have moved on and your unchanged copies have not; `--fix` brings them up to date |

**Checked before the app runs:** a screen is a `.gsx` file of your app, so it is compiled against your own components and your own messages, as your pages are.

**Not checked:** a screen you have changed is yours. charter no longer updates it and says so, and `add screen:<name> -diff` shows how it differs. How anything looks is seen in a browser.

## The interface is gsxui's

Every visual piece is a [gsxui](https://ui.gsxhq.dev) component, copied into the app's `ui/` folder, where it is the app's own.

```sh
go tool charter run ui:list        # what there is
go tool charter add ui:dialog      # one more (gsxui add; -diff and -overwrite pass through)
```

## Screens

What every app needs and nobody should write twice is a screen: gsx that charter copies into `pages/screens`, the way gsxui copies a component. A screen is built from the app's own gsxui components, so it has the app's look, and its text comes translated.

```sh
go tool charter add screen:docs screen:theme-toggle
```

| Screen | What it is | In the page tree |
|---|---|---|
| `docs` | The docs' pages, from `docs/*.md` | `docs screens.Docs` at `/docs` |
| `theme-toggle` | `<screens.ThemeToggle/>`: light and dark | |
| `theme` | `screens.Theme`: gsxui's theme editor, and `<screens.ThemeLink/>`, the way to it | |
| `language-list` | `<screens.LanguageList/>`: the page in every language | |
| `language-offer` | `<screens.LanguageOffer/>`: the page offered in the reader's own language | |
| `preferences` | `<screens.PreferencesDialog/>`: time zone and clock | `preferences screens.Preferences` at `/preferences` |
| `admin` | [The admin screen](15-admin.md): the app at a glance, its bindings, its databases; `<screens.AdminLink/>` | `screens.Admin` at `/admin` |
| `sign-in` | [Signing in](14-sign-in.md) with a passkey, the account page, `<screens.AccountLink/>` | `screens.SignIn` at `/sign-in`, `screens.Account` at `/account` |

A copy you have not changed follows charter: `check` says when charter writes it differently, and `check --fix` brings it up to date. A copy you changed is yours, and is left alone (`go tool charter add screen:docs -diff` shows the difference, `-overwrite` takes charter's).

A screen's text follows the same way, message by message and in every language: one you have as charter wrote it takes charter's new wording, and one you worded yourself keeps yours. `locales/intl.sum` records which is which (its `file text:…` lines), so you never mark anything.

## The layout

```sh
go tool charter add screen:layout
```

One document is around every page of an app: `screens.Layout`. Every page has the same header (the app's name, the ways to its pages, its tools) and the same footer, charter's screens too: the docs, the theme editor, signing in, the account, the admin's pages. The app says them once, as it starts:

```go
	screens.Around.Links, screens.Around.Tools, screens.Around.Footer = Links(), Tools(), Footer()
```

A page then names its mode and nothing else. It writes no `<!DOCTYPE html>`, no header, no width and no way back: the app's name in the header is the way to the front page, on every page.

```gsx
	<screens.Layout mode={screens.LayoutPage} title={title |> text} description={description |> text}>
```

What moves in the layout is gsxui's own components, as `gsxui add` brings and updates them: its `Sidebar`, which is a drawer on a phone by itself, and its `Sheet` for what else has no room there.

| Mode | For | What it is |
|---|---|---|
| `LayoutMarketing` | a front page | as wide as the header |
| `LayoutPage` | a page of the app: a list, a form, signing in, the admin's pages | one column at a width that reads, the same on every one |
| `LayoutDocs` | a page to read | gsxui's `Sidebar` with the pages, the page beside it, its sections at the other side |
| `LayoutWorkspace` | the theme editor | the whole window under the header, no footer |

| What | On a wide window | On a phone |
|---|---|---|
| The app's name | at the start of the header | the same |
| `Around.Links` | in the header | in a drawer, from the header's menu button; on a page of docs, at the top of the list of pages |
| `Around.Tools` | in the header | in the header |
| `Around.Footer` | under the page | under the page |
| A docs page's `navigation` | in the `Sidebar`, which its button or Cmd-B folds away | the `Sidebar` slides in over the page |
| A docs page's `contents` | at the other side, staying in view | under "On this page" at the top of the page |

The links are shown in two places, so nothing in them has an id.

What is checked: `check` fails a page that writes a document of its own. A browser test opens each page at a phone's, a tablet's, a laptop's and a desk's width and fails one that is wider than its window, has something in its header cut off, or shows a menu before it is opened (`page.Wide(390)`, `page.Fits()`).

What is not: how a page looks at a width. And inside its part of the layout a page still places its own elements, with Tailwind's classes.

## Themes

The look is gsxui's, all of it, and so is the way to change it: [gsxui's theme editor](https://ui.gsxhq.dev/theme). charter adds no colour, no setting and no control of its own. What the editor makes is a preset:

| Part of a preset | gsxui has | Changed |
|---|---|---|
| Style: how the components are drawn | 8: Vega, Nova, Maia, Lyra, Mira, Luma, Sera, Rhea | by a reader, and while developing |
| Mode | Light, Dark | by a reader, or their system |
| Base color | 7 | by a reader, and while developing |
| Theme | the base color's own, and 17 accents | by a reader, and while developing |
| Chart color | 18 | by a reader, and while developing |
| Menu accent | Subtle, Bold | by a reader, and while developing |
| Radius | None, Small, Medium, Large | by a reader, and while developing |
| Body font, Heading font | 6 each | by a reader, and while developing |

A preset has a share code (`gsxui:p1:4nI`), which is how it is kept, sent and applied.

### The editor, in your app

```sh
go tool charter add screen:theme
```

`screens.Theme`, mounted at `/theme`, is gsxui's own editor: the same controls, under the same names, as on gsxui's site. Beside them, where gsxui's site shows its gallery, is your app's own front page, changing as a reader chooses. `<screens.ThemeLink/>` in a header is the way there; the docs' pages have it.

What a reader chooses:

- **is kept by their browser**, and every page of the app has it from then on, before its first paint, in the light and in the dark, in the style they chose;
- **is in the page's address** (`/theme?preset=gsxui:p1:ZJC4`): send the address and whoever opens it has the same, in this app or in another made with charter, and in gsxui's own editor;
- **is undone** by "The app's own theme".

Everything in it is gsxui's own code: its markup and its catalogue of choices, rendered by gsxui's own Go, and its three scripts. `charter check --fix` writes them into the app, as gsxui writes its components there (`pages/screens/theme-editor.gen.html`, `web/gsxui-site/`), and again when the app's preset or its gsxui changes; `charter check` fails while they are behind. A page of the app shown in the editor is its preview by gsxui's own preview protocol.

Why it is written into the app and not compiled into it: gsxui's editor is 25 MiB as Go in a Worker, and its preview of every component in every style 40, of the [64 a Worker may have](16-watching.md#cloudflares-limits). Rendered once on your machine it is a file of half a megabyte.

### A style

A style is the one part of a preset that is not CSS. gsxui has it in each component's own markup: a button in Lyra has other classes than a button in Nova, and a card other elements. So the app has its components in every one of gsxui's styles, and the Worker answers each reader in theirs:

| Where | What | Written by |
|---|---|---|
| `ui/` | the components in the app's own style | `gsxui add`, `gsxui apply`: yours to change |
| `ui/styles/<style>/` | the same components in each of the other seven | `charter check --fix`, by running gsxui's own `apply` in a copy of the app |
| `ui/styles/` | a package named `ui` with every component, each rendering the reader's style of it | `charter check --fix` |

A page takes its components from the third, where it would take them from `ui`:

```go
	ui "github.com/joeblew999/charter/site/ui/styles"
```

`<ui.Button>` in a page is then the reader's style of the button. `check` fails a page that imports `ui` itself, which would be the app's own style whatever was chosen.

The editor tells the Worker the style in a cookie (`gsxui-style`), since the Worker needs it to write the markup; the rest of a preset stays in the browser. Choosing a style in the editor asks for the previewed page again, and it comes back in that style.

What this costs, and what it does not do:

- **Size.** Seven more styles of the example's 19 components are 3 MiB of its Worker (measured: 28.1 MiB with one style, 31.2 with eight, of the [64 a Worker may have](16-watching.md#cloudflares-limits)).
- **A component you change in `ui/` is changed in the app's own style only.** The other seven are gsxui's, written again whenever gsxui or the app's components change. A reader who chose another style sees gsxui's.
- **A component that takes a type of its own package** (a chart's `ChartConfig`, a calendar's locale) is the app's own style whatever is chosen: another package's component cannot be handed it.
- **`charter add ui:<name>`, not `gsxui add` alone**, once a page uses the component: charter has `ui/styles` declare it first, then writes it in every style.

### While developing

```sh
go tool charter run ui:theme                            # what the app has, and what there is
go tool charter run ui:theme --style lyra               # another of gsxui's styles, keeping the colours
go tool charter run ui:theme 'gsxui:p1:ZJC4'            # a whole preset: a share code from the editor
go tool charter run ui:theme my-preset.json --only theme
```

Each is `gsxui apply`. It writes `gsxui.preset.json` and `web/gsxui/theme.css`, and for another style every component in `ui/` again in that style; charter then writes `ui/styles` again, which is every style but the app's own. That is the app's own theme: what every reader has who has chosen nothing. The pages follow at once under `charter dev`.

So the docs and the example have one theme by having one preset: apply the same share code to each.

### Light and dark

`app.Head` decides which before the first paint: what the reader chose, else what their system asks for. `<screens.ThemeToggle/>` switches, and any element with `data-theme-toggle` does. It is as gsxui's own site does it. In the editor, Mode shows the preview in the other; a reader's own light or dark stays the toggle's.

### What is checked, and what is not

A browser test drives the editor in the app: it chooses a theme, a radius and a font with gsxui's own controls, sees the preview change, finds the choice on another page in the light and in the dark, hands the address to a second browser, chooses another style and finds the app's pages drawn in it, and goes back to the app's own. The share code charter makes for `--style` is in gsxui's format, which a test holds to gsxui's own source.

Not checked by anything: how a preset looks, and how each page of yours looks in each of the eight styles: the test tells Lyra from Nova by one button. That a colour has enough contrast on another is gsxui's work, and seen in a browser. The editor is in English, as gsxui wrote it. And the two buttons with which gsxui's editor chooses between its own two preview pages are not shown: here the preview is your app's front page.
