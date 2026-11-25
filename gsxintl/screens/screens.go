// Package ui is intl's own screens, which mise-intl add copies into an app the way gsxui add copies components: the
// language list, the offer of the page in the reader's language, and the time zone and clock dialog. Each is a gsx
// template made of gsxui components (*.gsx.tmpl: gsx generate leaves them alone here), and their text is already
// translated, in locales/: add writes it into the app's catalogs under [intl], with the text the gsxui components
// write themselves under [gsxui]. They are a registry (intl/registry), as gsxkit's components are.
package screens

import (
	"embed"

	"github.com/joeblew999/charter/gsxintl/registry"
)

//go:embed *.gsx.tmpl locales/*.toml
var files embed.FS

// Screens are intl's screens.
var Screens = &registry.Registry{
	Command: "mise-intl add",
	Table:   "intl",
	Files:   files,
	Locales: "github.com/joeblew999/charter/gsxintl/screens/locales",
	Components: []registry.Component{
		{Name: "language-list", Gsxui: []string{"button"},
			Use: "  <intlui.LanguageList/>       on every page, outside boosted links (a footer: switching language is a full page load)"},
		{Name: "language-offer", Gsxui: []string{"alert"},
			Use: "  <intlui.LanguageOffer/>      at the top of every page's content"},
		{Name: "preferences", Gsxui: []string{"button", "dialog", "field", "native-select"},
			Use: "  <intlui.PreferencesDialog/>  in the header, with its two routes (site is your intl.Site):\n" +
				"    GET  /fragments/preferences  renders intlui.PreferencesForm(intl.ReturnTo(r), site.ConnectionZone(r))\n" +
				"    POST /preferences            site.Preferences"},
	},
}
