package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/site/locales"
	"github.com/joeblew999/charter/site/pages/screens"
	ui "github.com/joeblew999/charter/site/ui/styles"
)

// Page is the front page: what charter is, where to read, see and get it, and what it gives an app.
component (p Home) Page() {
	// charter's layout screen, which is gsxui's own site's shell, as a centred page.
	<screens.Layout
		mode={screens.LayoutMarketing}
		title={locales.SiteTitle |> text}
		description={locales.SiteDescription |> text}
		brand={ <screens.LayoutBrand/> }
		links={ <Links/> }
		tools={ <Tools/> }
	>
		<div class="flex flex-col gap-10">
			<section class="flex flex-col gap-4">
				<h1 class="text-4xl font-semibold tracking-tight">{ locales.HomeHeading }</h1>
				<p class="text-lg text-muted-foreground">{ locales.HomeIntro }</p>
				<div class="flex flex-wrap gap-3">
					<ui.Button href={screens.DocsIndex{} |> url}>{ locales.HomeDocs }</ui.Button>
					<ui.Button variant="outline" href={Example} rel="noopener">{ locales.HomeExample }</ui.Button>
				</div>
			</section>
			<section class="flex flex-col gap-2">
				<p class="text-sm text-muted-foreground">{ locales.HomeStart }</p>
				<pre
					class="overflow-x-auto rounded-lg border bg-muted p-4 text-sm"
					translate="no"
				><code>{ start }</code></pre>
			</section>
			<section class="grid gap-4 sm:grid-cols-2">
				<Gives title={locales.HomePagesTitle} about={locales.HomePages}/>
				<Gives title={locales.HomeApiTitle} about={locales.HomeApi}/>
				<Gives title={locales.HomeLiveTitle} about={locales.HomeLive}/>
				<Gives title={locales.HomeDatabaseTitle} about={locales.HomeDatabase}/>
				<Gives title={locales.HomeWorldTitle} about={locales.HomeWorld}/>
				<Gives title={locales.HomeTestsTitle} about={locales.HomeTests}/>
			</section>
		</div>
	</screens.Layout>
}

// Gives is one thing charter gives an app.
component Gives(title intl.Msg, about intl.Msg) {
	<ui.Card>
		<ui.CardHeader>
			<ui.CardTitle>
				<h2>{ title }</h2>
			</ui.CardTitle>
			<ui.CardDescription>{ about }</ui.CardDescription>
		</ui.CardHeader>
	</ui.Card>
}

var _ gsx.Node

// Links are the ways out of the front page. The layout shows them in the header, and under its menu where the
// window has no room, so nothing in them has an id.
component Links() {
	<ui.Button variant="ghost" size="sm" href={screens.DocsIndex{} |> url}>{ locales.HomeDocs }</ui.Button>
	<ui.Button variant="ghost" size="sm" href={Repo} rel="noopener" data-github>{ locales.HomeGithub }</ui.Button>
}

// Tools are what the header always has: the theme editor, and light or dark.
component Tools() {
	<screens.ThemeLink/>
	<screens.ThemeToggle/>
}
