package intl

// Head is what <head> says about the page's languages: its canonical URL, the page in every shipped locale (hreflang,
// reciprocal, x-default the source locale's), and Open Graph's og:url, og:locale and og:locale:alternate. page is the
// path and the query parameters that make it a different page, without a locale prefix.
component Head(page string) {
	<link rel="canonical" href={Canonical(ctx, page)}/>
	{ for _, a := range Alternates(ctx, page) {
		<link rel="alternate" hreflang={a.Lang} href={a.Href}/>
	} }
	<meta property="og:url" content={Canonical(ctx, page)}/>
	<meta property="og:locale" content={OGLocale(Current(ctx))}/>
	{ for _, l := range Shipped() {
		{ if l != Current(ctx) {
			<meta property="og:locale:alternate" content={OGLocale(l)}/>
		} }
	} }
}
