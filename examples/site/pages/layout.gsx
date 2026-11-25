package pages

import (
	"github.com/gsxhq/gsx"
	"github.com/gsxhq/vite"
)

// Layout is the document around every page. htmx 4, its SSE extension and the stylesheet are web/main.js, as Vite
// built it: no other site is asked for anything.
component Layout(title string, children gsx.Node) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1"/>
			<title>{ title }</title>
			{{ b := vite.FromContext(ctx).Entry("web/main.js") }}
			{ for _, href := range b.CSS {
				<link rel="stylesheet" href={href}/>
			} }
			{ for _, src := range b.JS {
				<script type="module" src={src}></script>
			} }
		</head>
		<body>
			<main>{ children }</main>
		</body>
	</html>
}
