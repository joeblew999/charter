package app

import (
	"slices"

	"github.com/joeblew999/charter/internal/gsxuinames"
)

// Themes are the themes a reader may choose beside the app's own, as the page runs: gsxui's accents, by gsxui's
// names and in its order (amber, blue … yellow). Each is in web/gsxui/themes.css, which charter check --fix writes
// from gsxui itself for the app's style and base colour: <html data-theme="blue"> has one, light and dark alike.
// The theme-picker screen lists them.
//
// A style (nova, lyra …) is not among them: gsxui compiles one into an app's components, and it is changed while
// developing (charter run ui:theme).
func Themes() []string { return slices.Clone(gsxuinames.Accents) }
