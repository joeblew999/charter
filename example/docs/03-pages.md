# Pages

Pages are a [structpages](https://jackielii.github.io/structpages/) tree, rendered with [gsx](https://gsxhq.github.io) and built from [gsxui](https://ui.gsxhq.dev) components. The tree is the one source of routes, links and the sitemap.

## The tree

Every route is a field, with its route in the field's tag.

```go
type Pages struct {
	home     Home     `route:"GET /{$} Home"`
	bindings Bindings `route:"GET /bindings Bindings"`
	docs     Docs     `route:"/docs Docs"`

	post   PostNote    `route:"POST /notes PostNote"`
	stream NotesStream `route:"GET /notes/stream NotesStream"`
}
```

A page is a type with a `Page` component, and `Props` for its data. What a page calls is a type with `ServeHTTP`.

```go
type Home struct{}

func (Home) Props(r *http.Request) (HomeProps, error) { … }
```

## Links

A link or a form's target comes from the tree, never from a path written as a string.

```gsx
<a href={Bindings{} |> url}>…</a>
<form hx-post={PostNote{} |> action}>…</form>
```

`url` is a page's address in the reader's language; `action` is a route that is no page. structpages' own linter runs inside `charter check` and fails on a hard-coded path.

## Which pages search engines get

The tree's GET pages without parameters are in the sitemap, in every language. A page whose route has a parameter lists its addresses with a `Paths` method, as these docs do.

## The head

```gsx
<html { app.HTML(ctx)... }>
	<head>
		<app.Head title={title} description={description}/>
	</head>
```

`app.Head` writes the title and description, the canonical address, every language's alternate, Open Graph, and the scripts Vite built.
