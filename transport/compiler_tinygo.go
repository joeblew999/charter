//go:build js && wasm && tinygo

package transport

import "runtime"

// full says whether this runtime should serve no more requests. TinyGo's heap is fixed and its collector costs more
// than a new runtime does: the runtime goes once a collection ran, or when less is free than twice what a request
// took (what was allocated since the last look, or by the first request alone).
func full(stats *runtime.MemStats, mostRequest uint64) bool {
	return stats.NumGC > 0 || stats.HeapIdle < 2*mostRequest
}

// fromJS runs f for a call from JavaScript. TinyGo starts a goroutine for every such call, which may wait (for a
// promise, a timer) as any goroutine does: f runs in it.
func fromJS(f func()) { f() }
