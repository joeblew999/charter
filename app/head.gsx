package app

import (
	"github.com/gsxhq/vite"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/seo"
)

// Head is a page's <head>, but its charset and viewport: its title and description as search engines and link
// previews read them, its languages (canonical URL, every language's alternate), and the site's own files as Vite
// built them. title and description are the page's own text, in its language.
component Head(title string, description string) {
	{{ h := headOf(ctx, "") }}
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1"/>
	<seo.Head
		title={title}
		description={description}
		siteName={h.Name}
		icon={h.Icon}
		home={h.Home}
		lang={h.Lang}
	/>
	<intl.Head page={h.Page}/>
	<theme/>
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

// theme is the light or dark theme of the app's gsxui preset (web/gsxui/theme.css: the class dark on <html>), as
// gsxui's own site does it. It runs before the first paint, so a page never shows in the wrong one: what the reader
// chose (kept by the browser), else what their system asks for. Any element with data-theme-toggle switches it:
// the theme-toggle screen is one.
component theme() {
	<script>
		(function () {
			var root = document.documentElement;
			try {
				var chosen = localStorage.getItem("theme");
				if (chosen === "dark" || (!chosen && matchMedia("(prefers-color-scheme: dark)").matches)) root.classList.add("dark");
			} catch (e) {}
			document.addEventListener("click", function (event) {
				if (!event.target.closest || !event.target.closest("[data-theme-toggle]")) return;
				var dark = root.classList.toggle("dark");
				try { localStorage.setItem("theme", dark ? "dark" : "light"); } catch (e) {}
			});
		})();
	</script>
}
