package pages

import (
	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/site/locales"
	"github.com/joeblew999/charter/site/pages/screens"
	"github.com/joeblew999/charter/site/ui"
)

// Page is the front page: what charter is, where to read, see and get it, and what it gives an app.
component (p Home) Page() {
	<!DOCTYPE html>
	<html { app.HTML(ctx)... }>
		<head>
			<app.Head title={locales.SiteTitle |> text} description={locales.SiteDescription |> text}/>
		</head>
		<body class="min-h-svh bg-background font-sans text-foreground antialiased">
			<header class="mx-auto flex max-w-4xl items-center gap-1 p-4">
				<span class="font-semibold" translate="no">charter</span>
				<span class="flex-1"></span>
				<ui.Button variant="ghost" size="sm" href={screens.DocsIndex{} |> url}>{ locales.HomeDocs }</ui.Button>
				<ui.Button variant="ghost" size="sm" href={Repo} rel="noopener" id="github">{ locales.HomeGithub }</ui.Button>
				<screens.ThemeToggle/>
			</header>
			<ui.Separator/>
			<main class="mx-auto flex max-w-4xl flex-col gap-10 p-4 py-12">
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
			</main>
		</body>
	</html>
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
