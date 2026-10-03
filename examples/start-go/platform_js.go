//go:build js && wasm

package main

import (
	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/charter/examples/start-go/api"
)

// env binds the API to the Worker's bindings (cloudflare.config.ts): APP_NAME. The D1 database is
// bound as DB too: d1.Open("DB") (github.com/joeblew999/charter/go/d1) opens it per request.
func env() api.Env {
	return api.Env{Var: cloudflare.Getenv}
}
