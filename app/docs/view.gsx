package docs

import (
	"github.com/gsxhq/gsx"
	"github.com/gsxhq/gsxui/ui"

	"github.com/joeblew999/charter/app"
)

// Page is a page of the docs, for both of its routes.
component (p Index) Page(v View) {
	<Document v={v}/>
}

component (p Page) Page(v View) {
	<Document v={v}/>
}

// Document is the page: the docs' pages at the side, the page itself, its sections, and where it comes from. The
// docs are written in English, so what is around them here is too.
component Document(v View) {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={v.Doc.Title + " · " + v.Docs.Name} description={v.Doc.Description}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased" lang="en" dir="ltr">
			<header class="mx-auto flex max-w-6xl flex-wrap items-center gap-3 p-4">
				<a href={home(ctx)} class="font-semibold" translate="no">{ v.Docs.Name }</a>
				<ui.Badge variant="secondary">Docs</ui.Badge>
				<span class="flex-1"></span>
				{ if v.Docs.Repo != "" {
					<ui.Button variant="ghost" size="sm" href={v.Docs.Repo} rel="noopener" id="docs-repo">
						<span translate="no">{ repo(v.Docs.Repo) }</span>
						on GitHub
					</ui.Button>
				} }
			</header>
			<ui.Separator/>
			<div class="mx-auto flex max-w-6xl flex-col gap-8 p-4 md:flex-row">
				<nav id="docs-nav" aria-label="Docs" class="flex shrink-0 flex-col gap-1 md:w-52">
					{ for _, other := range v.Docs.Pages {
						<a
							href={link(ctx, other)}
							class={linkClass(other.Slug == v.Doc.Slug)}
							{ if other.Slug == v.Doc.Slug {
								aria-current="page"
							} }
						>
							{ other.Title }
						</a>
					} }
				</nav>
				<main class="min-w-0 flex-1">
					// The page: HTML that charter build wrote from docs/*.md, the app's own files.
					<article id="doc" class={prose}>{ gsx.Raw(v.Doc.HTML) }</article>
					{ if v.Docs.Edit(v.Doc) != "" {
						<p class="mt-10 border-t pt-4 text-sm text-muted-foreground">
							<a id="docs-edit" href={v.Docs.Edit(v.Doc)} rel="noopener" class="underline underline-offset-4">Edit this page on GitHub</a>
						</p>
					} }
				</main>
				{ if len(v.Doc.Headings) > 0 {
					<aside class="hidden shrink-0 lg:block lg:w-48">
						<p class="mb-2 text-sm font-medium">On this page</p>
						<ul id="docs-toc" class="flex flex-col gap-1 text-sm text-muted-foreground">
							{ for _, h := range v.Doc.Headings {
								<li class={tocClass(h.Depth)}>
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
