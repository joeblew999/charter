package screens

import (
	"context"
	"net/http"
	"strings"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/site/locales"
	"github.com/joeblew999/charter/site/ui"
	"github.com/joeblew999/charter/site/ui/icon"
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

// DocsDocument is the page: the docs' pages at the side in their groups, the page itself, its sections, the pages
// before and after it, and where it comes from.
// The docs are written in one language (English), which the article says; what is around them is in the reader's.
component DocsDocument(v DocsView) {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={v.Doc.Title + " · " + v.Docs.Name} description={v.Doc.Description}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased" { app.Body()... }>
			// The top of every page stays where it is as the page is read: the way to anywhere in the docs is always
			// one press away.
			<header
				class="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/80"
			>
				<div class="mx-auto flex max-w-7xl flex-wrap items-center gap-3 px-4 py-3">
					<a href={intl.URL(ctx, "/")} class="font-semibold" translate="no">{ v.Docs.Name }</a>
					<ui.Badge variant="secondary">{ locales.ScreensDocs }</ui.Badge>
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
						<ui.Button
							variant="ghost"
							size="sm"
							class="hidden sm:inline-flex"
							href={v.Docs.Repo}
							rel="noopener"
							id="docs-repo"
						>
							{ locales.ScreensDocsOnGithub(docsRepo(v.Docs.Repo)) }
						</ui.Button>
					} }
					<ThemeToggle/>
				</div>
				// Where there is no room for the list of pages at the side: a bar with the way to it, which slides in
				// over the page, and the page's own sections.
				<div class="flex items-center justify-between gap-3 border-t px-4 py-1.5 md:hidden">
					<ui.Sheet>
						<ui.SheetTrigger
							id="docs-menu"
							class="inline-flex h-8 items-center gap-2 rounded-md px-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground"
						>
							<icon.Menu class="size-4"/>
							{ locales.ScreensDocsMenu }
						</ui.SheetTrigger>
						<ui.SheetContent side="left" id="docs-menu-pages" class="w-72 gap-0 overflow-y-auto p-4">
							<ui.SheetTitle class="mb-3">{ v.Docs.Name }</ui.SheetTitle>
							<nav aria-label={locales.ScreensDocs} class="flex flex-col gap-1">
								<DocsList v={v}/>
							</nav>
						</ui.SheetContent>
					</ui.Sheet>
					{ if len(v.Doc.Headings) > 0 {
						// The page's sections come down from the top, and go again when one is chosen (gsxui closes a
						// dialog from anything in it marked data-gsxui-dialog-close; the link is then followed).
						<ui.Sheet>
							<ui.SheetTrigger
								id="docs-sections"
								class="inline-flex h-8 items-center gap-1 rounded-md px-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground"
							>
								{ locales.ScreensDocsOnThisPage }
								<icon.ChevronDown class="size-4"/>
							</ui.SheetTrigger>
							<ui.SheetContent side="top" id="docs-sections-list" class="max-h-[80svh] gap-0 overflow-y-auto p-4">
								<ui.SheetTitle class="mb-3">{ locales.ScreensDocsOnThisPage }</ui.SheetTitle>
								<DocsSections v={v} closes/>
							</ui.SheetContent>
						</ui.Sheet>
					} }
				</div>
			</header>
			<div class="mx-auto flex max-w-7xl gap-8 px-4 py-6">
				// The list of pages, in the groups the docs' front page has them in. It stays in view, and scrolls by
				// itself when it is longer than the window.
				<nav
					id="docs-nav"
					aria-label={locales.ScreensDocs}
					class="sticky top-20 hidden max-h-[calc(100svh-6rem)] w-56 shrink-0 flex-col gap-1 self-start overflow-y-auto pe-2 md:flex"
				>
					<DocsList v={v}/>
				</nav>
				<main class="min-w-0 flex-1">
					// The page: HTML that charter build wrote from docs/*.md, the app's own files.
					<article id="doc" lang="en" dir="ltr" class={ docsProse }>{ gsx.Raw(v.Doc.HTML) }</article>
					<DocsAround v={v}/>
					{ if v.Docs.Edit(v.Doc) != "" {
						<ui.Button
							variant="link"
							size="sm"
							class="mt-2 px-0 text-muted-foreground"
							href={v.Docs.Edit(v.Doc)}
							rel="noopener"
							id="docs-edit"
						>
							{ locales.ScreensDocsEdit }
						</ui.Button>
					} }
				</main>
				{ if len(v.Doc.Headings) > 0 {
					<aside
						class="sticky top-20 hidden max-h-[calc(100svh-6rem)] w-52 shrink-0 self-start overflow-y-auto xl:block"
					>
						<p class="mb-2 text-sm font-medium">{ locales.ScreensDocsOnThisPage }</p>
						<div id="docs-toc">
							<DocsSections v={v}/>
						</div>
					</aside>
				} }
			</div>
		</body>
	</html>
}

// DocsList is the docs' pages, the one being read marked, in the groups the docs' front page lists them in
// (app.Docs.Groups): at the side, and in what slides in where there is no side.
component DocsList(v DocsView) {
	{ for _, group := range v.Docs.Groups() {
		{ if group.Name != "" {
			<p
				lang="en"
				dir="ltr"
				class="mt-4 mb-1 px-3 text-xs font-semibold tracking-wide text-muted-foreground uppercase first:mt-0"
			>
				{ group.Name }
			</p>
		} }
		{ for _, other := range group.Pages {
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
	} }
}

// DocsSections is the sections of the page being read, each a link to it: at the side, and under "On this page"
// where there is no side, which closes as one is chosen (closes).
component DocsSections(v DocsView, closes bool) {
	<ul lang="en" dir="ltr" class="flex flex-col gap-1 text-sm text-muted-foreground">
		{ for _, h := range v.Doc.Headings {
			<li class={ docsDepth(h.Depth) }>
				<a
					href={"#" + h.ID}
					class="block py-0.5 hover:text-foreground"
					{ if closes {
						data-gsxui-dialog-close
					} }
				>
					{ h.Title }
				</a>
			</li>
		} }
	</ul>
}

// DocsAround is the way to the page before the one being read and to the one after it, at its foot: the docs read
// from one end to the other without going back to the list.
component DocsAround(v DocsView) {
	{ if docsBefore(v) != nil || docsAfter(v) != nil {
		<nav
			id="docs-around"
			aria-label={locales.ScreensDocs}
			class="mt-10 grid max-w-3xl gap-3 border-t pt-6 sm:grid-cols-2"
		>
			{ if docsBefore(v) != nil {
				<a
					id="docs-previous"
					href={docsURL(ctx, *docsBefore(v))}
					class="flex flex-col gap-1 rounded-lg border p-3 transition-colors hover:border-foreground/40"
				>
					<span class="text-xs text-muted-foreground">{ locales.ScreensDocsPrevious }</span>
					<span lang="en" dir="ltr" class="font-medium">{ docsBefore(v).Title }</span>
				</a>
			} else {
				<span class="hidden sm:block"></span>
			} }
			{ if docsAfter(v) != nil {
				<a
					id="docs-next"
					href={docsURL(ctx, *docsAfter(v))}
					class="flex flex-col gap-1 rounded-lg border p-3 text-end transition-colors hover:border-foreground/40"
				>
					<span class="text-xs text-muted-foreground">{ locales.ScreensDocsNext }</span>
					<span lang="en" dir="ltr" class="font-medium">{ docsAfter(v).Title }</span>
				</a>
			} }
		</nav>
	} }
}

// docsBefore and docsAfter are the pages before and after the one being read, nil at either end.
func docsBefore(v DocsView) *app.Doc {
	before, _ := v.Docs.Around(v.Doc)
	return before
}

func docsAfter(v DocsView) *app.Doc {
	_, after := v.Docs.Around(v.Doc)
	return after
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
const docsProse = "flex max-w-3xl min-w-0 flex-col gap-4 leading-7 " +
	// Nothing in a page is wider than the page: a table or a sample that is scrolls by itself, on a phone too.
	"[&>*]:min-w-0 [&>*]:max-w-full [&_table]:block [&_table]:overflow-x-auto " +
	"[&_h1]:text-3xl [&_h1]:font-semibold [&_h1]:tracking-tight " +
	"[&_h2]:mt-6 [&_h2]:scroll-mt-28 [&_h2]:border-b [&_h2]:pb-2 [&_h2]:text-xl [&_h2]:font-semibold " +
	"[&_h3]:mt-2 [&_h3]:scroll-mt-28 [&_h3]:text-lg [&_h3]:font-medium " +
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
