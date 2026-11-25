// Package js is intl's JavaScript, which mise-intl assets copies into an app:
//
//	intl.mjs          Intl itself, the same module on the Worker and in the browser
//	intl-worker.mjs   what Go calls on a Worker (globalThis.goIntl), with the catalogs (intl-messages.mjs)
//	intl-browser.mjs  the browser half: redoes each value in the viewer's own zone, keeps "3 min ago" fresh
//
// and which mise-intl gen writes beside the catalogs for Intl outside a Worker, with intl.mjs and intl-worker.mjs:
//
//	intl-workerd.mjs    intl-worker.mjs's goIntl over HTTP, for intl/workerd
//	intl-workerd.capnp  workerd's config for it (mise-intl workerd serves it)
package js

import "embed"

//go:embed *.mjs *.capnp
var FS embed.FS

// Workerd are the files workerd serves outside a Worker, with the catalogs (intl-messages.mjs).
var Workerd = []string{"intl.mjs", "intl-worker.mjs", "intl-workerd.mjs", "intl-workerd.capnp"}
