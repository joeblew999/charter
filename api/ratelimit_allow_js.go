//go:build js && wasm

package api

import "context"

// allow asks the Worker's Rate Limiting binding whether key may call once more (cloud.RateLimit.Allow: no network
// round trip, the counter is local to the Cloudflare location). A Worker without the binding lets the call
// through with cloud.ErrNoBinding, which is logged once.
func allow(_ context.Context, limit Limit, key string) (bool, error) {
	return limit.Binding.Allow(key)
}
