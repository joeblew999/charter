//go:build !(js && wasm)

package main

import (
	"net/http"
	"os"

	"github.com/joeblew999/orpc-api/api/go/showcase"
)

// env reads the settings from the environment, and sends webhooks with net/http.
func env() showcase.Env {
	return showcase.Env{Var: os.Getenv, HTTP: http.DefaultClient}
}
