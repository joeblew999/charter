package screens

import (
	"github.com/gsxhq/gsx"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/site/locales"
	"github.com/joeblew999/charter/site/ui/icon"
	ui "github.com/joeblew999/charter/site/ui/styles"
)

// LayoutMode says what a page is for, as gsxui's own site says it of each of its pages (its site/pages/layout.gsx,
// and the design it is built to: one shell, three explicit modes, never inferred from the address).
type LayoutMode string

const (
	// LayoutMarketing is a centred page with a header and a footer: a front page, a page of the app.
	LayoutMarketing LayoutMode = "marketing"
	// LayoutDocs is a page to read: the pages at one side, an article of a readable width, its sections at the
	// other. The sections go where there is no room for them, and the pages into the header's menu below that.
	LayoutDocs LayoutMode = "docs"
	// LayoutWorkspace fills the window under the header, with no footer: the theme editor and its preview.
	LayoutWorkspace LayoutMode = "workspace"
)

// Layout is the document around every page of the app, the charter screens' too: gsxui's own site's shell, with
// its three modes, its widths and what it shows and hides at each window's width. A page says its mode and what
// goes where; it sets no width of its own and builds no header. Nothing in a page can make the page wider than the
// window: every column here may shrink (min-w-0), so what is too wide scrolls where it is.
//
//   - brand: the app's name, at the start of the header (LayoutBrand is the name as a link to the front page).
//   - links: the ways to the app's pages. In the header where there is room; in its menu where there is not.
//   - tools: what is always in the header: the theme switch, who is signed in.
//   - navigation, contents: a docs page's list of pages and its own sections.
//   - footer: under the page; a workspace has none.
//
// From charter add screen:layout: the app's own now, to change as it likes.
component Layout(mode LayoutMode, title string, description string, brand gsx.Node, links gsx.Node, tools gsx.Node, navigation gsx.Node, contents gsx.Node, footer gsx.Node, children gsx.Node) {
	{{
		headerContainerClass := "mx-auto flex h-14 items-center justify-between gap-2"
		headerClass := "sticky top-0 z-10 border-b border-border bg-background/95 backdrop-blur"
		contentContainerClass := "mx-auto w-full py-10"
		mainClass := "min-w-0 flex-1"
		bodyClass := "min-h-svh bg-background font-sans text-foreground antialiased"
		footerContainerClass := "mx-auto px-4 py-6 text-sm text-muted-foreground"
		// The menu is there where the links, or the docs' pages, are not.
		menuClass := "md:hidden"

		switch mode {
		case LayoutDocs:
			headerContainerClass += " max-w-[1568px] px-4 sm:px-6 lg:px-8"
			contentContainerClass += " max-w-[1568px] grid grid-cols-1 px-4 sm:px-6 lg:grid-cols-[288px_minmax(0,1fr)] lg:px-8 xl:grid-cols-[288px_minmax(0,1fr)_288px]"
			mainClass = "mx-auto w-full min-w-0 max-w-[640px]"
			footerContainerClass += " max-w-[1568px] sm:px-6 lg:px-8"
			menuClass = "lg:hidden"
		case LayoutWorkspace:
			headerContainerClass += " max-w-none px-4"
			headerClass += " shrink-0"
			contentContainerClass += " flex max-w-none px-4 lg:min-h-0 lg:flex-1 lg:py-4"
			mainClass += " lg:min-h-0 lg:overflow-y-auto"
			bodyClass += " lg:flex lg:h-svh lg:flex-col lg:overflow-hidden"
		default:
			headerContainerClass += " max-w-6xl px-4"
			contentContainerClass += " flex max-w-6xl px-4"
			footerContainerClass += " max-w-6xl"
		}
	}}
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={title} description={description}/>
		</head>
		<body data-layout={string(mode)} class={ bodyClass } { app.Body()... }>
			<header data-layout-header class={ headerClass }>
				<div class={ headerContainerClass }>
					<div class="flex min-w-0 items-center gap-2">
						{ if brand != nil {
							{ brand }
						} }
						{ if links != nil || navigation != nil {
							// Where the links and the docs' pages have no room: one button, and they are under it.
							<ui.Popover data-layout-menu class={ menuClass }>
								<ui.PopoverTrigger
									aria-label={locales.ScreensMenu}
									class="inline-flex h-8 items-center gap-2 rounded-md px-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
								>
									<icon.Menu class="size-4"/>
									<span>{ locales.ScreensMenu }</span>
								</ui.PopoverTrigger>
								<ui.PopoverContent class="flex max-h-[calc(100svh-5rem)] w-56 flex-col gap-4 overflow-y-auto p-3">
									{ if links != nil {
										<div data-layout-menu-links class="flex flex-col items-start gap-1">{ links }</div>
									} }
									{ if navigation != nil {
										<div>{ navigation }</div>
									} }
								</ui.PopoverContent>
							</ui.Popover>
						} }
					</div>
					<nav class="flex shrink-0 items-center gap-1">
						{ if links != nil {
							<div data-layout-links class="hidden items-center gap-1 md:flex">{ links }</div>
						} }
						{ if tools != nil {
							{ tools }
						} }
					</nav>
				</div>
			</header>
			<div class={ contentContainerClass }>
				{ if mode == LayoutDocs {
					<aside data-layout-navigation class="hidden min-w-0 lg:block">
						<nav class="sticky top-24 max-h-[calc(100svh-7rem)] overflow-y-auto pe-16 pb-1">
							{ if navigation != nil {
								{ navigation }
							} }
						</nav>
					</aside>
				} }
				<main data-layout-main class={ mainClass }>
					{ children }
				</main>
				{ if mode == LayoutDocs && contents != nil {
					<aside data-layout-contents class="hidden min-w-0 xl:block">
						<div class="sticky top-24 max-h-[calc(100svh-7rem)] overflow-y-auto ps-16 pb-1">
							{ contents }
						</div>
					</aside>
				} }
			</div>
			{ if mode != LayoutWorkspace && footer != nil {
				<footer data-layout-footer class="border-t border-border">
					<div class={ footerContainerClass }>
						{ footer }
					</div>
				</footer>
			} }
		</body>
	</html>
}

// LayoutBrand is the app's name (app.App's Name) as the way to its front page: what a header begins with.
component LayoutBrand() {
	<a href={intl.URL(ctx, "/")} class="flex items-center font-semibold" translate="no">{ app.Name(ctx) }</a>
}
