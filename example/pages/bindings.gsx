package pages

import (
	"strconv"

	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/example/bindings"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/intl"
)

// Cloud is the page at /bindings: one card per thing Cloudflare gives the Worker, each used through workers-go's own
// package, as simply as it can be (package bindings, a file each). The forms are plain posts, answered with a redirect back here.
component (p Bindings) Page(s bindings.State) {
	<Layout title={locales.BindingsTitle} description={locales.BindingsDescription}>
		<h1 class="text-2xl font-semibold">{ locales.BindingsHeading }</h1>
		<p class="text-muted-foreground">{ locales.BindingsIntro }</p>
		<a href={Home{} |> url} class="text-sm underline underline-offset-4">{ locales.BindingsBack }</a>
		{ if !s.OnWorkers {
			<p class="rounded-md border p-3 text-sm">{ locales.BindingsOffWorkers }</p>
		} }
		<BindingsCard name="KV" about={locales.BindingsKv}>
			<p id="visits" data-value={strconv.Itoa(s.Visits)} class="text-3xl font-semibold tabular-nums">
				{ s.Visits |> number }
			</p>
		</BindingsCard>
		<BindingsCard name="R2" about={locales.BindingsR2}>
			<form
				method="post"
				action={BindingsAction{} |> url(map[string]any{"action": "note"})}
				class="flex flex-wrap gap-2"
			>
				<ui.Input
					name="note"
					class="flex-1"
					value={s.Note}
					dir="auto"
					maxlength="200"
					aria-label={locales.BindingsNote}
				/>
				<ui.Button type="submit" variant="secondary">{ locales.BindingsSave }</ui.Button>
			</form>
		</BindingsCard>
		<BindingsCard name="Queues" about={locales.BindingsQueues}>
			<form
				method="post"
				action={BindingsAction{} |> url(map[string]any{"action": "queue"})}
				class="flex flex-wrap gap-2"
			>
				<ui.Input
					name="message"
					class="flex-1"
					required
					dir="auto"
					maxlength="80"
					aria-label={locales.BindingsMessage}
				/>
				<ui.Button type="submit" variant="secondary">{ locales.BindingsSend }</ui.Button>
			</form>
			<ul id="queued" class="flex flex-col gap-1 text-sm">
				{ for _, m := range s.Queued {
					<li dir="auto" class="rounded-md border px-2 py-1">{ m }</li>
				} }
			</ul>
		</BindingsCard>
		<BindingsCard name="D1" about={locales.BindingsD1}>
			<p id="rows" class="text-3xl font-semibold tabular-nums">{ s.Rows }</p>
		</BindingsCard>
		<BindingsCard name="Cache API" about={locales.BindingsCache}>
			<p id="cached" class="text-sm" translate="no">{ s.Cached }</p>
		</BindingsCard>
		<BindingsCard name="Durable Objects" about={locales.BindingsObject}>
			<p id="object" class="text-sm" translate="no">{ s.Object }</p>
		</BindingsCard>
		<BindingsCard name="TCP sockets" about={locales.BindingsSocket}>
			<p id="socket" class="text-sm" translate="no">{ s.Socket }</p>
		</BindingsCard>
		<BindingsCard name="waitUntil" about={locales.BindingsAfter}>
			<p id="after" data-value={s.After} class="text-sm">
				{ if !s.AfterAt().IsZero() {
					<span>{ s.AfterAt() |> date("dateStyle", "medium", "timeStyle", "medium") }</span>
				} }
			</p>
		</BindingsCard>
		<BindingsCard name="request.cf" about={locales.BindingsRequest}>
			<p id="request" class="text-sm" translate="no">{ s.Request }</p>
		</BindingsCard>
		<BindingsCard name="Cron Triggers" about={locales.BindingsCron}>
			<p id="cron" data-value={s.Cron} class="text-sm">
				<span translate="no">{ s.CronSchedule() }</span>
				{ if !s.CronAt().IsZero() {
					<span>{ s.CronAt() |> date("dateStyle", "medium", "timeStyle", "medium") }</span>
				} }
			</p>
		</BindingsCard>
		<BindingsCard name="Email" about={locales.BindingsEmail}>
			<form
				method="post"
				action={BindingsAction{} |> url(map[string]any{"action": "email"})}
				class="flex flex-wrap gap-2"
			>
				<ui.Input
					name="to"
					type="email"
					class="flex-1"
					placeholder="you@example.com"
					aria-label={locales.BindingsEmailTo}
				/>
				<ui.Button type="submit" variant="secondary">{ locales.BindingsSend }</ui.Button>
			</form>
			<p id="sent" data-value={s.Sent} class="text-sm">
				{ if !s.SentAt().IsZero() {
					<span>{ s.SentAt() |> date("dateStyle", "medium", "timeStyle", "medium") }</span>
				} }
				<span translate="no">{ s.SentTo() }</span>
			</p>
			<p id="received" class="text-sm text-muted-foreground" translate="no">{ s.Received }</p>
		</BindingsCard>
		<BindingsCard name="fetch" about={locales.BindingsFetch}>
			<p id="fetched" class="text-sm" translate="no">{ s.Fetched }</p>
		</BindingsCard>
		<BindingsCard name="wrangler.jsonc" about={locales.BindingsVars}>
			<p id="name" class="text-sm" translate="no">{ s.Name }</p>
		</BindingsCard>
	</Layout>
}

// BindingsCard is one binding: its name as Cloudflare writes it, what the card does with it, and the result.
component BindingsCard(name string, about intl.Msg, children gsx.Node) {
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
