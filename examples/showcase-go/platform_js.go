//go:build js && wasm

package main

import (
	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/fetch"

	"github.com/joeblew999/charter/examples/showcase-go/api"
)

// env reads the settings from the Worker's variables and secrets (cloudflare.config.ts), and sends
// webhooks with the Worker's fetch.
func env() api.Env {
	return api.Env{
		Var:  cloudflare.Getenv,
		HTTP: fetch.NewClient().HTTPClient(fetch.RedirectModeFollow),
	}
}
