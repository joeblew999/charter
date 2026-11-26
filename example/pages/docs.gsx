package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/docs"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/intl"
)

// Page is a page of the docs, for both of its routes: the pages of the docs at the side, the page itself, and its
// sections. The prose is the Markdown's, in English; everything around it is the app's, in the reader's language.
component (p DocsIndex) Page(d app.Doc) {
	<DocView d={d}/>
}

component (p DocPage) Page(d app.Doc) {
	<DocView d={d}/>
}

component DocView(d app.Doc) {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={d.Title + " · charter"} description={d.Description}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased">
			<header class="mx-auto flex max-w-6xl flex-wrap items-center gap-3 p-4">
				<a href={Home{} |> url} class="font-semibold" translate="no">charter</a>
				<ui.Badge variant="secondary">{ locales.DocsNav }</ui.Badge>
				<span class="flex-1"></span>
				<ui.Button variant="ghost" size="sm" href={Home{} |> url}>{ locales.DocsNotes }</ui.Button>
				<ui.Button variant="ghost" size="sm" href={Bindings{} |> url}>{ locales.BindingsLink }</ui.Button>
			</header>
			<ui.Separator/>
			<div class="mx-auto flex max-w-6xl flex-col gap-8 p-4 md:flex-row">
				<nav id="docs-nav" class="flex shrink-0 flex-col gap-1 md:w-52" lang="en" dir="ltr">
					{ for _, other := range docs.Pages {
						<DocLink other={other} current={other.Slug == d.Slug}/>
					} }
				</nav>
				<main class="min-w-0 flex-1">
					{ if intl.Current(ctx).ID() != "en" {
						<p class="mb-4 rounded-md border p-3 text-sm text-muted-foreground">{ locales.DocsEnglish }</p>
					} }
					// The page: HTML that charter build wrote from docs/*.md, the app's own files.
					<article id="doc" lang="en" dir="ltr" class={ proseClass }>{ gsx.Raw(d.HTML) }</article>
				</main>
				{ if len(d.Headings) > 0 {
					<aside class="hidden shrink-0 lg:block lg:w-48">
						<p class="mb-2 text-sm font-medium">{ locales.DocsOnThisPage }</p>
						<ul id="docs-toc" class="flex flex-col gap-1 text-sm text-muted-foreground" lang="en" dir="ltr">
							{ for _, h := range d.Headings {
								<li class={ tocClass(h.Depth) }>
									<a href={"#" + h.ID} class="hover:text-foreground">{ h.Title }</a>
								</li>
							} }
						</ul>
					</aside>
				} }
			</div>
		</body>
	</html>
}

// DocLink is a page of the docs in the list at the side.
component DocLink(other app.Doc, current bool) {
	<a
		href={docURL(ctx, other)}
		class={ docLinkClass(current) }
		{ if current {
			aria-current="page"
		} }
	>
		{ other.Title }
	</a>
}
