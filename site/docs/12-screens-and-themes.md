# Screens and themes

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
| `language-list` | `<screens.LanguageList/>`: the page in every language | |
| `language-offer` | `<screens.LanguageOffer/>`: the page offered in the reader's own language | |
| `preferences` | `<screens.PreferencesDialog/>`: time zone and clock | `preferences screens.Preferences` at `/preferences` |

A copy you have not changed follows charter: `check` says when charter writes it differently, and `check --fix` brings it up to date. A copy you changed is yours, and is left alone (`go tool charter add screen:docs -diff` shows the difference, `-overwrite` takes charter's).

## Themes

The look is gsxui's preset: a style (how components are shaped), colours for light and for dark, a radius and fonts. It is `gsxui.preset.json`, and `web/gsxui/theme.css` written from it.

```sh
go tool charter run ui:theme                            # what the app has
go tool charter run ui:theme '<share code>'             # another, made at https://ui.gsxhq.dev/theme
go tool charter run ui:theme my-preset.json --only theme
```

That is `gsxui apply`. For another style it copies every component again in that style. The pages follow at once under `charter dev`, the docs' pages among them.

Light and dark are the theme's own two halves. `app.Head` decides which before the first paint: what the reader chose, else what their system asks for. `<screens.ThemeToggle/>` switches, and any element with `data-theme-toggle` does.
