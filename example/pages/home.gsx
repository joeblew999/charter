package pages

import (
	"strconv"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/live"

	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/pages/screens"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/example/ui/icon"
)

// Page is the home page: the form, and the newest notes, newest first. The list keeps itself current: its stream
// (hx-sse:connect, the hub's) says when the notes have changed, and it then asks for itself again (hx-get, with the
// list as its target) and morphs what comes into place. So a note added, one taken away, and several at once all
// reach every open page, and a row a reader has opened stays open.
component (p Home) Page(props HomeProps) {
	<Layout title={locales.SiteTitle} description={locales.SiteDescription}>
		// A line for everyone, when whoever may moderate has set one.
		{ if props.Notice.Text != "" {
			<ui.Alert id="notice">
				<ui.AlertTitle dir="auto">{ props.Notice.Text }</ui.AlertTitle>
			</ui.Alert>
		} }
		<ui.Card>
			<ui.CardHeader>
				<ui.CardTitle>
					<h1>{ locales.HomeHeading }</h1>
				</ui.CardTitle>
				<ui.CardDescription>{ locales.HomeIntro }</ui.CardDescription>
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
				// The stream is this element's; the list inside it asks for itself again when the stream says so.
				<div { api.Notes.Stream()... }>
					<ul
						id={Home.List |> id}
						class="flex flex-col gap-2"
						hx-get={Home{} |> url}
						hx-target={Home.List |> target}
						{ live.Follows()... }
					>
						<p.List notes={props.Notes} reader={props.Caller}/>
					</ul>
				</div>
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

// List is the notes of the list, each with what the reader may do with it. A row of it that the page has already
// is kept as the page has it (Item's kept): the list is asked for again whenever the notes change, and a row the
// reader has opened meanwhile, which the Worker knows nothing of, must not be shut by that.
component (p Home) List(notes []api.Note, reader charter.Caller) {
	{ for _, note := range notes {
		<p.Item note={note} open={false} removable={Removable(api.MayRemove(reader, note))} kept={true}/>
	} }
}

// Item is one note: in the list, what the stream sends of each new one, and what its own button asks for again. Its
// id is the component's, keyed by the note, so a button's target is this row and no other, and Home's Props knows
// which note is wanted (structpages.KeyOf). It says who wrote it; open, also when, in full. Whoever may take it
// away (its writer, a moderator) has the button for that.
//
// kept says the row is one of a list that is being asked for again: htmx then leaves the row the page has as it is
// (hx-preserve), whatever this one says, and puts in only the rows that are new. A row asked for by itself, by its
// own button, is not kept: it is the answer.
component (p Home) Item(note api.Note, open Open, removable Removable, kept Kept) {
	<li
		id={Home.Item |> key(strconv.FormatInt(note.ID, 10))}
		class="flex flex-wrap items-center gap-2 rounded-md border px-3 py-2"
		{ if kept {
			hx-preserve
		} }
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
