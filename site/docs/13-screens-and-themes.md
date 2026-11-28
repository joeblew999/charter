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

## Themes

The look is gsxui's, all of it, and so is the way to change it: [gsxui's theme editor](https://ui.gsxhq.dev/theme). charter adds no colour, no setting and no control of its own. What the editor makes is a preset:

| Part of a preset | gsxui has | Changed |
|---|---|---|
| Style: how the components are drawn | 8: Vega, Nova, Maia, Lyra, Mira, Luma, Sera, Rhea | while developing: it is compiled into the components |
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

- **is kept by their browser**, and every page of the app has it from then on, before its first paint, in the light and in the dark;
- **is in the page's address** (`/theme?preset=gsxui:p1:ZJC4`): send the address and whoever opens it has the same, in this app or in another made with charter, and in gsxui's own editor;
- **is undone** by "The app's own theme".

Everything in it is gsxui's own code: its markup and its catalogue of choices, rendered by gsxui's own Go, and its three scripts. `charter check --fix` writes them into the app, as gsxui writes its components there (`pages/screens/theme-editor.gen.html`, `web/gsxui-site/`), and again when the app's preset or its gsxui changes; `charter check` fails while they are behind. A page of the app shown in the editor is its preview by gsxui's own preview protocol.

Why it is written into the app and not compiled into it: gsxui's editor is 25 MiB as Go in a Worker, and its preview of every component in every style 40, of the [64 a Worker may have](16-watching.md#cloudflares-limits). Rendered once on your machine it is a file of half a megabyte.

### A style

A style is the one part a reader cannot have in your app: gsxui compiles one into the components' own markup, so an app has one and not eight. Choose another in the editor and your app's pages stay as they are; the page says so, sends the reader to gsxui's own editor with their choice, where every component is shown in it, and gives the command that builds your app in it.

### While developing

```sh
go tool charter run ui:theme                            # what the app has, and what there is
go tool charter run ui:theme --style lyra               # another of gsxui's styles, keeping the colours
go tool charter run ui:theme 'gsxui:p1:ZJC4'            # a whole preset: a share code from the editor
go tool charter run ui:theme my-preset.json --only theme
```

Each is `gsxui apply`. It writes `gsxui.preset.json` and `web/gsxui/theme.css`, and for another style every component again in that style: the site went to Lyra and back to Nova in nine seconds each way, and came back byte for byte. That is the app's own theme: what every reader has who has chosen nothing. The pages follow at once under `charter dev`.

So the docs and the example have one theme by having one preset: apply the same share code to each.

### Light and dark

`app.Head` decides which before the first paint: what the reader chose, else what their system asks for. `<screens.ThemeToggle/>` switches, and any element with `data-theme-toggle` does. It is as gsxui's own site does it. In the editor, Mode shows the preview in the other; a reader's own light or dark stays the toggle's.

### What is checked, and what is not

A browser test drives the editor in the app: it chooses a theme, a radius and a font with gsxui's own controls, sees the preview change, finds the choice on another page in the light and in the dark, hands the address to a second browser, chooses a style the app is not built in, and goes back to the app's own. The share code charter makes for `--style` is in gsxui's format, which a test holds to gsxui's own source.

Not checked by anything: how a preset looks. That a colour has enough contrast on another is gsxui's work, and seen in a browser. And the editor is in English, as gsxui wrote it.
