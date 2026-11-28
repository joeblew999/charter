// Package screens is charter's own screens, which charter add screen:<name> copies into an app the way gsxui add
// copies components: the language list, the offer of the page in the reader's language, the time zone and clock
// dialog, the switch between the light and the dark theme, signing in with a passkey, and the docs' pages. Each is a gsx template made of gsxui
// components (*.gsx.tmpl: gsx generate leaves them alone here), so in an app it is built from the app's own
// components, in the app's own style and theme. Their text is already translated, in locales/: add writes it into
// the app's catalogs under [screens], with the text the gsxui components write themselves under [gsxui]. A screen
// with routes of its own (the docs, the dialog's form) has them as types the app mounts in its page tree.
package screens

import (
	"embed"

	"github.com/joeblew999/charter/api"
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
	// What the screens said before intl.sum recorded what charter shipped: an app that still says it follows.
	Superseded: map[string][]string{
		"screens.name":          {"Name"},
		"screens.invited_intro": {"Create an account below, or sign in with your passkey if you have one. The invitation is yours either way."},
	},
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
		{Name: "sign-in", Gsxui: []string{"badge", "button", "card", "input", "native-select", "separator", "icon"}, Schema: api.AccountsSchema,
			Use: "  the app's own people, signing in with a passkey; its routes in the page tree, and the app's Auth (api.NewAuth with api.People):\n" +
				"    signIn  screens.SignIn  `route:\"/sign-in SignIn\"`\n" +
				"    account screens.Account `route:\"/account Account\"`\n" +
				"    (&app.App{Auth: auth, SignIn: screens.SignInPage{}, Args: []any{auth}, RequestArgs: []any{auth.Caller}, …})\n" +
				"  <screens.AccountLink/>        in the header: who is signed in, or the way to sign in"},
		{Name: "admin", Gsxui: []string{"badge", "button", "card", "icon"}, Needs: []string{"sign-in"},
			Use: "  what an admin sees of the system: the app at a glance, what the Worker is bound to, what its databases hold. In the page tree:\n" +
				"    admin screens.Admin `route:\"/admin Admin\"`\n" +
				"  <screens.AdminLink/>          in the header: there for whoever may open it\n" +
				"  (&app.App{Facts: []app.Fact{{Label: locales.Notes, Value: …}}, …}: lines of the app's own on its overview)"},
		{Name: "docs", Gsxui: []string{"badge", "button", "separator", "command", "kbd", "sheet", "sidebar", "breadcrumb", "item", "icon"}, Needs: []string{"theme-toggle"},
			Use: "  the docs' pages (docs/*.md), in the page tree, with the docs named in the App:\n" +
				"    docs screens.Docs `route:\"/docs Docs\"`\n" +
				"    Docs: &app.Docs{Name: \"My App\", Pages: docs.Pages}"},
	},
}
