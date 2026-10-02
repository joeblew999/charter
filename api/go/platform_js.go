//go:build js && wasm

package main

import (
	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/orpc-api/api/go/api"
	"github.com/joeblew999/orpc-api/api/go/d1"
	"github.com/joeblew999/orpc-api/api/go/hub"
)

// env binds the API to the Worker's bindings (cloudflare.config.ts): APP_NAME, DB (D1), HUB (the hub Durable Object, worker/hub.mjs).
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
		Hub: func() (api.Hub, error) { return hub.DurableObject[api.Note]("HUB", "notes") },
	}
}
