//go:build !(js && wasm)

package main

import (
	"os"

	"github.com/joeblew999/charter/examples/start-go/api"
)

// env is the process's environment.
func env() api.Env {
	return api.Env{
		Var: func(name string) string {
			if name == "APP_NAME" && os.Getenv(name) == "" {
				return "charter-start-go (go run)"
			}
			return os.Getenv(name)
		},
	}
}
