package app

import "strings"

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
