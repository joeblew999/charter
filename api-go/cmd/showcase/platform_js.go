//go:build js && wasm

package main

import (
	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/fetch"

	"github.com/joeblew999/orpc-api/api-go/showcase"
)

// env reads the settings from the Worker's variables and secrets (cloudflare.config.ts), and sends
// webhooks with the Worker's fetch.
func env() showcase.Env {
	return showcase.Env{
		Var:  cloudflare.Getenv,
		HTTP: fetch.NewClient().HTTPClient(fetch.RedirectModeFollow),
	}
}
