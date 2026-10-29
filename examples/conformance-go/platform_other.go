//go:build !(js && wasm)

package main

import (
	"net/http"
	"os"

	"github.com/joeblew999/charter/examples/conformance-go/api"
)

var memory = api.NewMemStore()

// env reads the settings from the environment, keeps the notes in one in-memory store and hub for
// the process, and sends webhooks with net/http.
func env() api.Env {
	return api.Env{
		Var:   os.Getenv,
		Store: func() (api.Store, error) { return memory, nil },
		Hub:   func() (api.Hub, error) { return memory, nil },
		HTTP:  http.DefaultClient,
	}
}
