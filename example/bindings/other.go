//go:build !(js && wasm)

package bindings

import (
	"errors"
	"net/http"
)

// Off Workers there are none of workers-go's bindings.

// Background does nothing off Workers.
func Background() {}

// Read is the empty state: the page says it needs a Worker.
func Read(*http.Request) (State, error) { return State{}, nil }

// Post refuses: the forms need a Worker.
func Post(*http.Request) error { return errors.New("the Cloudflare page's forms need a Worker") }
