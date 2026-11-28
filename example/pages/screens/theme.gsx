package screens

import (
	"encoding/json"

	"github.com/gsxhq/gsx"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui/icon"
	ui "github.com/joeblew999/charter/example/ui/styles"
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
// which charter check --fix writes into the app (pages/screens/theme-editor.gen.html, web/gsxui-site). A style is
// in the components' markup, where the rest of a theme is CSS: the app has its components in each of gsxui's
// styles (ui/styles, written by gsxui's own apply), the editor tells the Worker which the reader chose
// (app.StyleCookie), and every page is answered in it.
//
// From charter add screen:theme: the app's own now, to change as it likes.
type Theme struct{}

// FullLoad says the page is loaded as a document, never put in place of another by a link: gsxui's editor script
// starts as its document loads.
func (Theme) FullLoad() bool { return true }

component (p Theme) Page() {
	// gsxui's layout for a workspace: the window under the header, the editor's controls and its preview in it.
	<Layout
		mode={LayoutWorkspace}
		title={locales.ScreensThemePick |> text}
		description={locales.ScreensThemePick |> text}
		brand={ <LayoutBrand/> }
		tools={ <ThemeToggle/> }
	>
		// gsxui's editor is in English, as gsxui wrote it.
		<div lang="en" dir="ltr">
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
						What you choose here is kept by your browser, and every page of this app has it from now on, the style too.
						The preview is this app's own front page. The address of this page carries your choice
						(<code data-charter-code class="font-mono text-xs"></code>): send it, and whoever opens it has the same.
					</p>
					<div class="flex flex-wrap gap-2">
						<ui.Button type="button" variant="outline" size="sm" data-charter-own>The app's own theme</ui.Button>
						<ui.Button variant="outline" size="sm" href="https://ui.gsxhq.dev/theme" rel="noopener" data-charter-gsxui>
							Open in gsxui's editor
						</ui.Button>
					</div>
				</section>
			} }
		</div>
	</Layout>
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
