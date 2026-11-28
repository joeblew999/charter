// Written by charter check --fix: gsxui's lyra of ui/skeleton.gsx, as gsxui apply writes it. Do not edit: a reader who
// chooses lyra in the theme editor is shown it.

package lyra

import "github.com/gsxhq/gsx"

// Skeleton is the shadcn/ui Skeleton loading placeholder. Straight port; no
// divergences.
component Skeleton(attrs gsx.Attrs) {
	<div class={ "bg-muted rounded-none animate-pulse" } { attrs... } data-gsxui-slot-skeleton></div>
}
