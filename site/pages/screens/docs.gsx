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
	"github.com/joeblew999/charter/site/ui/icon"
	ui "github.com/joeblew999/charter/site/ui/styles"
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

// A page of the docs is loaded as a document, never put in place of another by a link: what marks the section being
// read as the page scrolls (gsxui's own script for its docs) starts as its document loads.
func (DocsIndex) FullLoad() bool { return true }
func (DocsPage) FullLoad() bool  { return true }

// DocsDocument is the page, in gsxui's own layout for a page of docs (Layout, LayoutDocs): the docs' pages at one
// side in their groups, the page itself at a width that reads well, its sections at the other side, and under it
// the pages before and after it and where it comes from. Where the window has no room the sections go, and then
// the pages, which are under the header's menu.
// The docs are written in one language (English), which the article says; what is around them is in the reader's.
component DocsDocument(v DocsView) {
	<Layout
		mode={LayoutDocs}
		title={v.Doc.Title + " · " + v.Docs.Name}
		description={v.Doc.Description}
		brand={ <DocsBrand v={v}/> }
		links={ <DocsLinks v={v}/> }
		tools={ <DocsTools v={v}/> }
		navigation={ <DocsList v={v}/> }
		contents={ <DocsSections v={v}/> }
	>
		// The page: what charter build wrote from docs/*.md, the app's own files. It is marked data-doc, which
		// gsxui's own stylesheet for its docs styles (web/gsxui-site/site.css), with a section under each heading,
		// and a table as gsxui's Table.
		<article id="doc" data-doc="docs" lang="en" dir="ltr" class="flex min-w-0 flex-col gap-10">
			{ for _, section := range v.Doc.Sections {
				<section class="flex min-w-0 flex-col gap-3">
					{ for _, part := range section.Parts {
						{ if part.Head == nil {
							{ gsx.Raw(part.HTML) }
						} else {
							<ui.Table>
								<ui.TableHeader>
									<ui.TableRow>
										{ for _, cell := range part.Head {
											<ui.TableHead>{ gsx.Raw(cell) }</ui.TableHead>
										} }
									</ui.TableRow>
								</ui.TableHeader>
								<ui.TableBody>
									{ for _, row := range part.Rows {
										<ui.TableRow>
											{ for _, cell := range row {
												<ui.TableCell class="align-top whitespace-normal">{ gsx.Raw(cell) }</ui.TableCell>
											} }
										</ui.TableRow>
									} }
								</ui.TableBody>
							</ui.Table>
						} }
					} }
				</section>
			} }
		</article>
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
	</Layout>
}

// DocsBrand is what the docs' header begins with: the docs' name, the way to the app's front page, and that these
// are its docs.
component DocsBrand(v DocsView) {
	<a href={intl.URL(ctx, "/")} class="flex items-center gap-2 font-semibold">
		<span translate="no">{ v.Docs.Name }</span>
		<ui.Badge variant="secondary">{ locales.ScreensDocs }</ui.Badge>
	</a>
}

// DocsLinks is the way to the repository, for docs that name one: in the header where there is room, under its
// menu where there is not. The layout shows it in both, so nothing in it has an id.
component DocsLinks(v DocsView) {
	{ if v.Docs.Repo != "" {
		<ui.Button variant="ghost" size="sm" href={v.Docs.Repo} rel="noopener" data-docs-repo>
			{ locales.ScreensDocsOnGithub(docsRepo(v.Docs.Repo)) }
		</ui.Button>
	} }
}

// DocsTools is what the docs' header always has: the search, the way to the theme editor, and light or dark.
// The search is as gsxui's own docs have it: every page and every section of one, by its title. Cmd-K (Ctrl-K)
// opens it.
component DocsTools(v DocsView) {
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
	<ThemeLink/>
	<ThemeToggle/>
}

// DocsList is the docs' pages, the one being read marked, in the groups the docs' front page lists them in
// (app.Docs.Groups), as gsxui's own docs list theirs. The layout shows it at the side and under the header's menu,
// so nothing in it has an id. Its links load the page they lead to, as every way to a page of the docs does.
component DocsList(v DocsView) {
	<div data-docs-list class="flex flex-col gap-4 text-sm" hx-boost:inherited="false">
		{ for _, group := range v.Docs.Groups() {
			<div class="flex flex-col gap-1">
				{ if group.Name != "" {
					<h3 class="px-2 text-xs font-semibold tracking-wide text-muted-foreground uppercase" lang="en" dir="ltr">
						{ group.Name }
					</h3>
				} }
				{ for _, other := range group.Pages {
					<a
						href={docsURL(ctx, other)}
						lang="en"
						dir="ltr"
						class={
							"rounded-md px-2 py-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
							"bg-accent text-accent-foreground": other.Slug == v.Doc.Slug
						}
						{ if other.Slug == v.Doc.Slug {
							aria-current="page"
						} }
					>
						{ other.Title }
					</a>
				} }
			</div>
		} }
	</div>
}

// DocsSections is the sections of the page being read, each a link to it, as gsxui's own docs list theirs: the one
// being read is marked as the page scrolls, by gsxui's own script (web/gsxui-site/web/site.js).
component DocsSections(v DocsView) {
	{ if len(v.Doc.Headings) > 0 {
		<nav id="docs-toc" aria-label={locales.ScreensDocsOnThisPage} class="flex flex-col gap-3 text-sm">
			<div class="font-medium">{ locales.ScreensDocsOnThisPage }</div>
			<div class="flex flex-col gap-2" lang="en" dir="ltr">
				{ for _, h := range v.Doc.Headings {
					<a
						data-site-toc-link
						href={"#" + h.ID}
						class={
							"text-muted-foreground transition-colors hover:text-foreground data-[active]:font-medium data-[active]:text-foreground",
							"ps-3": h.Depth == 3
						}
					>
						{ h.Title }
					</a>
				} }
			</div>
		</nav>
	} }
}

// DocsAround is the way to the page before the one being read and to the one after it, at its foot, each a gsxui
// Item: the docs read from one end to the other without going back to the list.
component DocsAround(v DocsView) {
	{ if docsBefore(v) != nil || docsAfter(v) != nil {
		<nav
			id="docs-around"
			aria-label={locales.ScreensDocs}
			class="mt-10 grid gap-3 border-t pt-6 sm:grid-cols-2"
			hx-boost:inherited="false"
		>
			{ if docsBefore(v) != nil {
				<a id="docs-previous" href={docsURL(ctx, *docsBefore(v))} class="rounded-lg">
					<ui.Item variant="outline" class="hover:bg-muted">
						<ui.ItemContent>
							<ui.ItemDescription>{ locales.ScreensDocsPrevious }</ui.ItemDescription>
							<ui.ItemTitle lang="en" dir="ltr">{ docsBefore(v).Title }</ui.ItemTitle>
						</ui.ItemContent>
					</ui.Item>
				</a>
			} else {
				<span class="hidden sm:block"></span>
			} }
			{ if docsAfter(v) != nil {
				<a id="docs-next" href={docsURL(ctx, *docsAfter(v))} class="rounded-lg">
					<ui.Item variant="outline" class="hover:bg-muted">
						<ui.ItemContent class="items-end text-end">
							<ui.ItemDescription>{ locales.ScreensDocsNext }</ui.ItemDescription>
							<ui.ItemTitle lang="en" dir="ltr">{ docsAfter(v).Title }</ui.ItemTitle>
						</ui.ItemContent>
					</ui.Item>
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

// docsRepo is the repository's name as a reader knows it: owner/name.
func docsRepo(url string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimRight(url, "/"), "https://"), "github.com/")
}

// docsClasses are the classes the HTML a page's Markdown became has on a list, a link, a heading and a sample's
// caption: the ones gsxui's own doc pages give those (charter build writes them, internal/docsgen). They are
// written here so that Tailwind, which reads the app's templates and not its data, has them.
const docsClasses = "list-disc list-decimal space-y-2 ps-6 scroll-mt-20 font-medium underline decoration-foreground/30 hover:decoration-foreground flex flex-col gap-1 font-mono text-xs text-muted-foreground"
