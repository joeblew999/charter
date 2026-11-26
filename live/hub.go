// Package live is a feed that browsers and API clients follow as it grows, with no item lost or sent twice:
//
//   - a Hub is the wake-up signal: publish an item and every subscriber gets it. On Cloudflare it is a Durable
//     Object (DurableObject), natively and in tests it is Memory. It may drop, restart or deliver out of order.
//   - Follow reads the feed: the log (D1) is the source of truth, the hub only says "look again".
//   - Stream serves it to a browser as server-sent events, each item rendered by the caller.
//
// One hub carries one type of item. A second feed is a second hub, with its own name:
//
//	notes, err := DurableObject[Note]("HUB", "notes")       // on Cloudflare (hub_js.go)
//	devices, err := DurableObject[Device]("HUB", "devices") // the same class, another object
//	var local Memory[Device]                                // natively and in tests
package live

import (
	"context"
	"sync"
)

// Hub is the live fan-out of items of one type.
type Hub[T any] interface {
	Publish(ctx context.Context, item T) error
	// Subscribe calls listener for every published item and onError when the subscription breaks;
	// neither may block. It returns unsubscribe.
	Subscribe(listener func(T), onError func(error)) (unsubscribe func(), err error)
}

// Memory is a Hub in one process: for a native build and for tests. The zero value is ready.
type Memory[T any] struct {
	mu        sync.Mutex
	listeners map[int]func(T)
	next      int
}

func (m *Memory[T]) Publish(_ context.Context, item T) error {
	m.mu.Lock()
	listeners := make([]func(T), 0, len(m.listeners))
	for _, listener := range m.listeners {
		listeners = append(listeners, listener)
	}
	m.mu.Unlock()
	for _, listener := range listeners {
		listener(item)
	}
	return nil
}

func (m *Memory[T]) Subscribe(listener func(T), _ func(error)) (func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listeners == nil {
		m.listeners = map[int]func(T){}
	}
	n := m.next
	m.next++
	m.listeners[n] = listener
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.listeners, n)
	}, nil
}
