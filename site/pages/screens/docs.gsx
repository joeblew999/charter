package screens

import (
	"context"
	"net/http"
	"strings"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"
	"github.com/joeblew999/charter/app"
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

// Shared says a page of the docs is the same for every reader: Cloudflare keeps it, and answers a reader with it
// before the Worker runs.
func (DocsIndex) Shared() bool { return true }
func (DocsPage) Shared() bool  { return true }

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

// DocsDocument is the page, in the layout for a page of docs (Layout, LayoutDocs), which is gsxui's Sidebar: the
// docs' pages at the side in their groups (a drawer on a phone), where the page is among them at the top, the page
// itself, its sections at the other side (under a button at the top of the page on a narrow window), and under it
// the pages before and after it and where it comes from.
// The docs are written in one language (English), which the article says; what is around them is in the reader's.
component DocsDocument(v DocsView) {
	<Layout
		mode={LayoutDocs}
		title={v.Doc.Title + " · " + v.Docs.Name}
		description={v.Doc.Description}
		tools={ <DocsTools v={v}/> }
		navigation={ <DocsList v={v}/> }
		contents={ <DocsSections v={v}/> }
		here={ <DocsHere v={v}/> }
	>
		// The page: what charter build wrote from docs/*.md, the app's own files. It is marked data-doc, which
		// gsxui's own stylesheet for its docs styles (web/gsxui-site/site.css), with a section under each heading,
		// and a table as gsxui's Table.
		<article id="doc" data-doc="docs" lang="en" dir="ltr" class="flex max-w-3xl min-w-0 flex-col gap-10">
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
												<ui.TableCell class="min-w-40 align-top whitespace-normal">{ gsx.Raw(cell) }</ui.TableCell>
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
		// Where the page comes from: its own file, and the repository.
		<div class="mt-2 flex flex-wrap gap-x-4">
			{ if v.Docs.Edit(v.Doc) != "" {
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
			{ if v.Docs.Repo != "" {
				<ui.Button
					variant="link"
					size="sm"
					class="px-0 text-muted-foreground"
					href={v.Docs.Repo}
					rel="noopener"
					id="docs-repo"
				>
					{ locales.ScreensDocsOnGithub(docsRepo(v.Docs.Repo)) }
				</ui.Button>
			} }
		</div>
	</Layout>
}

// DocsHere is where the page is among the others, at the top, which stays as the page is read: its group, and
// itself. gsxui's Breadcrumb.
component DocsHere(v DocsView) {
	<ui.Breadcrumb class="min-w-0">
		<ui.BreadcrumbList class="flex-nowrap">
			{ if v.Doc.Group != "" {
				<ui.BreadcrumbItem class="hidden md:inline-flex">
					<span lang="en" dir="ltr">{ v.Doc.Group }</span>
				</ui.BreadcrumbItem>
				<ui.BreadcrumbSeparator class="hidden md:block"/>
			} }
			<ui.BreadcrumbItem class="min-w-0">
				<ui.BreadcrumbPage id="docs-here" lang="en" dir="ltr" class="truncate">{ v.Doc.Title }</ui.BreadcrumbPage>
			</ui.BreadcrumbItem>
		</ui.BreadcrumbList>
	</ui.Breadcrumb>
}

// DocsTools is what the docs add to the app's header: the search. It is as gsxui's own docs have it: every page and every section of one, by its title. Cmd-K (Ctrl-K)
// opens it.
component DocsTools(v DocsView) {
	<ui.CommandDialog
		title={locales.ScreensDocsSearch |> text}
		description={locales.ScreensDocsSearch |> text}
		trigger={
			<ui.DialogTrigger
				id="docs-search"
				aria-label={locales.ScreensDocsSearch}
				class="inline-flex h-8 items-center gap-2 rounded-lg border bg-muted/50 px-2.5 text-sm text-muted-foreground transition-colors hover:bg-muted lg:w-56"
			>
				<icon.Search class="size-4"/>
				<span class="hidden flex-1 text-start lg:inline">{ locales.ScreensDocsSearch }</span>
				<ui.Kbd class="hidden lg:inline-flex">⌘K</ui.Kbd>
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
}

// DocsList is the docs' pages, the one being read marked, in the groups the docs' front page lists them in
// (app.Docs.Groups): gsxui's Sidebar groups and menus. The Sidebar shows it twice, at the side and in what slides
// in on a phone, so nothing in it has an id.
component DocsList(v DocsView) {
	{ for _, group := range v.Docs.Groups() {
		<ui.SidebarGroup>
			{ if group.Name != "" {
				<ui.SidebarGroupLabel lang="en" dir="ltr">{ group.Name }</ui.SidebarGroupLabel>
			} }
			<ui.SidebarGroupContent>
				<ui.SidebarMenu>
					{ for _, other := range group.Pages {
						<ui.SidebarMenuItem>
							<ui.SidebarMenuButton
								href={docsURL(ctx, other)}
								isActive={other.Slug == v.Doc.Slug}
								class="h-auto py-1.5 whitespace-normal"
							>
								<span lang="en" dir="ltr">{ other.Title }</span>
							</ui.SidebarMenuButton>
						</ui.SidebarMenuItem>
					} }
				</ui.SidebarMenu>
			</ui.SidebarGroupContent>
		</ui.SidebarGroup>
	} }
}

// DocsSections is the sections of the page being read, each a link to it, as a gsxui Sidebar menu. The layout
// shows it at the side and under a button at the top of the page, so nothing in it has an id.
component DocsSections(v DocsView) {
	<ui.SidebarMenu data-docs-sections lang="en" dir="ltr">
		{ for _, h := range v.Doc.Headings {
			<ui.SidebarMenuItem>
				<ui.SidebarMenuButton
					size="sm"
					href={"#" + h.ID}
					class={ "h-auto py-1 whitespace-normal text-muted-foreground", "ps-5": h.Depth > 2 }
				>
					<span>{ h.Title }</span>
				</ui.SidebarMenuButton>
			</ui.SidebarMenuItem>
		} }
	</ui.SidebarMenu>
}

// DocsAround is the way to the page before the one being read and to the one after it, at its foot, each a gsxui
// Item: the docs read from one end to the other without going back to the list.
component DocsAround(v DocsView) {
	{ if docsBefore(v) != nil || docsAfter(v) != nil {
		<nav id="docs-around" aria-label={locales.ScreensDocs} class="mt-10 grid gap-3 border-t pt-6 sm:grid-cols-2">
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
