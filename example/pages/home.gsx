package pages

import (
	"strconv"

	charter "github.com/joeblew999/charter/api"

	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/pages/screens"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/example/ui/icon"
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
				// Anyone reads; writing is for whoever is signed in.
				{ if props.MayWrite {
					<p.Form body="" refused={false}/>
				} else {
					<p id="sign-in-to-write" class="text-sm text-muted-foreground">
						<ui.Button variant="link" class="px-0" href={screens.SignInPage{} |> url}>
							{ locales.HomeSignInToWrite }
						</ui.Button>
					</p>
				} }
				<ul
					id={Home.List |> id}
					class="flex flex-col gap-2"
					hx-sse:connect=f`@{NotesStream{} |> url}?after=@{strconv.FormatInt(props.After, 10)}`
					hx-swap="afterbegin"
				>
					<p.List notes={props.Notes} reader={props.Caller}/>
				</ul>
			</ui.CardContent>
		</ui.Card>
	</Layout>
}

// Form is the form, a region of the page that changes by itself: PostNote answers with it, and htmx puts it in place
// of the one sent (its target is this component, by the tree), empty, or with what was typed, marked as refused.
component (p Home) Form(body string, refused bool) {
	<form
		id={Home.Form |> id}
		class="flex flex-wrap gap-2"
		hx-post={PostNote{} |> url}
		hx-target={Home.Form |> target}
		hx-swap="outerHTML"
	>
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

// List is the notes of the list, each with what the reader may do with it.
component (p Home) List(notes []api.Note, reader charter.Caller) {
	{ for _, note := range notes {
		<p.Item note={note} open={false} removable={Removable(api.MayRemove(reader, note))}/>
	} }
}

// Item is one note: in the list, what the stream sends of each new one, and what its own button asks for again. Its
// id is the component's, keyed by the note, so a button's target is this row and no other, and Home's Props knows
// which note is wanted (structpages.KeyOf). It says who wrote it; open, also when, in full. Whoever may take it
// away (its writer, a moderator) has the button for that.
component (p Home) Item(note api.Note, open Open, removable Removable) {
	<li
		id={Home.Item |> key(strconv.FormatInt(note.ID, 10))}
		class="flex flex-wrap items-center gap-2 rounded-md border px-3 py-2"
	>
		<span class="flex flex-1 flex-col">
			<span dir="auto">{ note.Body }</span>
			{ if note.Author != "" {
				<span class="text-xs text-muted-foreground" dir="auto" translate="no">{ note.Author }</span>
			} }
		</span>
		// How long ago, in the reader's language; the browser's half of intl keeps it current.
		<span class="text-xs text-muted-foreground">{ note.Created() |> since }</span>
		<ui.Button
			variant="ghost"
			size="icon-sm"
			type="button"
			hx-get=f`@{Home{} |> url}@{itemQuery(open)}`
			hx-target={Home.Item |> keytarget(strconv.FormatInt(note.ID, 10))}
			hx-swap="outerHTML"
			aria-expanded={strconv.FormatBool(bool(open))}
			aria-label={locales.HomeWhen}
			title={locales.HomeWhen}
		>
			<icon.Clock/>
		</ui.Button>
		{ if removable {
			<ui.Button
				variant="ghost"
				size="icon-sm"
				type="button"
				data-remove
				hx-post={RemoveNote{} |> url(map[string]any{"note": note.ID})}
				hx-target={Home.Item |> keytarget(strconv.FormatInt(note.ID, 10))}
				hx-swap="delete"
				aria-label={locales.HomeRemove}
				title={locales.HomeRemove}
			>
				<icon.Trash2/>
			</ui.Button>
		} }
		{ if open {
			<p class="w-full text-sm text-muted-foreground">
				{ note.Created() |> date("dateStyle", "full", "timeStyle", "long") }
			</p>
		} }
	</li>
}

// itemQuery is what a row's button asks for: the row open when it is closed, closed when it is open.
func itemQuery(open Open) string {
	if open {
		return ""
	}
	return "?open=1"
}
