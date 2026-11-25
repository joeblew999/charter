package gsxapp

import (
	"github.com/gsxhq/vite"
	"github.com/joeblew999/charter/gsxintl"
	"github.com/joeblew999/charter/gsxseo"
)

// Head is a page's <head>, but its charset and viewport: its title and description as search engines and link
// previews read them, its languages (canonical URL, every language's alternate), and the site's own files as Vite
// built them. title and description are the page's own text, in its language.
component Head(title string, description string) {
	{{ h := headOf(ctx, "") }}
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1"/>
	<gsxseo.Head
		title={title}
		description={description}
		siteName={h.Name}
		icon={h.Icon}
		home={h.Home}
		lang={h.Lang}
	/>
	{ if h.Languages {
		<gsxintl.Head page={h.Page}/>
	} }
	<assets bundle={h.Assets}/>
}

// Entry loads another of Vite's entries (vite.config.ts's input), for the one page that needs it: its stylesheets,
// the chunks it imports, and its module.
component Entry(name string) {
	<assets bundle={headOf(ctx, name).Assets}/>
}

component assets(bundle vite.Bundle) {
	{ for _, href := range bundle.CSS {
		<link rel="stylesheet" href={href}/>
	} }
	{ for _, href := range bundle.Preloads {
		<link rel="modulepreload" href={href}/>
	} }
	{ for _, src := range bundle.JS {
		<script type="module" src={src}></script>
	} }
}
