//go:build js && wasm

package cloud

import (
	"fmt"
	"net/http"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/fetch"

	"github.com/joeblew999/charter/internal/promise"
)

// OnWorkers reports whether the code runs as a Worker.
const OnWorkers = true

// Var is the Worker's variable or secret called name, "" without one: the function an api.Auth reads its settings
// with (api.NewAuth(cloud.Var, …)). An app reads one of its own by its value in env: env.APP_NAME.Value().
func Var(name string) string { return cloudflare.Getenv(name) }

// allow asks the Rate Limiting binding called name whether key may make one more call (RateLimit.Allow).
func allow(name, key string) (bool, error) {
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

// HTTPClient is a client for the requests the Worker makes: they go out through the Worker's fetch (a Worker has no
// sockets, so net/http's own transport can't), following redirects. Each call returns a new client, the caller's to
// set (its Timeout).
func HTTPClient() *http.Client { return fetch.NewClient().HTTPClient(fetch.RedirectModeFollow) }
