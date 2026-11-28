package app

import (
	"context"
	"strings"
)

// gsxui's theme editor, as charter check --fix wrote it into the app (internal/themes): its markup, with gsxui's
// catalogue of styles, base colours, themes, chart colours, menu accents, radii and fonts in it, and the app's own
// preset. The app's generated Go hands them over as the program starts; the theme screen shows them.
var themeEditor, themePreset string

// SetThemeEditor is called by the Go charter check --fix writes beside the theme screen
// (pages/screens/theme_editor.gen.go), and by nothing else.
func SetThemeEditor(markup, preset string) { themeEditor, themePreset = markup, preset }

// themePreviewPlaceholder is what the markup has where the address of the page the editor previews goes.
const themePreviewPlaceholder = "/__charter/theme-preview-address"

// ThemeEditor is gsxui's theme editor, trusted HTML: gsxui's own, rendered by gsxui's own Go when charter check
// --fix ran. preview is the page of the app it shows changing as a reader chooses (the app's front page). Empty for
// an app whose editor has not been written yet (go tool charter check --fix).
func ThemeEditor(preview string) string {
	return strings.ReplaceAll(themeEditor, themePreviewPlaceholder, preview)
}

// ThemePreset is the app's own preset (gsxui.preset.json), as JSON: where the editor starts for a reader who has
// chosen nothing.
func ThemePreset() string { return themePreset }

// StyleCookie is where a reader's choice of gsxui's style is kept: gsxui's theme editor in the app sets it
// (internal/themes/js/charter.js). A style is in the components' markup, so the Worker has to know it to answer;
// the rest of a preset is CSS and stays in the browser.
const StyleCookie = "gsxui-style"

type styleKey struct{}

// Style is the gsxui style the reader of this request chose in the theme editor, by its name (lyra, maia, ...), or
// "" for the app's own. The app's components ask it as they render (ui/styles, which charter check --fix writes):
// a name that is no style of gsxui's renders the app's own.
func Style(ctx context.Context) string {
	p, _ := ctx.Value(key{}).(page)
	if p.r == nil {
		return ""
	}
	if s, ok := p.memo.Load(styleKey{}); ok {
		return s.(string)
	}
	style := ""
	if c, err := p.r.Cookie(StyleCookie); err == nil {
		style = c.Value
	}
	p.memo.Store(styleKey{}, style)
	return style
}
