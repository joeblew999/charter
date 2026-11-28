package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/pages/screens"
	ui "github.com/joeblew999/charter/example/ui/styles"
	"github.com/joeblew999/charter/intl"
)

// Layout is the document around every page of the app: charter's layout screen, which is gsxui's own site's shell,
// as a centred page. This says what is in it: the ways to the pages, the tools in the header, and the languages
// under the page. app.Head is its whole <head> (the title and description as search engines read them, the page in
// every language, the theme, and web/main.js as Vite built it).
component Layout(title intl.Msg, description intl.Msg, children gsx.Node) {
	<screens.Layout
		mode={screens.LayoutMarketing}
		title={title |> text}
		description={description |> text}
		brand={ <screens.LayoutBrand/> }
		links={ <Links/> }
		tools={ <Tools/> }
		footer={ <screens.LanguageList/> }
	>
		<div class="flex flex-col gap-6">
			<screens.LanguageOffer/>
			{ children }
		</div>
	</screens.Layout>
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
