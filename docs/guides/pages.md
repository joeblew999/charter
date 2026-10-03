---
title: Server-rendered pages
nav_order: 6
parent: Guides
---
# Server-rendered pages: gsx with htmx 4 or Datastar

`charter new -empty -ui htmx` makes a project whose one Worker serves HTML pages beside the API, its specs and MCP. The pages are written in [gsx](https://github.com/gsxhq/gsx) and made live with [htmx 4](https://four.htmx.org), in Go, built with TinyGo like the rest. A message posted on one page shows on every open page at once, as HTML over SSE. `-ui datastar` makes the same project with [Datastar](https://data-star.dev) in place of htmx.

```sh
mise x go node github:joeblew999/charter -- charter new -empty -ui htmx -name my-site -module github.com/you/my-site -subdomain you
cd my-site && mise install && mise run setup && mise run check
mise run run          # http://localhost:5179/ : open it in two windows, post in one
```

The projects are `examples/start-htmx/` and `examples/start-datastar/` (port 5180) in this repo.

## What is where

| Path | What it is |
|---|---|
| `pages/*.gsx` | The pages and fragments: `Layout`, `Home`, `Form`, `Item` |
| `pages/*.x.go` | Generated from the `.gsx` by `mise run ui:gen`; never edit. `mise run check` fails if they are stale (`ui:check`) |
| `pages/pages.go` | The routes: `GET /`, `POST /messages`, `GET /static/...`. Every other path goes to the API |
| `pages/stream.go` | `GET /messages/stream`: new messages as HTML, over SSE, by [go/fragments](../reference/packages.md) |
| `pages/static/` | htmx 4.0.0 and its SSE extension, as the npm package `htmx.org` has them, or Datastar 1.0.4's `bundles/datastar.js`, as its release tag has it; and `site.css`. The Worker serves them, the versioned ones as immutable |
| `api/` | The contract: `GET /api/hello`, `GET` and `POST /api/messages`. The pages call its `Env.Post`, `Env.Recent` and `Env.Feed` in the process |
| `gsx.toml` | htmx only: gsx's settings, which htmx attributes hold URLs |

gsx is pinned once, in `go.mod` (its `tool` line), for both the generator and the runtime the pages import.

## How a message reaches every page

1. The form posts with `hx-post="/messages"`. The handler calls `Env.Post`, as `POST /api/messages` does: D1 stores the message, and the hub Durable Object is told.
2. The answer is the empty form. A refused message gets a 422 with the form and the reason, which htmx 4 swaps in too.
3. Every page's list has `hx-sse:connect="/messages/stream?after=<newest shown>"`. The stream follows D1 and the hub with `follow.Follow` ([the rules](streaming.md#the-rules)): one SSE event per message, `id:` its id, `data:` its `Item` rendered by gsx. htmx puts it at the top of the list (`hx-swap="afterbegin"`).
4. A stream ends after five minutes, or at a deploy. htmx connects again with `Last-Event-ID`, and the stream goes on from there: nothing is lost or shown twice.

The page that posted gets its message the same way as every other page.

## With Datastar

The server is the same; the attributes and the events differ:

| | htmx 4 | Datastar |
|---|---|---|
| The form | `hx-post="/messages"` | ``data-on:submit=js`@post('/messages', {contentType: 'form'})` `` |
| Its answer | The form; a refused message: 422 | The form with the header `Datastar-Mode: replace`; a refused message: 200 too, since Datastar patches nothing from another status |
| The list | `hx-sse:connect`, `hx-swap="afterbegin"` | `data-init="@get('/messages/stream?after=N', {retry: 'always'})"` |
| An event | `data:` the `Item` | `event: datastar-patch-elements`, `data: selector #messages`, `data: mode prepend`, `data: elements` the `Item` |

`retry: 'always'` makes Datastar connect again when a stream ends; it sends `Last-Event-ID` as htmx does. The `data-init` value is built in Go (`follows` in `home.gsx`): gsx does not take a hole in a Datastar expression.

## Add a page

1. Write a component in a `.gsx` file in `pages/`, inside `Layout`.
2. Add a case to the switch in `Handler` (`pages/pages.go`) that renders it with `render`. Not `ServeMux` patterns such as `GET /about`: TinyGo's `net/http` matches no methods in them.
3. Run `mise run ui:gen`, add a check to `test/live-test.mjs`, then `mise run check`.

To push another fragment, write a function like `htmxEvent` or `datastarEvent` that renders it, and pass it to `serveStream`.

## Limits

The pages and `POST /messages` are public, as is the API: a bearer token ([go/auth](../reference/packages.md)) does not fit a browser form, and the project has no sign-in.
