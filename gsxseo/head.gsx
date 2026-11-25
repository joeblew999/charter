package gsxseo

import (
	"path"
)

// Head is what a page's <head> tells search engines and link previews, beside the page's language versions and
// canonical URL (with mise-intl, gsxintl.Head): its title and description, the same for Open Graph and a summary card,
// the site's icon, and on the home page the site's name as WebSite structured data, which Google shows beside the
// site's results. title and description are the page's own text in its language (with intl: { locales.Title |> text });
// home is the home page's absolute URL on the home page itself, "" on every other page; lang is the page's language,
// for the structured data. mise-seo audit checks every one of them.
component Head(title string, description string, siteName string, icon string, home string, lang string) {
	<title>{ title }</title>
	<meta name="description" content={description}/>
	<meta property="og:type" content="website"/>
	<meta property="og:site_name" content={siteName}/>
	<meta property="og:title" content={title}/>
	<meta property="og:description" content={description}/>
	<meta name="twitter:card" content="summary"/>
	<link
		rel="icon"
		href={icon}
		{ if t := iconType(icon); t != "" {
			type={t}
		} }
	/>
	{ if home != "" {
		<script
			type="application/ld+json"
		>@{ map[string]string{"@context": "https://schema.org", "@type": "WebSite", "name": siteName, "url": home, "inLanguage": lang} }</script>
	} }
}

// iconType is an icon's media type from its extension.
func iconType(icon string) string {
	switch path.Ext(icon) {
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".ico":
		return "image/x-icon"
	}
	return ""
}
