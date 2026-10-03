//go:build !(js && wasm)

package ratelimit

import "context"

// memory is the process's counters on the host, where there are no bindings.
var memory = &Memory{}

// Allow counts in memory, as the Worker's binding would (go run, go test).
func Allow(ctx context.Context, limit Limit, key string) (bool, error) {
	return memory.Allow(ctx, limit, key)
}
