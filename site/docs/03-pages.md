# Pages

Pages are a [structpages](https://jackielii.github.io/structpages/) tree, rendered with [gsx](https://gsxhq.github.io) and built from [gsxui](https://ui.gsxhq.dev) components. The tree is the one source of routes, links, element ids and the sitemap.

## The tree

Every route is a field, with its route in the field's tag.

```go
type Pages struct {
	home     Home       `route:"GET /{$} Home"`
	bindings Bindings   `route:"GET /bindings Bindings"`
	docs     screens.Docs `route:"/docs Docs"`

	post   PostNote    `route:"POST /notes PostNote"`
	stream NotesStream `route:"GET /notes/stream NotesStream"`
}
```

## A page declares what it needs

A page is a type with a `Page` component, and `Props` for its data. `Props` names what it needs as parameters, and structpages hands each over by its type.

```go
type Home struct{}

func (Home) Props(r *http.Request, store api.Store) (HomeProps, error) {
	notes, err := store.Before(r.Context(), math.MaxInt64, shown)
	…
}
```

The store comes from `main.go`: `RequestArgs: []any{api.OpenStore}`, a function from the request to the value. No page opens a database or reads a binding itself, so a test hands a page a store of its own.

## A region that changes by itself

A part of a page that htmx replaces is a component of that page. Its id and its target come from the tree, so the three places that must agree cannot drift apart.

```gsx
component (p Home) Form(body string, refused bool) {
	<form
		id={Home.Form |> id}
		hx-post={PostNote{} |> url}
		hx-target={Home.Form |> target}
		hx-swap="outerHTML"
	>
		…
	</form>
}
```

One of many (a row, a card) has a key: `id={Home.Item |> key(note.ID)}`.

## What a page calls

A type with `ServeHTTP` that returns an error. It answers with a component, and never writes an error or a redirect itself.

```go
func (PostNote) ServeHTTP(w http.ResponseWriter, r *http.Request, store api.Store) error {
	…
	if _, err := api.Post(r.Context(), store, body); err != nil {
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
