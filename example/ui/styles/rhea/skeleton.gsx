// Written by charter check --fix: gsxui's rhea of ui/skeleton.gsx, as gsxui apply writes it. Do not edit: a reader who
// chooses rhea in the theme editor is shown it.

package rhea

import "github.com/gsxhq/gsx"

// Skeleton is the shadcn/ui Skeleton loading placeholder. Straight port; no
// divergences.
component Skeleton(attrs gsx.Attrs) {
	<div class={ "bg-muted rounded-2xl animate-pulse" } { attrs... } data-gsxui-slot-skeleton></div>
}
