package screens

import (
	"context"

	"github.com/gsxhq/gsx"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/site/locales"
	"github.com/joeblew999/charter/site/ui/icon"
	ui "github.com/joeblew999/charter/site/ui/styles"
)

// LayoutMode says what a page is for. A page names its mode and nothing else: no width, no header, no way back.
type LayoutMode string

const (
	// LayoutMarketing is a front page: as wide as the header.
	LayoutMarketing LayoutMode = "marketing"
	// LayoutPage is a page of the app (a list, a form, signing in, the admin's pages): one column at a width that
	// reads, the same on every one of them.
	LayoutPage LayoutMode = "page"
	// LayoutDocs is a page to read, with its pages listed at the side in gsxui's Sidebar: beside the page on a
	// wide window, folded away by its button, and sliding in over the page on a phone.
	LayoutDocs LayoutMode = "docs"
	// LayoutWorkspace fills the window under the header, with no footer: the theme editor and its preview.
	LayoutWorkspace LayoutMode = "workspace"
)

// Around is what every page of the app has around it, which the app says once, as it starts (pages/layout.gsx):
// the ways to its pages, what its header always has, and what is under every page. Every page then has the same
// header and the same footer, charter's screens too: none passes its own.
var Around struct {
	// Links are the ways to the app's pages: in the header where there is room, and in a drawer (the header's menu
	// button; on a page of docs, the list of pages) where there is not. Shown in both, so nothing in them has an id.
	Links gsx.Node
	// Tools are what the header always has: who is signed in, the theme editor, light or dark.
	Tools gsx.Node
	// Footer is under every page but a workspace.
	Footer gsx.Node
}

// Layout is the document around every page of the app, the charter screens' too: one header (the app's name, its
// links, its tools: Around), the page, one footer. Everything in it that moves is a gsxui component, as gsxui add
// brings them and keeps them: its Sidebar (which is a drawer on a phone by itself), its Sheet for what else has no
// room on a phone, its Separator. Nothing in a page can make the page wider than the window: every column here may
// shrink (min-w-0), so what is too wide scrolls where it is.
//
// What a page may add, and only where it is the page's own:
//
//   - tools: before the app's own in the header (the docs' search).
//   - navigation: a docs page's list of pages, as gsxui Sidebar groups (LayoutDocs).
//   - contents: a docs page's own sections: at the other side on a wide window, and under a button at the top of
//     the page where there is no side. Shown in both, so nothing in them has an id.
//   - heading: where a docs page is among the others, in its header.
//
// From charter add screen:layout: the app's own now, to change as it likes.
component Layout(mode LayoutMode, title string, description string, tools gsx.Node, navigation gsx.Node, contents gsx.Node, heading gsx.Node, children gsx.Node) {
	{{
		bodyClass := "min-h-svh bg-background font-sans text-foreground antialiased"
		containerClass := "mx-auto w-full max-w-6xl px-4"
		mainClass := "min-w-0 flex-1 py-10"
		links, footer := Around.Links, Around.Footer
		if mode == LayoutPage {
			mainClass = "mx-auto flex w-full max-w-3xl min-w-0 flex-col gap-6 py-10"
		}
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
									<ui.SidebarMenuButton size="lg" href={intl.URL(ctx, "/")}>
										<span class="truncate font-semibold" translate="no">{ app.Name(ctx) }</span>
									</ui.SidebarMenuButton>
								</ui.SidebarMenuItem>
							</ui.SidebarMenu>
						</ui.SidebarHeader>
						<ui.SidebarContent>
							{ if links != nil {
								// The ways to the app's pages, where the header has no room for them.
								<div data-layout-menu-links class="flex flex-col items-start gap-1 p-2 xl:hidden">{ links }</div>
							} }
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
								// Beside the Sidebar the header is narrower: the links are in it on a wide window, and at
								// the top of the list of pages below that.
								<div data-layout-links class="hidden items-center gap-1 xl:flex">{ links }</div>
							} }
							<LayoutTools tools={tools}/>
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
						<LayoutFooter footer={footer} class="mt-auto px-4 md:px-8"/>
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
							<a href={intl.URL(ctx, "/")} class="min-w-0 truncate font-semibold" translate="no">{ app.Name(ctx) }</a>
						</div>
						{ if links != nil {
							<div data-layout-links class="hidden items-center gap-1 md:flex">{ links }</div>
						} }
						<LayoutTools tools={tools}/>
					</div>
				</header>
				<div class={ "flex", containerClass, "lg:min-h-0 lg:flex-1": mode == LayoutWorkspace }>
					<main data-layout-main class={ mainClass }>
						{ children }
					</main>
				</div>
				{ if mode != LayoutWorkspace {
					<LayoutFooter footer={footer} class={ containerClass }/>
				} }
			} }
		</body>
	</html>
}

// LayoutTools is what a header ends with: what the page adds of its own, then what the app's header always has.
component LayoutTools(tools gsx.Node) {
	{ if tools != nil {
		{ tools }
	} }
	{ if Around.Tools != nil {
		{ Around.Tools }
	} }
}

// LayoutFooter is what is under a page: the app's footer, the same under every one.
component LayoutFooter(footer gsx.Node, class string) {
	{ if footer != nil {
		<footer data-layout-footer class="border-t">
			<div class={ "py-6 text-sm text-muted-foreground", class }>
				{ footer }
			</div>
		</footer>
	} }
}

// layoutSide is the side a drawer comes from and a list of pages is at: where a line starts, in the reader's
// language. gsxui's Sidebar and Sheet take a side of the window, not of the text.
func layoutSide(ctx context.Context) string {
	if intl.Current(ctx).Dir() == "rtl" {
		return "right"
	}
	return "left"
}
