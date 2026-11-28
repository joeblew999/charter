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
	<styled dev={h.Dev}/>
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

// styled keeps a page from being seen before it is styled, under charter dev. Vite's dev server gives the stylesheet
// by script (so a change to it shows without a reload), which comes after the page's markup: the page would be
// seen bare, then styled. It is hidden until then, as gsxui's own site does it. What is built for a deploy has its
// stylesheet as a link in the head, which a browser waits for by itself.
component styled(dev bool) {
	{ if dev {
		<style>
			html[data-loading] body {
				visibility: hidden;
			}

			html[data-loading] * {
				transition: none !important;
			}
		</style>
		<script>
			document.documentElement.dataset.loading = "true";
			var unhide = function () {
				document.documentElement.removeAttribute("data-loading");
			};
			var reveal = function () {
				requestAnimationFrame(function () { requestAnimationFrame(unhide); });
			};
			if (document.readyState === "loading") {
				document.addEventListener("DOMContentLoaded", reveal);
			} else {
				reveal();
			}
			setTimeout(unhide, 5000);
		</script>
	} }
}

// theme is the light or dark of the app's theme (the class dark on <html>), as gsxui's own site does it, and the
// theme a reader chose in gsxui's theme editor (the theme screen), which the browser keeps as that theme's CSS. It
// runs before the first paint, so a page never shows in the wrong one: light or dark as the reader chose, else as
// their system asks; the theme they chose, else the app's own (web/gsxui/theme.css). Any element with
// data-theme-toggle switches light and dark: the theme-toggle screen is one. The two are apart: a theme has its
// light and its dark.
component theme() {
	<script>
		(function () {
			var root = document.documentElement;
			// A reader's theme is gsxui's own CSS for it (:root and .dark), said of <html> so that it comes before the
			// app's own wherever the two are in the page.
			var chosenTheme = function () {
				var css = null;
				try { css = localStorage.getItem("gsxui-preset-css"); } catch (e) {}
				var style = document.getElementById("gsxui-preset");
				if (!css) {
					if (style) style.remove();
					return;
				}
				if (!style) {
					style = document.createElement("style");
					style.id = "gsxui-preset";
					document.head.appendChild(style);
				}
				style.textContent = css.replace(/:root/g, "html:root").replace(/\.dark/g, "html.dark");
			};
			try {
				var chosen = localStorage.getItem("theme");
				if (chosen === "dark" || (!chosen && matchMedia("(prefers-color-scheme: dark)").matches)) root.classList.add("dark");
			} catch (e) {}
			chosenTheme();
			document.addEventListener("charter:preset", chosenTheme);
			document.addEventListener("click", function (event) {
				if (!event.target.closest || !event.target.closest("[data-theme-toggle]")) return;
				var dark = root.classList.toggle("dark");
				try { localStorage.setItem("theme", dark ? "dark" : "light"); } catch (e) {}
			});
		})();
	</script>
}
