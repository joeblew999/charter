package pages

import (
	"strconv"

	"github.com/joeblew999/charter/examples/site/api"
	"github.com/joeblew999/charter/examples/site/locales"
	"github.com/joeblew999/charter/examples/site/ui"
)

// Home is the page at /: the form, and the newest messages, newest first. The list follows the stream from the
// newest message it shows (after): htmx 4's SSE extension (hx-sse:connect) puts the HTML of each event at the top of
// the list (hx-swap), and when it connects again it sends the id of the last event it had, so no message is lost or
// shown twice.
component Home(messages []api.Message, after int64) {
	<Layout title={locales.SiteTitle} description={locales.SiteDescription}>
		<ui.Card>
			<ui.CardHeader>
				<ui.CardTitle>
					<h1>{ locales.HomeHeading }</h1>
				</ui.CardTitle>
				<ui.CardDescription>{ locales.HomeIntro }</ui.CardDescription>
			</ui.CardHeader>
			<ui.CardContent class="flex flex-col gap-4">
				<Form body="" problem=""/>
				<ul
					id="messages"
					class="flex flex-col gap-2"
					hx-sse:connect={"/messages/stream?after=" + strconv.FormatInt(after, 10)}
					hx-swap="afterbegin"
				>
					{ for _, message := range messages {
						<Item message={message}/>
					} }
				</ul>
			</ui.CardContent>
		</ui.Card>
	</Layout>
}

// Form is the form, and what POST /messages answers: htmx puts it in place of the one sent (outerHTML), empty, or
// with what was typed and why it was refused.
component Form(body string, problem string) {
	<form id="post" class="flex flex-wrap gap-2" hx-post="/messages" hx-swap="outerHTML">
		<ui.Input
			name="body"
			class="flex-1"
			value={body}
			maxlength={strconv.Itoa(api.MaxBody)}
			required
			autofocus
			autocomplete="off"
			dir="auto"
			placeholder={locales.HomePlaceholder |> text}
			aria-label={locales.HomeLabel |> text}
		/>
		<ui.Button type="submit">{ locales.HomePost }</ui.Button>
		{ if problem != "" {
			<p class="w-full text-sm text-destructive">{ problem }</p>
		} }
	</form>
}

// Item is one message: in the list, and what the stream sends of each new one.
component Item(message api.Message) {
	<li id={"message-" + strconv.FormatInt(message.ID, 10)} class="rounded-md border px-3 py-2">
		<span dir="auto">{ message.Body }</span>
	</li>
}
