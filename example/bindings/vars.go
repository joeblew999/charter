//go:build js && wasm

package bindings

import "github.com/syumai/workers-go/cloudflare"

// A variable of the Worker (wrangler.jsonc's vars, .dev.vars locally).

func variable() string { return cloudflare.Getenv("APP_NAME") }
