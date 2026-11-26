package screens

import (
	"context"
	"net/http"
	"strings"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/example/ui/icon"
	"github.com/joeblew999/charter/intl"
)

// Docs is the app's docs as a group of pages: /docs/ is docs/index.md, /docs/<name> every other Markdown file of
// docs/. Mount it in the tree at /docs, and name the docs in the App:
//
//	docs screens.Docs `route:"/docs Docs"`
//
//	Docs: &app.Docs{Name: "My App", Pages: docs.Pages}
//
// From charter add screen:docs: the app's own now, to change as it likes.
type Docs struct {
	index DocsIndex `route:"GET /{$} Docs"`
	page  DocsPage  `route:"GET /{slug} Doc"`
}

// DocsIndex is docs/index.md.
type DocsIndex struct{}

// DocsPage is every other page of the docs.
type DocsPage struct{}

// DocsView is what a page of the docs shows: the page, and the docs it is one of.
type DocsView struct {
	Docs *app.Docs
	Doc  app.Doc
}

func (DocsIndex) Props(docs *app.Docs) (DocsView, error) { return docsView(docs, "") }

func (DocsPage) Props(r *http.Request, docs *app.Docs) (DocsView, error) {
	return docsView(docs, r.PathValue("slug"))
}

func docsView(docs *app.Docs, slug string) (DocsView, error) {
	doc, ok := docs.Find(slug)
	if !ok {
		return DocsView{}, app.NotFound("no such page of the docs")
	}
	return DocsView{docs, doc}, nil
}

component (p DocsIndex) Page(v DocsView) {
	<DocsDocument v={v}/>
}

component (p DocsPage) Page(v DocsView) {
	<DocsDocument v={v}/>
}

// DocsDocument is the page: the docs' pages at the side, the page itself, its sections, and where it comes from.
// The docs are written in one language (English), which the article says; what is around them is in the reader's.
component DocsDocument(v DocsView) {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={v.Doc.Title + " · " + v.Docs.Name} description={v.Doc.Description}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased">
			<header class="mx-auto flex max-w-6xl flex-wrap items-center gap-3 p-4">
				<a href={intl.URL(ctx, "/")} class="font-semibold" translate="no">{ v.Docs.Name }</a>
				<ui.Badge variant="secondary">{ locales.ScreensDocs }</ui.Badge>
				// The list of pages, where there is no room for it at the side.
				<ui.Popover class="md:hidden">
					<ui.PopoverTrigger
						aria-label={locales.ScreensDocs}
						class="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground"
					>
						<icon.Menu class="size-4"/>
					</ui.PopoverTrigger>
					<ui.PopoverContent class="max-h-[calc(100svh-5rem)] w-64 overflow-y-auto p-2">
						<nav aria-label={locales.ScreensDocs} class="flex flex-col gap-1">
							<DocsList v={v}/>
						</nav>
					</ui.PopoverContent>
				</ui.Popover>
				<span class="flex-1"></span>
				// Search, as gsxui's own docs have it: every page and every section of one, by its title. Cmd-K
				// (Ctrl-K) opens it.
				<ui.CommandDialog
					title={locales.ScreensDocsSearch |> text}
					description={locales.ScreensDocsSearch |> text}
					trigger={
						<ui.DialogTrigger
							id="docs-search"
							aria-label={locales.ScreensDocsSearch}
							class="inline-flex h-8 items-center gap-2 rounded-lg border bg-muted/50 px-2.5 text-sm text-muted-foreground transition-colors hover:bg-muted sm:w-56"
						>
							<icon.Search class="size-4"/>
							<span class="hidden flex-1 text-start sm:inline">{ locales.ScreensDocsSearch }</span>
							<ui.Kbd class="hidden sm:inline-flex">⌘K</ui.Kbd>
						</ui.DialogTrigger>
					}
				>
					<ui.CommandInput placeholder={locales.ScreensDocsSearch |> text}/>
					<ui.CommandList>
						<ui.CommandEmpty>{ locales.ScreensDocsNone }</ui.CommandEmpty>
						{ for _, page := range v.Docs.Pages {
							<ui.CommandGroup heading={page.Title} lang="en" dir="ltr">
								<ui.CommandItem data-href={docsURL(ctx, page)}>{ page.Title }</ui.CommandItem>
								{ for _, h := range page.Headings {
									<ui.CommandItem data-href=f`@{docsURL(ctx, page)}#@{h.ID}` value={page.Title + " " + h.Title}>
										{ h.Title }
									</ui.CommandItem>
								} }
							</ui.CommandGroup>
						} }
					</ui.CommandList>
				</ui.CommandDialog>
				{ if v.Docs.Repo != "" {
					<ui.Button variant="ghost" size="sm" href={v.Docs.Repo} rel="noopener" id="docs-repo">
						{ locales.ScreensDocsOnGithub(docsRepo(v.Docs.Repo)) }
					</ui.Button>
				} }
				<ThemeToggle/>
			</header>
			<ui.Separator/>
			<div class="mx-auto flex max-w-6xl flex-col gap-8 p-4 md:flex-row">
				<nav id="docs-nav" aria-label={locales.ScreensDocs} class="hidden shrink-0 flex-col gap-1 md:flex md:w-52">
					<DocsList v={v}/>
				</nav>
				<main class="min-w-0 flex-1">
					// The page: HTML that charter build wrote from docs/*.md, the app's own files.
					<article id="doc" lang="en" dir="ltr" class={ docsProse }>{ gsx.Raw(v.Doc.HTML) }</article>
					{ if v.Docs.Edit(v.Doc) != "" {
						<ui.Separator class="mt-10 mb-2"/>
						<ui.Button
							variant="link"
							size="sm"
							class="px-0 text-muted-foreground"
							href={v.Docs.Edit(v.Doc)}
							rel="noopener"
							id="docs-edit"
						>
							{ locales.ScreensDocsEdit }
						</ui.Button>
					} }
				</main>
				{ if len(v.Doc.Headings) > 0 {
					<aside class="hidden shrink-0 lg:block lg:w-48">
						<p class="mb-2 text-sm font-medium">{ locales.ScreensDocsOnThisPage }</p>
						<ul id="docs-toc" lang="en" dir="ltr" class="flex flex-col gap-1 text-sm text-muted-foreground">
							{ for _, h := range v.Doc.Headings {
								<li class={ docsDepth(h.Depth) }>
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

// DocsList is the docs' pages, the one being read marked: at the side, and in the menu where there is no side.
component DocsList(v DocsView) {
	{ for _, other := range v.Docs.Pages {
		<ui.Button
			variant={docsVariant(other.Slug == v.Doc.Slug)}
			size="sm"
			class="h-auto justify-start py-1.5 text-start whitespace-normal"
			href={docsURL(ctx, other)}
			{ if other.Slug == v.Doc.Slug {
				aria-current="page"
			} }
		>
			<span lang="en" dir="ltr">{ other.Title }</span>
		</ui.Button>
	} }
}

// docsURL is a docs page's address, from the tree, in the reader's language.
func docsURL(ctx context.Context, d app.Doc) (string, error) {
	if d.Slug == "" {
		return structpages.URLFor(ctx, DocsIndex{})
	}
	return structpages.URLFor(ctx, DocsPage{}, "slug", d.Slug)
}

// docsVariant marks the page being read in the list of pages.
func docsVariant(current bool) string {
	if current {
		return "secondary"
	}
	return "ghost"
}

// docsDepth indents a section under a section.
func docsDepth(depth int) string {
	if depth > 2 {
		return "ps-3"
	}
	return ""
}

// docsRepo is the repository's name as a reader knows it: owner/name.
func docsRepo(url string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimRight(url, "/"), "https://"), "github.com/")
}

// docsProse styles the HTML the Markdown became: Tailwind classes on the elements inside the article, in the app's
// gsxui tokens, since the HTML itself carries none.
const docsProse = "flex max-w-3xl flex-col gap-4 leading-7 " +
	"[&_h1]:text-3xl [&_h1]:font-semibold [&_h1]:tracking-tight " +
	"[&_h2]:mt-6 [&_h2]:scroll-mt-4 [&_h2]:border-b [&_h2]:pb-2 [&_h2]:text-xl [&_h2]:font-semibold " +
	"[&_h3]:mt-2 [&_h3]:scroll-mt-4 [&_h3]:text-lg [&_h3]:font-medium " +
	"[&_a]:underline [&_a]:underline-offset-4 " +
	"[&_ul]:list-disc [&_ul]:space-y-1 [&_ul]:ps-6 [&_ol]:list-decimal [&_ol]:space-y-1 [&_ol]:ps-6 " +
	"[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-sm " +
	"[&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:bg-muted [&_pre]:p-4 [&_pre_code]:bg-transparent [&_pre_code]:p-0 " +
	"[&_table]:w-full [&_table]:text-sm [&_th]:border-b [&_th]:px-3 [&_th]:py-2 [&_th]:text-start [&_th]:font-medium " +
	"[&_td]:border-b [&_td]:px-3 [&_td]:py-2 [&_td]:align-top " +
	// A sample taken from a file says which, above the code.
	"[&_figure]:flex [&_figure]:flex-col [&_figure]:gap-1 [&_figcaption]:font-mono [&_figcaption]:text-xs [&_figcaption]:text-muted-foreground " +
	// Code, by what each word is (the classes charter build gives it), in the theme's own colours: so it follows
	// the app's theme, light and dark.
	"[&_pre_.k]:font-medium [&_pre_.kd]:font-medium [&_pre_.kn]:font-medium [&_pre_.kr]:font-medium [&_pre_.kc]:font-medium " +
	"[&_pre_.k]:text-chart-1 [&_pre_.kd]:text-chart-1 [&_pre_.kn]:text-chart-1 [&_pre_.kr]:text-chart-1 [&_pre_.kc]:text-chart-1 [&_pre_.kt]:text-chart-1 " +
	"[&_pre_.s]:text-chart-2 [&_pre_.s1]:text-chart-2 [&_pre_.s2]:text-chart-2 [&_pre_.sb]:text-chart-2 [&_pre_.se]:text-chart-2 " +
	"[&_pre_.m]:text-chart-4 [&_pre_.mi]:text-chart-4 [&_pre_.mf]:text-chart-4 " +
	"[&_pre_.nf]:text-chart-3 [&_pre_.nb]:text-chart-3 [&_pre_.nt]:text-chart-3 [&_pre_.na]:text-chart-3 " +
	// A comment is set apart by its slant alone: a paler colour on the code's own background is too faint to read.
	"[&_pre_.c]:italic [&_pre_.c1]:italic [&_pre_.cm]:italic"
