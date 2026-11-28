package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/pages/screens"
	ui "github.com/joeblew999/charter/example/ui/styles"
	"github.com/joeblew999/charter/intl"
)

// Layout is a page of the app, in the one document every page has: charter's layout screen, made of gsxui's
// components. It has the header and the footer said below, once, for every page: these, the docs, the theme
// editor, signing in and the admin's pages. app.Head is its whole <head> (the title and description as search
// engines read them, the page in every language, the theme, and web/main.js as Vite built it).
component Layout(title intl.Msg, description intl.Msg, children gsx.Node) {
	<screens.Layout mode={screens.LayoutPage} title={title |> text} description={description |> text}>
		<screens.LanguageOffer/>
		{ children }
	</screens.Layout>
}

// What is around every page of the app, said once.
func init() {
	screens.Around.Links, screens.Around.Tools, screens.Around.Footer = Links(), Tools(), Footer()
}

// Links are the pages, the one being shown marked (current, by the tree). The layout shows them in the header, and
// under its menu where the window has no room, so nothing in them has an id.
component Links() {
	<ui.Button variant={current(ctx, Home{})} size="sm" href={Home{} |> url}>{ locales.HomeHeading }</ui.Button>
	<ui.Button variant={current(ctx, Built{})} size="sm" href={BuiltIndex{} |> url}>
		{ locales.BuiltNav }
	</ui.Button>
	<ui.Button variant="ghost" size="sm" href={screens.DocsIndex{} |> url}>{ locales.DocsNav }</ui.Button>
}

// Tools are what the header always has, each a screen of charter's: the admin's pages for whoever may open them,
// who is signed in, the time zone and clock dialog, the theme editor, and light or dark.
component Tools() {
	<screens.AdminLink/>
	<screens.AccountLink/>
	<screens.PreferencesDialog/>
	<screens.ThemeLink/>
	<screens.ThemeToggle/>
}

// Footer is under every page: the page in each language the app ships. A full page load: <html lang> changes.
component Footer() {
	<screens.LanguageList/>
}
