package pages

import (
	"strconv"

	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui"
)

// Page is the home page: the form, and the newest notes, newest first. The list follows the feed from the newest
// note it shows: htmx 4's SSE extension (hx-sse:connect) puts the HTML of each event at the top of the list
// (hx-swap), and when it connects again it sends the id of the last event it had, so no note is lost or shown twice.
component (p Home) Page(props HomeProps) {
	<Layout title={locales.SiteTitle} description={locales.SiteDescription}>
		<ui.Card>
			<ui.CardHeader>
				<ui.CardTitle>
					<h1>{ locales.HomeHeading }</h1>
				</ui.CardTitle>
				<ui.CardDescription>{ locales.HomeIntro }</ui.CardDescription>
				<a href={Bindings{} |> url} class="text-sm underline underline-offset-4">{ locales.BindingsLink }</a>
			</ui.CardHeader>
			<ui.CardContent class="flex flex-col gap-4">
				<Form body="" refused={false}/>
				<ul
					id="notes"
					class="flex flex-col gap-2"
					hx-sse:connect=f`@{NotesStream{} |> action}?after=@{strconv.FormatInt(props.After, 10)}`
					hx-swap="afterbegin"
				>
					{ for _, note := range props.Notes {
						<Item note={note}/>
					} }
				</ul>
			</ui.CardContent>
		</ui.Card>
	</Layout>
}

// Form is the form, and what PostNote answers: htmx puts it in place of the one sent (outerHTML), empty, or with
// what was typed, marked as refused.
component Form(body string, refused bool) {
	<form id="post" class="flex flex-wrap gap-2" hx-post={PostNote{} |> action} hx-swap="outerHTML">
		<ui.Input
			name="body"
			class="flex-1"
			value={body}
			maxlength={strconv.Itoa(maxBody)}
			required
			autofocus
			autocomplete="off"
			dir="auto"
			placeholder={locales.HomePlaceholder}
			aria-label={locales.HomeLabel}
			{ if refused {
				aria-invalid="true"
			} }
		/>
		<ui.Button type="submit">{ locales.HomePost }</ui.Button>
	</form>
}

// Item is one note: in the list, and what the stream sends of each new one.
component Item(note api.Note) {
	<li id={"note-" + strconv.FormatInt(note.ID, 10)} class="rounded-md border px-3 py-2">
		<span dir="auto">{ note.Body }</span>
		// How long ago, in the reader's language; the browser's half of intl keeps it current.
		<span class="ms-2 text-xs text-muted-foreground">{ note.Created() |> since }</span>
	</li>
}
