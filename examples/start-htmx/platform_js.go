//go:build js && wasm

package main

import (
	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/charter/examples/start-htmx/api"
	"github.com/joeblew999/charter/go/d1"
	"github.com/joeblew999/charter/go/hub"
)

// env binds the API and the pages to the Worker's bindings (cloudflare.config.ts): APP_NAME, DB
// (D1, the log), HUB (the hub Durable Object, build/hub.mjs: the wake-up of every open page).
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
		Hub: func() (api.Hub, error) { return hub.DurableObject[api.Message]("HUB", "messages") },
	}
}
