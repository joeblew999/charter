package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/app/docs"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/intl"
)

// Layout is the document around every page. app.Head is its whole <head>: the title and description as search
// engines read them, the page in every language, and web/main.js as Vite built it (htmx 4, gsxui, the CSS).
component Layout(title intl.Msg, description intl.Msg, children gsx.Node) {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={title |> text} description={description |> text}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased">
			<header class="mx-auto flex max-w-xl items-center gap-3 px-4 pt-4">
				<a href={Home{} |> url} class="font-semibold" translate="no">charter</a>
				<span class="flex-1"></span>
				<a href={docs.Index{} |> url} class="text-sm underline underline-offset-4">{ locales.DocsNav }</a>
			</header>
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
