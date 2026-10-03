package pages

import "github.com/gsxhq/gsx"

// Layout is the document around every page. Datastar comes from this Worker (/static/), as does the
// stylesheet: no other site is asked for anything.
component Layout(title string, children gsx.Node) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1"/>
			<title>{title}</title>
			<link rel="stylesheet" href="/static/site.css"/>
			<script type="module" src="/static/datastar-1.0.4.js"></script>
		</head>
		<body>
			<main>{children}</main>
		</body>
	</html>
}
