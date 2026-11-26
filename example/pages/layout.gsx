package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/pages/screens"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/intl"
)

// Layout is the document around every page. app.Head is its whole <head>: the title and description as search
// engines read them, the page in every language, the theme, and web/main.js as Vite built it (htmx 4, gsxui, the
// CSS). What is around the page is charter's screens: the time zone and clock dialog and the theme switch in the
// header, the offer of the page in the reader's language above it, the languages below.
component Layout(title intl.Msg, description intl.Msg, children gsx.Node) {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={title |> text} description={description |> text}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased">
			<header class="mx-auto flex max-w-xl items-center gap-1 px-4 pt-4">
				<ui.Button variant="link" class="px-0 font-semibold" href={Home{} |> url} translate="no">charter</ui.Button>
				<span class="flex-1"></span>
				<ui.Button variant="ghost" size="sm" href={screens.DocsIndex{} |> url}>{ locales.DocsNav }</ui.Button>
				<screens.PreferencesDialog/>
				<screens.ThemeToggle/>
			</header>
			<main class="mx-auto flex max-w-xl flex-col gap-6 p-4">
				<screens.LanguageOffer/>
				{ children }
			</main>
			<footer class="mx-auto max-w-xl p-4">
				// The page in each language the app ships. A full page load: <html lang> changes.
				<screens.LanguageList/>
			</footer>
		</body>
	</html>
}
