# Pages

Pages are a [structpages](https://jackielii.github.io/structpages/) tree, rendered with [gsx](https://gsxhq.github.io) and built from [gsxui](https://ui.gsxhq.dev) components. The tree is the one source of routes, links, element ids and the sitemap.

## The tree

Every route is a field, with its route in the field's tag.

```go file=../example/pages/pages.go#Pages
// Pages is the tree.
type Pages struct {
	home  Home         `route:"GET /{$} Home"`
	built Built        `route:"/built Built"`
	docs  screens.Docs `route:"/docs Docs"`

	// The app's own people: signing in with a passkey, and what is someone's own (charter's sign-in screen).
	signIn  screens.SignIn  `route:"/sign-in SignIn"`
	account screens.Account `route:"/account Account"`

	// What an admin sees of the system (charter's admin screen): the app at a glance, its bindings, its database.
	admin screens.Admin `route:"/admin Admin"`

	// The time zone and clock dialog's form, and where it is sent.
	preferences screens.Preferences `route:"/preferences Preferences"`

	// What the pages call: routes, not pages.
	post   PostNote   `route:"POST /notes PostNote"`
	remove RemoveNote `route:"POST /notes/{note}/remove RemoveNote"`
}
```

## A page declares what it needs

A page is a type with a `Page` component, and `Props` for its data. `Props` names what it needs as parameters, and structpages hands each over by its type.

```go file=../example/pages/pages.go#Home.Props
// Props loads what the request wants, as structpages has it. A row's button asks for that row alone (its target is
// Home's Item, with the note's id as its key): then one note is read, and structpages renders Item with it. The
// list asks for itself when its stream says the notes changed (its target is Home's List): then the newest notes
// are read, and structpages renders List with them, for this reader. Anything else is the page. Each component
// takes its share of the results by their types.
func (p Home) Props(r *http.Request, store api.Store, caller charter.Caller, target structpages.RenderTarget) (HomeProps, []api.Note, charter.Caller, api.Note, Open, Removable, error) {
	if target.Is(p.Item) {
		id, err := strconv.ParseInt(structpages.KeyOf(target), 10, 64)
		if err != nil {
			return HomeProps{}, nil, caller, api.Note{}, false, false, app.NotFound("no such note")
		}
		note, err := store.Get(r.Context(), id)
		if err != nil {
			return HomeProps{}, nil, caller, api.Note{}, false, false, app.NotFound("no such note")
		}
		return HomeProps{}, nil, caller, note, r.URL.Query().Get("open") != "", Removable(api.MayRemove(caller, note)), nil
	}
	notes, err := store.Before(r.Context(), math.MaxInt64, shown)
	if err != nil {
		return HomeProps{}, nil, caller, api.Note{}, false, false, err
	}
	if target.Is(p.List) {
		return HomeProps{}, notes, caller, api.Note{}, false, false, nil
	}
	props := HomeProps{Notes: notes, MayWrite: caller.May(PostNote{}.Needs()...), Caller: caller}
	// One read of KV, for every reader: what it is for.
	if props.Notice, err = built.NoticeNow(r.Context()); err != nil {
		return HomeProps{}, nil, caller, api.Note{}, false, false, err
	}
	return props, notes, caller, api.Note{}, false, false, nil
}
```

The store comes from `main.go`: `RequestArgs: []any{api.OpenStore}`, a function from the request to the value. No page opens a database or reads a binding itself, so a test hands a page a store of its own. The target is what the request wants: the whole page, or one part of it.

## A region that changes by itself

A part of a page that htmx replaces is a component of that page. Its id and its target come from the tree, so the three places that must agree cannot drift apart.

```gsx file=../example/pages/home.gsx#Home.Form
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
```

One of many (a row, a card) has a key: `id={Home.Item |> key(id)}`, and its own button targets `{Home.Item |> keytarget(id)}`. `Props` then asks whether that row is what is wanted, reads which from `structpages.KeyOf`, and loads that note alone, as above.

## The page being shown

`structpages.Within(ctx, Bindings{})` says whether the page being shown is that page or one under it: how the example's header marks its links.

## A fragment opened as a page

A route with `Props` and no `Page` is a fragment: markup for part of a page, such as a dialog's form. Its address opened in a browser would be that markup alone, so charter sends such a request to the page the fragment is part of (structpages' own convention), before its `Props` runs. Only an htmx request for part of a page gets the fragment.

## What a page calls

A type with `ServeHTTP` that returns an error. It answers with a component, and never writes an error or a redirect itself.

```go file=../example/pages/pages.go#PostNote.ServeHTTP
func (PostNote) ServeHTTP(w http.ResponseWriter, r *http.Request, store api.Store, caller charter.Caller) error {
	body := strings.TrimSpace(r.PostFormValue("body"))
	if body == "" || len(body) > maxBody || strings.Contains(body, api.END) {
		// htmx 4 swaps a 422 too: the form comes back with what was typed.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return structpages.RenderComponent(Home{}.Form(body, true))
	}
	if _, err := api.Post(r.Context(), store, caller, body); err != nil {
		return err
	}
	return structpages.RenderComponent(Home{}.Form("", false))
}
```

| To say | Return |
|---|---|
| It is not there | `app.NotFound("no such note")` |
| Another status | `app.Status(http.StatusForbidden, "…")` |
| Go somewhere else | `app.Redirect{To: url}`: `HX-Location` for htmx, a 303 otherwise |
| It broke | the error: a 500, logged, with nothing of it shown |

## Links

A link or a form's target comes from the tree, never from a path written as a string.

```gsx
<a href={Bindings{} |> url}>…</a>
<form hx-post={PostNote{} |> url}>…</form>
<a href={Note{} |> url(map[string]any{"id": note.ID})}>…</a>
```

`url` is structpages' own `URLFor`. charter mounts the tree once for each language, each told its prefix, so every address it gives is in the reader's language by itself: a page, a form's target, a stream. structpages' linter runs inside `charter check`, for the machine and for the Worker's build, and fails on a link to a page that is not there, a missing parameter, or a path written as a string.

## Which pages search engines get

The tree's GET pages without parameters are in the sitemap, in every language. The pages of the docs are too.

## The head

```gsx
<html { app.HTML(ctx)... }>
	<head>
		<app.Head title={title} description={description}/>
	</head>
```

`app.Head` writes the title and description, the canonical address, every language's alternate, Open Graph, and the scripts Vite built.
