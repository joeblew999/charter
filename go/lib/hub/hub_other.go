//go:build !(js && wasm)

package hub

import (
	"fmt"
	"sync"
)

// DurableObject is, off Workers (go test, an ordinary Go server), the in-memory hub called name: every DurableObject
// of the same binding and name in the process is the same Memory, so a publish reaches this process's subscribers.
func DurableObject[T any](binding, name string) (Hub[T], error) {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	key := binding + "\x00" + name
	if h, ok := objects.hubs[key]; ok {
		m, ok := h.(*Memory[T])
		if !ok {
			return nil, fmt.Errorf("hub %s/%s carries %T, not %T", binding, name, h, m)
		}
		return m, nil
	}
	m := &Memory[T]{}
	objects.hubs[key] = m
	return m, nil
}

var objects = struct {
	mu   sync.Mutex
	hubs map[string]any
}{hubs: map[string]any{}}
