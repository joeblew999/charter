// Written by charter check --fix: gsxui's vega of ui/skeleton.gsx, as gsxui apply writes it. Do not edit: a reader who
// chooses vega in the theme editor is shown it.

package vega

import "github.com/gsxhq/gsx"

// Skeleton is the shadcn/ui Skeleton loading placeholder. Straight port; no
// divergences.
component Skeleton(attrs gsx.Attrs) {
	<div class={ "bg-muted rounded-md animate-pulse" } { attrs... } data-gsxui-slot-skeleton></div>
}
