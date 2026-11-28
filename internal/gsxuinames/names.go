// Package gsxuinames is the names gsxui gives its choices, in gsxui's order: its styles, its base colours and its
// accents. They are gsxui's (internal/preset/compact.go, "the append-only p1 transport ABI"), written here once
// for what makes a share code of them (internal/themes) and for what lists the themes on a page (app.Themes).
// internal/themes' test holds them to gsxui's own source.
package gsxuinames

var (
	// Styles are how the components are drawn: density, borders, shadows, type. One is compiled into an app.
	Styles = []string{"nova", "maia", "vega", "lyra", "mira", "luma", "sera", "rhea"}
	// Bases are the greys a theme is built on.
	Bases = []string{"neutral", "stone", "zinc", "mauve", "olive", "mist", "taupe"}
	// Accents are the themes with a colour: what a reader may choose beside the app's own.
	Accents = []string{"amber", "blue", "cyan", "emerald", "fuchsia", "green", "indigo", "lime", "orange", "pink", "purple", "red", "rose", "sky", "teal", "violet", "yellow"}
)
