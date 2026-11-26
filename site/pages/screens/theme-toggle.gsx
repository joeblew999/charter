package screens

import (
	"github.com/gsxhq/gsx"
	"github.com/joeblew999/charter/site/locales"
	"github.com/joeblew999/charter/site/ui"
	"github.com/joeblew999/charter/site/ui/icon"
)

// ThemeToggle switches the page between the light and the dark theme of the app's gsxui preset (web/gsxui/theme.css),
// as gsxui's own site does: one click flips it, and the choice is kept. Until a reader chooses, the page follows
// their system. app.Head has the script that does both, before the first paint; this button only carries the
// attribute it listens for. Extra attributes fall through to the button.
//
// From charter add screen:theme-toggle: the app's own now, to change as it likes.
component ThemeToggle(attrs gsx.Attrs) {
	<ui.Button
		variant="ghost"
		size="icon-sm"
		type="button"
		data-theme-toggle
		aria-label={locales.ScreensTheme}
		title={locales.ScreensTheme}
		{ attrs... }
	>
		<icon.Moon class="dark:hidden"/>
		<icon.Sun class="hidden dark:block"/>
	</ui.Button>
}
