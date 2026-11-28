# Screens and themes

## With the commands

| When | Command | What it does |
|---|---|---|
| You want a screen | `add screen:sign-in`, `screen:docs`, `screen:admin`, … | copies it into `pages/screens/`, with the gsxui components it is made of |
| You want a component | `add ui:dialog`; `run ui:list` | gsxui's component, in `ui/` |
| You want another look | `run ui:theme --style lyra`, `run ui:theme <preset>` | another of gsxui's 8 styles, or a style and theme from its editor: `gsxui apply` |
| Readers choose a theme | `add screen:theme-picker` | gsxui's 17 accents in `web/gsxui/themes.css`, chosen as the page runs, light and dark |
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
| `theme-picker` | `<screens.ThemePicker/>`: gsxui's themes, chosen as the page runs | |
| `language-list` | `<screens.LanguageList/>`: the page in every language | |
| `language-offer` | `<screens.LanguageOffer/>`: the page offered in the reader's own language | |
| `preferences` | `<screens.PreferencesDialog/>`: time zone and clock | `preferences screens.Preferences` at `/preferences` |
| `admin` | [The admin screen](15-admin.md): the app at a glance, its bindings, its databases; `<screens.AdminLink/>` | `screens.Admin` at `/admin` |
| `sign-in` | [Signing in](14-sign-in.md) with a passkey, the account page, `<screens.AccountLink/>` | `screens.SignIn` at `/sign-in`, `screens.Account` at `/account` |

A copy you have not changed follows charter: `check` says when charter writes it differently, and `check --fix` brings it up to date. A copy you changed is yours, and is left alone (`go tool charter add screen:docs -diff` shows the difference, `-overwrite` takes charter's).

A screen's text follows the same way, message by message and in every language: one you have as charter wrote it takes charter's new wording, and one you worded yourself keeps yours. `locales/intl.sum` records which is which (its `file text:…` lines), so you never mark anything.

## Themes

The look is gsxui's, all of it, in gsxui's own three parts. charter adds no colour, no setting and no stylesheet of its own to them.

| Part | What it is | How many gsxui has | Changed |
|---|---|---|---|
| Style | how the components are drawn: density, borders, shadows, type | 8: `nova`, `maia`, `vega`, `lyra`, `mira`, `luma`, `sera`, `rhea` | while developing: it is compiled into the components |
| Theme | colours, radius and fonts, as CSS variables and nothing else | a base colour (7) and an accent (17), in any combination | while developing, and by a reader as the page runs |
| Light and dark | the two halves every theme has | | by a reader, or by their system |

### While developing

```sh
go tool charter run ui:theme                            # what the app has, and what there is
go tool charter run ui:theme --style lyra               # another of gsxui's styles
go tool charter run ui:theme '<share code>'             # a style and a theme made at https://ui.gsxhq.dev/theme
go tool charter run ui:theme my-preset.json --only theme
```

Each is `gsxui apply`. It writes `gsxui.preset.json` and `web/gsxui/theme.css`, and for another style it writes every component again in that style: the site went to `lyra` and back to `nova` in nine seconds each way, and came back byte for byte. The pages follow at once under `charter dev`, the docs' pages and charter's screens among them, since they are made of the same components.

Why a style is not changed in the browser: gsxui compiles one into the components' own markup, so a page carries one style and not eight. That is gsxui's design, and charter follows it.

### As the page runs

A theme changes CSS variables only, so a reader can choose one.

```sh
go tool charter add screen:theme-picker
```

`<screens.ThemePicker/>` is a button in the header that lists the app's own theme and gsxui's 17 accents, each with a swatch of its own colour. Choosing one sets `<html data-theme="blue">`; the choice is kept by the browser and is there before the first paint, as light and dark are. The two are apart: every theme has its light and its dark, and `<screens.ThemeToggle/>` switches between them whichever theme is chosen.

The themes are in `web/gsxui/themes.css`, and each is gsxui's own: `charter check --fix` asks gsxui for every accent on the app's style and base colour (`gsxui apply`, with the accent's share code, on a copy of the app's gsxui files) and keeps what each changes. It is written again when `gsxui.preset.json` or the app's gsxui changes, and `charter check` fails while it is behind. Nothing in it is charter's choice: change the app's preset and the 17 follow.

To offer fewer, or others, the screen is yours: it lists `app.Themes()`, and any element with `data-theme-choice="rose"` chooses that theme (`data-theme-choice=""` the app's own).

### Light and dark

`app.Head` decides which before the first paint: what the reader chose, else what their system asks for. `<screens.ThemeToggle/>` switches, and any element with `data-theme-toggle` does. It is as gsxui's own site does it.

### What is checked, and what is not

The share codes charter asks gsxui with are in gsxui's format, which a test holds to gsxui's own source: when gsxui changes it, charter's tests fail before a wrong theme is written. A browser test chooses a theme on the docs, in the light and in the dark, across a link and a page loaded anew.

Not checked by anything: how a theme looks. That a colour has enough contrast on another is gsxui's work, and seen in a browser.
