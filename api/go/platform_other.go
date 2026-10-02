//go:build !(js && wasm)

package main

import (
	"os"

	"github.com/joeblew999/orpc-api/api/go/api"
)

var memory = &api.MemStore{}

// env is one in-memory store and hub for the process.
func env() api.Env {
	return api.Env{
		Var: func(name string) string {
			if name == "APP_NAME" && os.Getenv(name) == "" {
				return "orpc-api-go (go run)"
			}
			return os.Getenv(name)
		},
		Store: func() (api.Store, error) { return memory, nil },
		Hub:   func() (api.Hub, error) { return memory, nil },
	}
}
