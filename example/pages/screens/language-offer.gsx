package screens

import (
	"github.com/gsxhq/gsx"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/example/ui/icon"
	"github.com/joeblew999/charter/intl"
)

// LanguageOffer offers this page in the reader's own language, written in that language, on every page until they
// choose one: the language they chose, or the one their browser asks for, when the site has it and the page is in
// another (intl.Suggested). A link, never a redirect, so search engines still reach every language version. "No
// thanks" (intl.Decline) keeps the page's language as their choice. Nothing renders when there is nothing to offer.
// Extra attributes fall through to the alert.
//
// From charter add screen:language-offer: the app's own now, to change as it likes.
component LanguageOffer(attrs gsx.Attrs) {
	{ if s, ok := intl.Suggested(ctx); ok {
		<ui.Alert lang={s.Lang} dir="auto" { attrs... }>
			<icon.Languages/>
			<ui.AlertTitle>
				<a href={s.Href} hreflang={s.Lang} class="underline underline-offset-4" hx-boost="false">{ locales.ScreensAlsoIn(s.Name).In(s.Lang) }</a>
			</ui.AlertTitle>
			<ui.AlertDescription>
				<a href={intl.Decline(ctx)} rel="nofollow" hx-boost="false">{ locales.ScreensNoThanks.In(s.Lang) }</a>
			</ui.AlertDescription>
		</ui.Alert>
	} }
}
