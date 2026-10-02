//go:build js && wasm

package main

import (
	"database/sql"
	"fmt"

	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/d1"

	"github.com/joeblew999/orpc-api/api/go/api"
	"github.com/joeblew999/orpc-api/api/go/hub"
)

// env binds the API to the Worker's bindings (cloudflare.config.ts): APP_NAME, DB (D1), HUB (the hub Durable Object, worker/hub.mjs).
func env() api.Env {
	return api.Env{
		Var: cloudflare.Getenv,
		Store: func() (api.Store, error) {
			connector, err := d1.OpenConnector("DB")
			if err != nil {
				return nil, fmt.Errorf("DB is not bound (cloudflare.config.ts): %w", err)
			}
			return api.SQLStore{DB: sql.OpenDB(connector)}, nil
		},
		Hub: func() (api.Hub, error) { return hub.DurableObject[api.Note]("HUB", "notes") },
	}
}
