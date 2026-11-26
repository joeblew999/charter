// Package screens is charter's own screens, which charter add screen:<name> copies into an app the way gsxui add
// copies components: the language list, the offer of the page in the reader's language, the time zone and clock
// dialog, the switch between the light and the dark theme, and the docs' pages. Each is a gsx template made of gsxui
// components (*.gsx.tmpl: gsx generate leaves them alone here), so in an app it is built from the app's own
// components, in the app's own style and theme. Their text is already translated, in locales/: add writes it into
// the app's catalogs under [screens], with the text the gsxui components write themselves under [gsxui]. A screen
// with routes of its own (the docs, the dialog's form) has them as types the app mounts in its page tree.
package screens

import (
	"embed"

	"github.com/joeblew999/charter/intl/registry"
)

//go:embed *.gsx.tmpl locales/*.toml
var files embed.FS

// Dir is where the screens go in an app, from its root: the package screens beside its pages.
const Dir = "pages/screens"

// Screens are charter's screens.
var Screens = &registry.Registry{
	Command: "charter add screen:<name>",
	Table:   "screens",
	Files:   files,
	Locales: "github.com/joeblew999/charter/app/screens/locales",
	Components: []registry.Component{
		{Name: "language-list", Gsxui: []string{"button", "icon"},
			Use: "  <screens.LanguageList/>       on every page, outside boosted links (a footer: switching language is a full page load)"},
		{Name: "language-offer", Gsxui: []string{"alert", "icon"},
			Use: "  <screens.LanguageOffer/>      at the top of every page's content"},
		{Name: "preferences", Gsxui: []string{"button", "dialog", "field", "native-select", "icon"},
			Use: "  <screens.PreferencesDialog/>  in the header, with its routes in the page tree:\n" +
				"    preferences screens.Preferences `route:\"/preferences Preferences\"`"},
		{Name: "theme-toggle", Gsxui: []string{"button", "icon"},
			Use: "  <screens.ThemeToggle/>        in the header: light and dark, of the app's gsxui theme (charter run ui:theme)"},
		{Name: "docs", Gsxui: []string{"badge", "button", "separator"}, Needs: []string{"theme-toggle"},
			Use: "  the docs' pages (docs/*.md), in the page tree, with the docs named in the App:\n" +
				"    docs screens.Docs `route:\"/docs Docs\"`\n" +
				"    Docs: &app.Docs{Name: \"My App\", Pages: docs.Pages}"},
	},
}
