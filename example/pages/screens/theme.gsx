package screens

import (
	"encoding/json"

	"github.com/gsxhq/gsx"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/example/ui/icon"
	"github.com/joeblew999/charter/intl"
)

// Theme is gsxui's theme editor (https://ui.gsxhq.dev/theme), in the app: a reader chooses a style, light or dark,
// a base colour, a theme, a chart colour, a menu accent, a radius and a body and a heading font, with gsxui's own
// controls, and the app's own front page, beside them, changes as they choose. What they choose is gsxui's preset.
// The browser keeps it, every page of the app has it from then on (app.Head), and its share code, in the page's
// address, gives it to someone else or to another app. Mount it in the tree:
//
//	theme screens.Theme `route:"/theme Theme"`
//
// Everything in it is gsxui's: the editor's markup and its catalogue, rendered by gsxui's own Go, and its scripts,
// which charter check --fix writes into the app (pages/screens/theme-editor.gen.html, web/gsxui-site). One thing
// gsxui's design leaves to building: a style is compiled into an app's components. The app's own is what its pages
// show; another is seen in gsxui's own editor, where this page sends a reader with their choice, and built with
// go tool charter run ui:theme.
//
// From charter add screen:theme: the app's own now, to change as it likes.
type Theme struct{}

// FullLoad says the page is loaded as a document, never put in place of another by a link: gsxui's editor script
// starts as its document loads.
func (Theme) FullLoad() bool { return true }

component (p Theme) Page() {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={locales.ScreensThemePick |> text} description={locales.ScreensThemePick |> text}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased" { app.Body()... }>
			<header class="mx-auto flex max-w-[96rem] items-center gap-2 px-4 pt-4">
				<ui.Button
					variant="ghost"
					size="icon-sm"
					href={intl.URL(ctx, "/")}
					hx-boost="false"
					aria-label={locales.ScreensClose}
					title={locales.ScreensClose}
				>
					<icon.ArrowLeft class="rtl:rotate-180"/>
				</ui.Button>
				<span class="flex-1"></span>
				<ThemeToggle/>
			</header>
			// gsxui's editor is in English, as gsxui wrote it.
			<main class="mx-auto max-w-[96rem] px-4" lang="en" dir="ltr">
				{ if app.ThemeEditor("") == "" {
					<ui.Alert class="my-10">
						<ui.AlertDescription>
							gsxui's theme editor has not been written into this app yet: go tool charter check --fix
						</ui.AlertDescription>
					</ui.Alert>
				} else {
					// The app's own preset: where the editor starts for a reader who has chosen nothing.
					<script type="application/json" data-charter-preset>@{ json.RawMessage(app.ThemePreset()) }</script>
					{ gsx.Raw(app.ThemeEditor(intl.URL(ctx, "/"))) }
					<section class="flex flex-col gap-3 border-t py-8 text-sm">
						<h2 class="text-sm font-medium tracking-wide text-muted-foreground uppercase">In this app</h2>
						<p class="max-w-2xl text-muted-foreground">
							What you choose here is kept by your browser, and every page of this app has it from now on. The preview
							is this app's own front page. The address of this page carries your choice
							(<code data-charter-code class="font-mono text-xs"></code>): send it, and whoever opens it has the same.
						</p>
						<ui.Alert data-charter-other-style hidden>
							<ui.AlertDescription>
								This app is built in one of gsxui's styles, and <b data-charter-style></b> is another: its pages keep
								their own, since a style is compiled into the components. See <b data-charter-style></b> on every
								component
								in <a data-charter-gsxui class="underline underline-offset-4" href="https://ui.gsxhq.dev/theme" rel="noopener">gsxui's own editor</a>.
								To build this app in
								it: <code class="font-mono text-xs">go tool charter run ui:theme '<span data-charter-code></span>'</code>
							</ui.AlertDescription>
						</ui.Alert>
						<div class="flex flex-wrap gap-2">
							<ui.Button type="button" variant="outline" size="sm" data-charter-own>The app's own theme</ui.Button>
							<ui.Button
								variant="outline"
								size="sm"
								href="https://ui.gsxhq.dev/theme"
								rel="noopener"
								data-charter-gsxui
							>
								Open in gsxui's editor
							</ui.Button>
						</div>
					</section>
				} }
			</main>
		</body>
	</html>
}

// ThemeLink is the way to the theme editor, for a header: beside the theme toggle. Extra attributes fall through
// to the link.
component ThemeLink(attrs gsx.Attrs) {
	<ui.Button
		variant="ghost"
		size="icon-sm"
		href={Theme{} |> url}
		hx-boost="false"
		aria-label={locales.ScreensThemePick}
		title={locales.ScreensThemePick}
		{ attrs... }
	>
		<icon.Palette/>
	</ui.Button>
}
