// Package intl is a gsx app's languages: the runtime's own Intl, written in markup with gsx's own mechanisms. Go
// decides nothing about a language; it routes the request to a locale (WithRequest) and passes values.
//
//   - Msg is a message from the app's catalogs (ICU MessageFormat), rendered by Translate (gsx.toml [renderers]):
//     { locales.NavHome }, { locales.BoardNotes(n) }. mise-intl gen writes them, so a missing key is a compile
//     error.
//   - Value is any Intl call, rendered by Render inside an element js/intl-browser.mjs redoes in the viewer's zone.
//     Markup writes it with the filters package: { t |> date("dateStyle", "full") }, { n |> number }.
//
// Both are formatted by js/intl.mjs, the same module in the Worker (where Go calls it through js/intl-worker.mjs) and
// in the browser. Outside a Worker the only Intl is workerd's: tests run under intltest. The catalogs, their
// translation and checks are mise-intl (cmd/mise-intl) and intl's mise tasks (mise/tasks.toml). README.md is the whole
// flow.
package intl
