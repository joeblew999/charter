package pages

import (
	"strconv"

	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/examples/site/cloud"
	"github.com/joeblew999/charter/examples/site/locales"
	"github.com/joeblew999/charter/examples/site/ui"
	"github.com/joeblew999/charter/intl"
)

// Cloud is the page at /cloud: one card per thing Cloudflare gives the Worker, each used through workers-go's own
// package, as simply as it can be (package cloud, a file each). The forms are plain posts, answered with a redirect back here.
component Cloud(s cloud.State) {
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
		<CloudCard name="D1" about={locales.CloudD1}>
			<p id="rows" class="text-3xl font-semibold tabular-nums">{ s.Rows }</p>
		</CloudCard>
		<CloudCard name="Cache API" about={locales.CloudCache}>
			<p id="cached" class="text-sm" translate="no">{ s.Cached }</p>
		</CloudCard>
		<CloudCard name="Durable Objects" about={locales.CloudObject}>
			<p id="object" class="text-sm" translate="no">{ s.Object }</p>
		</CloudCard>
		<CloudCard name="TCP sockets" about={locales.CloudSocket}>
			<p id="socket" class="text-sm" translate="no">{ s.Socket }</p>
		</CloudCard>
		<CloudCard name="waitUntil" about={locales.CloudAfter}>
			<p id="after" class="text-sm" translate="no">{ s.After }</p>
		</CloudCard>
		<CloudCard name="request.cf" about={locales.CloudRequest}>
			<p id="request" class="text-sm" translate="no">{ s.Request }</p>
		</CloudCard>
		<CloudCard name="Cron Triggers" about={locales.CloudCron}>
			<p id="cron" class="text-sm" translate="no">{ s.Cron }</p>
		</CloudCard>
		<CloudCard name="Email" about={locales.CloudEmail}>
			<form method="post" action="/cloud/email" class="flex flex-wrap gap-2">
				<ui.Input
					name="to"
					type="email"
					class="flex-1"
					required
					placeholder="you@example.com"
					aria-label={locales.CloudEmailTo}
				/>
				<ui.Button type="submit" variant="secondary">{ locales.CloudSend }</ui.Button>
			</form>
			<p id="sent" class="text-sm" translate="no">{ s.Sent }</p>
			<p id="received" class="text-sm text-muted-foreground" translate="no">{ s.Received }</p>
		</CloudCard>
		<CloudCard name="fetch" about={locales.CloudFetch}>
			<p id="fetched" class="text-sm" translate="no">{ s.Fetched }</p>
		</CloudCard>
		<CloudCard name="wrangler.jsonc" about={locales.CloudVars}>
			<p id="name" class="text-sm" translate="no">{ s.Name }</p>
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
