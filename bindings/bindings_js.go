//go:build js && wasm

package bindings

import (
	"fmt"
	"net/http"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/charter/internal/promise"
)

// OnWorkers reports whether the code runs as a Worker.
const OnWorkers = true

// Var is the Worker's variable called name (a text binding), "" without one.
func Var(name string) string { return cloudflare.Getenv(name) }

// Allow asks the Rate Limiting binding called name whether key may make one more call. Its counter is local to the
// Cloudflare location, so it answers without a network round trip.
func Allow(name, key string) (bool, error) {
	b := cloudflare.GetBinding(name)
	if !b.Truthy() {
		return true, fmt.Errorf("%w: %s", ErrNoBinding, name)
	}
	res, err := promise.Await(b.Call("limit", js.ValueOf(map[string]any{"key": key})))
	if err != nil {
		return true, fmt.Errorf("bindings: %s: %w", name, err)
	}
	return res.Get("success").Bool(), nil
}

// StaticAssets is a 404: on Cloudflare the Static Assets answer for their files before the Worker runs, so a request
// that reaches it is for none of them.
func StaticAssets(dir string) http.Handler { return http.NotFoundHandler() }
