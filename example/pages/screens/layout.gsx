package screens

import (
	"context"

	"github.com/gsxhq/gsx"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui/icon"
	ui "github.com/joeblew999/charter/example/ui/styles"
	"github.com/joeblew999/charter/intl"
)

// LayoutMode says what a page is for. A page names its mode and never sets a width or builds a header.
type LayoutMode string

const (
	// LayoutMarketing is a centred page with a header and a footer: a front page, a page of the app.
	LayoutMarketing LayoutMode = "marketing"
	// LayoutDocs is a page to read, with its pages listed at the side in gsxui's Sidebar: beside the page on a
	// wide window, folded away by its button, and sliding in over the page on a phone.
	LayoutDocs LayoutMode = "docs"
	// LayoutWorkspace fills the window under the header, with no footer: the theme editor and its preview.
	LayoutWorkspace LayoutMode = "workspace"
)

// Layout is the document around every page of the app, the charter screens' too. Everything in it that moves is a
// gsxui component, as gsxui add brings them and keeps them: its Sidebar (which is a drawer on a phone by itself),
// its Sheet for what else has no room on a phone (the ways to the pages, a page's sections), its Separator. What
// is charter's is only where they are put. Nothing in a page can make the page wider than the window: every
// column here may shrink (min-w-0), so what is too wide scrolls where it is.
//
//   - brand: the app's name, at the start (LayoutBrand is the name as a link to the front page).
//   - links: the ways to the app's pages. In the header where there is room; in a drawer, from the header's menu
//     button, where there is not. Shown in both, so nothing in them has an id.
//   - tools: what is always in the header: the theme switch, who is signed in.
//   - navigation: a docs page's list of pages, as gsxui Sidebar groups (LayoutDocs).
//   - contents: a docs page's own sections: at the other side on a wide window, and under a button at the top of
//     the page where there is no side. Shown in both, so nothing in them has an id.
//   - heading: where a docs page is among the others, in its header.
//   - footer: under the page; a workspace has none.
//
// From charter add screen:layout: the app's own now, to change as it likes.
component Layout(mode LayoutMode, title string, description string, brand gsx.Node, links gsx.Node, tools gsx.Node, navigation gsx.Node, contents gsx.Node, heading gsx.Node, footer gsx.Node, children gsx.Node) {
	{{
		bodyClass := "min-h-svh bg-background font-sans text-foreground antialiased"
		containerClass := "mx-auto w-full max-w-6xl px-4"
		mainClass := "min-w-0 flex-1 py-10"
		if mode == LayoutWorkspace {
			bodyClass += " lg:flex lg:h-svh lg:flex-col lg:overflow-hidden"
			containerClass = "w-full px-4"
			mainClass = "min-w-0 flex-1 py-10 lg:min-h-0 lg:overflow-y-auto lg:py-4"
		}
	}}
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={title} description={description}/>
		</head>
		<body data-layout={string(mode)} class={ bodyClass } { app.Body()... }>
			{ if mode == LayoutDocs {
				<ui.SidebarProvider open={true}>
					<ui.Sidebar open={true} side={layoutSide(ctx)}>
						<ui.SidebarHeader>
							<ui.SidebarMenu>
								<ui.SidebarMenuItem>
									{ if brand != nil {
										{ brand }
									} }
								</ui.SidebarMenuItem>
							</ui.SidebarMenu>
						</ui.SidebarHeader>
						<ui.SidebarContent>
							{ if navigation != nil {
								{ navigation }
							} }
						</ui.SidebarContent>
						<ui.SidebarRail/>
					</ui.Sidebar>
					// What is beside the Sidebar may be narrower than what is in it: a wide table scrolls in its place.
					<ui.SidebarInset class="min-w-0">
						<header
							data-layout-header
							class="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-2 border-b bg-background/95 px-4 backdrop-blur supports-backdrop-filter:bg-background/80"
						>
							<ui.SidebarTrigger data-layout-menu/>
							<ui.Separator orientation="vertical" class="me-1 h-4"/>
							<div class="min-w-0 flex-1">
								{ if heading != nil {
									{ heading }
								} }
							</div>
							{ if links != nil {
								<div data-layout-links class="hidden items-center gap-1 md:flex">{ links }</div>
							} }
							{ if tools != nil {
								{ tools }
							} }
						</header>
						<div class="mx-auto flex w-full max-w-6xl min-w-0 gap-10 px-4 py-8 md:px-8">
							<main data-layout-main class="min-w-0 flex-1">
								{ if contents != nil {
									// The page's sections where there is no side for them: they come down from the top,
									// and go when one is chosen (gsxui closes a dialog from what is marked so).
									<ui.Sheet>
										<ui.SheetTrigger
											data-layout-contents-button
											class="mb-6 inline-flex h-8 items-center gap-1 rounded-md border px-2.5 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground xl:hidden"
										>
											{ locales.ScreensDocsOnThisPage }
											<icon.ChevronDown class="size-4"/>
										</ui.SheetTrigger>
										<ui.SheetContent
											side="top"
											data-layout-contents-sheet
											class="max-h-[80svh] gap-0 overflow-y-auto p-2"
										>
											<ui.SheetTitle class="px-2 pt-2">{ locales.ScreensDocsOnThisPage }</ui.SheetTitle>
											<div data-gsxui-dialog-close>{ contents }</div>
										</ui.SheetContent>
									</ui.Sheet>
								} }
								{ children }
							</main>
							{ if contents != nil {
								// At the other side: a gsxui Sidebar that does not fold, staying in view.
								<ui.Sidebar
									collapsible="none"
									data-layout-contents
									class="sticky top-20 hidden h-auto max-h-[calc(100svh-6rem)] w-56 shrink-0 self-start overflow-y-auto bg-transparent xl:flex"
								>
									<ui.SidebarGroup>
										<ui.SidebarGroupLabel>{ locales.ScreensDocsOnThisPage }</ui.SidebarGroupLabel>
										<ui.SidebarGroupContent>
											{ contents }
										</ui.SidebarGroupContent>
									</ui.SidebarGroup>
								</ui.Sidebar>
							} }
						</div>
					</ui.SidebarInset>
				</ui.SidebarProvider>
			} else {
				<header
					data-layout-header
					class="sticky top-0 z-30 shrink-0 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/80"
				>
					<div class={ "flex h-14 items-center gap-2", containerClass }>
						{ if links != nil {
							// Where the header has no room for the ways to the pages: a drawer, gsxui's Sheet, from a
							// button. It goes when one is chosen.
							<ui.Sheet>
								<ui.SheetTrigger
									data-layout-menu
									aria-label={locales.ScreensMenu}
									title={locales.ScreensMenu}
									class="inline-flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground md:hidden"
								>
									<icon.Menu class="size-4"/>
								</ui.SheetTrigger>
								<ui.SheetContent side={layoutSide(ctx)} data-layout-menu-sheet class="w-72 gap-0 p-4">
									<ui.SheetTitle>{ locales.ScreensMenu }</ui.SheetTitle>
									<div data-layout-menu-links data-gsxui-dialog-close class="flex flex-col items-start gap-1 pt-4">
										{ links }
									</div>
								</ui.SheetContent>
							</ui.Sheet>
						} }
						<div class="flex min-w-0 flex-1 items-center">
							{ if brand != nil {
								{ brand }
							} }
						</div>
						{ if links != nil {
							<div data-layout-links class="hidden items-center gap-1 md:flex">{ links }</div>
						} }
						{ if tools != nil {
							{ tools }
						} }
					</div>
				</header>
				<div class={ "flex", containerClass, "lg:min-h-0 lg:flex-1": mode == LayoutWorkspace }>
					<main data-layout-main class={ mainClass }>
						{ children }
					</main>
				</div>
				{ if mode != LayoutWorkspace && footer != nil {
					<footer data-layout-footer class="border-t">
						<div class={ "py-6 text-sm text-muted-foreground", containerClass }>
							{ footer }
						</div>
					</footer>
				} }
			} }
		</body>
	</html>
}

// LayoutBrand is the app's name (app.App's Name) as the way to its front page: what a header begins with. On one
// line: a name with no room is cut short, and the header stays its height.
component LayoutBrand() {
	<a href={intl.URL(ctx, "/")} class="min-w-0 truncate font-semibold" translate="no">{ app.Name(ctx) }</a>
}

// layoutSide is the side a drawer comes from and a list of pages is at: where a line starts, in the reader's
// language. gsxui's Sidebar and Sheet take a side of the window, not of the text.
func layoutSide(ctx context.Context) string {
	if intl.Current(ctx).Dir() == "rtl" {
		return "right"
	}
	return "left"
}
