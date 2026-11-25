package pages

import (
	"strconv"

	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/examples/site/locales"
	"github.com/joeblew999/charter/examples/site/ui"
	"github.com/joeblew999/charter/intl"
)

// CloudState is what the Cloudflare page shows, read from the Worker's bindings (cloud_js.go).
type CloudState struct {
	OnWorkers bool     // false off Workers (go test): the bindings are workers-go's, which a Worker has
	Visits    int      // KV: how often the page was opened
	Note      string   // R2: the note kept in the bucket
	Queued    []string // Queues: the messages the consumer has taken, newest first
	Cron      string   // Cron Triggers: the schedule that last ran, and when
	Name      string   // a variable of the Worker
	Place     string   // what Cloudflare knows of the request: country and time zone
	Fetched   string   // fetch: what this Worker's own /healthz answered
}

// Cloud is the page at /cloud: one card per thing Cloudflare gives the Worker, each used through workers-go's own
// package, as simply as it can be. The forms are plain posts, answered with a redirect back here.
component Cloud(s CloudState) {
	<Layout title={locales.CloudTitle} description={locales.CloudDescription}>
		<h1 class="text-2xl font-semibold">{ locales.CloudHeading }</h1>
		<p class="text-muted-foreground">{ locales.CloudIntro }</p>
		<a href={"/" |> localize} class="text-sm underline underline-offset-4">{ locales.CloudBack }</a>
		{ if !s.OnWorkers {
			<p class="rounded-md border p-3 text-sm">{ locales.CloudOffWorkers }</p>
		} }
		<CloudCard name="KV" about={locales.CloudKv}>
			<p id="visits" class="text-3xl font-semibold tabular-nums">{ strconv.Itoa(s.Visits) }</p>
		</CloudCard>
		<CloudCard name="R2" about={locales.CloudR2}>
			<form method="post" action="/cloud/note" class="flex flex-wrap gap-2">
				<ui.Input name="note" class="flex-1" value={s.Note} dir="auto" maxlength="200" aria-label={locales.CloudNote}/>
				<ui.Button type="submit" variant="secondary">{ locales.CloudSave }</ui.Button>
			</form>
		</CloudCard>
		<CloudCard name="Queues" about={locales.CloudQueues}>
			<form method="post" action="/cloud/queue" class="flex flex-wrap gap-2">
				<ui.Input name="message" class="flex-1" required dir="auto" maxlength="80" aria-label={locales.CloudMessage}/>
				<ui.Button type="submit" variant="secondary">{ locales.CloudSend }</ui.Button>
			</form>
			<ul id="queued" class="flex flex-col gap-1 text-sm">
				{ for _, m := range s.Queued {
					<li dir="auto" class="rounded-md border px-2 py-1">{ m }</li>
				} }
			</ul>
		</CloudCard>
		<CloudCard name="Cron Triggers" about={locales.CloudCron}>
			<p id="cron" class="text-sm" translate="no">{ s.Cron }</p>
		</CloudCard>
		<CloudCard name="fetch" about={locales.CloudFetch}>
			<p id="fetched" class="text-sm" translate="no">{ s.Fetched }</p>
		</CloudCard>
		<CloudCard name="wrangler.jsonc" about={locales.CloudVars}>
			<p id="name" class="text-sm" translate="no">{ s.Name }</p>
			<p id="place" class="text-sm text-muted-foreground" translate="no">{ s.Place }</p>
		</CloudCard>
	</Layout>
}

// CloudCard is one binding: its name as Cloudflare writes it, what the card does with it, and the result.
component CloudCard(name string, about intl.Msg, children gsx.Node) {
	<ui.Card>
		<ui.CardHeader>
			<ui.CardTitle>
				<h2 translate="no">{ name }</h2>
			</ui.CardTitle>
			<ui.CardDescription>{ about }</ui.CardDescription>
		</ui.CardHeader>
		<ui.CardContent class="flex flex-col gap-3">{ children }</ui.CardContent>
	</ui.Card>
}
