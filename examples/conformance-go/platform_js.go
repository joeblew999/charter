//go:build js && wasm

package main

import (
	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/fetch"

	"github.com/joeblew999/charter/d1"
	"github.com/joeblew999/charter/examples/conformance-go/api"
	"github.com/joeblew999/charter/hub"
)

// env binds the API to the Worker's bindings (charter.toml): its settings, DB (D1), HUB (the
// hub Durable Object, build/hub.mjs); webhooks go out with the Worker's fetch.
func env() api.Env {
	return api.Env{
		Var: cloudflare.Getenv,
		Store: func() (api.Store, error) {
			db, err := d1.Open("DB")
			if err != nil {
				return nil, err
			}
			return api.D1Store{DB: db}, nil
		},
		Hub:  func() (api.Hub, error) { return hub.DurableObject[api.Entry]("HUB", "notes") },
		HTTP: fetch.NewClient().HTTPClient(fetch.RedirectModeFollow),
	}
}
