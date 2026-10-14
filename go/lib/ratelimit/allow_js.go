//go:build js && wasm

package ratelimit

import (
	"context"
	"errors"
	"fmt"

	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/charter/go/internal/promise"
)

// errNoBinding is a Worker without the limit's binding: the call goes through, and is logged once.
var errNoBinding = errors.New("the Worker has no such Rate Limiting binding")

// Allow asks the Worker's Rate Limiting binding whether key may call once more (no network round
// trip: the counter is local to the Cloudflare location).
func Allow(_ context.Context, limit Limit, key string) (bool, error) {
	b := cloudflare.GetBinding(limit.Binding)
	if b.IsUndefined() || b.IsNull() {
		return true, errNoBinding
	}
	res, err := promise.Await(b.Call("limit", map[string]any{"key": key}))
	if err != nil {
		return true, fmt.Errorf("limit: %w", err)
	}
	return res.Get("success").Bool(), nil
}
