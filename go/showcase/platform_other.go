//go:build !(js && wasm)

package main

import (
	"net/http"
	"os"

	"github.com/joeblew999/charter/conformance/showcase-go/api"
)

// env reads the settings from the environment, and sends webhooks with net/http.
func env() api.Env {
	return api.Env{Var: os.Getenv, HTTP: http.DefaultClient}
}
