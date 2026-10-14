// Package hub is the live signal of a feed: publish an item, and every subscriber gets it.
//
// It is only a wake-up signal. The log (D1) is the source of truth, and follow.Follow fills any
// gap from it, so a hub may drop, restart or deliver out of order (docs/guides/streaming.md, rule 2).
//
// One hub carries one type of item. A second feed is a second hub, with its own name:
//
//	notes, err := hub.DurableObject[Note]("HUB", "notes")       // on Cloudflare (hub_js.go)
//	devices, err := hub.DurableObject[Device]("HUB", "devices") // the same class, another object
//	var local hub.Memory[Device]                                // natively and in tests
package hub

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
