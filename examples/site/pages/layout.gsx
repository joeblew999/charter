package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/examples/site/locales"
	"github.com/joeblew999/charter/site"
	"github.com/joeblew999/charter/site/intl"
)

// Layout is the document around every page. site.Head is its whole <head>: the title and description as search
// engines read them, the page in every language, and web/main.js as Vite built it (htmx 4, gsxui, the CSS).
component Layout(title intl.Msg, description intl.Msg, children gsx.Node) {
	<!DOCTYPE html>
	<html { site.HTML(ctx)... }>
		<head>
			<site.Head title={title |> text} description={description |> text}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased">
			<main class="mx-auto flex max-w-xl flex-col gap-6 p-4">{ children }</main>
			<footer class="mx-auto flex max-w-xl flex-wrap items-center gap-3 p-4 text-sm text-muted-foreground">
				<span>{ locales.HomeLanguages }</span>
				// The page in each language the app ships. A full page load: <html lang> changes.
				{ for _, l := range intl.Links(ctx) {
					<a
						href={l.Href}
						hreflang={l.Lang}
						lang={l.Lang}
						class="underline-offset-4 hover:underline"
						{ if l.Current {
							aria-current="page"
						} }
					>
						{ l.Name }
					</a>
				} }
			</footer>
		</body>
	</html>
}
