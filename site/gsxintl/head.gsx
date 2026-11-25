package gsxintl

import (
	"github.com/joeblew999/charter/site/intl"
)

// Head is what <head> says about the page's languages: its canonical URL, the page in every shipped locale (hreflang,
// reciprocal, x-default the source locale's), and Open Graph's og:url, og:locale and og:locale:alternate. page is the
// path and the query parameters that make it a different page, without a locale prefix.
component Head(page string) {
	<link rel="canonical" href={intl.Canonical(ctx, page)}/>
	{ for _, a := range intl.Alternates(ctx, page) {
		<link rel="alternate" hreflang={a.Lang} href={a.Href}/>
	} }
	<meta property="og:url" content={intl.Canonical(ctx, page)}/>
	<meta property="og:locale" content={intl.OGLocale(intl.Current(ctx))}/>
	{ for _, l := range intl.Shipped() {
		{ if l != intl.Current(ctx) {
			<meta property="og:locale:alternate" content={intl.OGLocale(l)}/>
		} }
	} }
}
