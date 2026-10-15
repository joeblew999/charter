package api

import (
	"context"
	"sync"
	"time"

	"github.com/joeblew999/charter/go/hub"
)

// Store is the log: D1 on Cloudflare (D1Store, store_js.go), memory for `go run .` and the tests
// (MemStore).
type Store interface {
	Create(ctx context.Context, body string) (Message, error)
	// Before returns messages with id < before, newest first, at most limit.
	Before(ctx context.Context, before int64, limit int) ([]Message, error)
	// Since returns messages with id > after, oldest first, at most limit.
	Since(ctx context.Context, after int64, limit int) ([]Message, error)
	// Latest is the newest id, 0 when there are none.
	Latest(ctx context.Context) (int64, error)
}

// Hub is the live fan-out of messages: only a wake-up for followers (go/hub). On Cloudflare it is
// the library's Durable Object (platform_js.go).
type Hub = hub.Hub[Message]

// MemStore is a Store and a Hub in one process: for `go run .` and the tests.
type MemStore struct {
	hub.Memory[Message]
	mu       sync.Mutex
	messages []Message
}

func (m *MemStore) Create(_ context.Context, body string) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	message := Message{ID: int64(len(m.messages) + 1), Body: body, CreatedAt: time.Now().UTC().Format("2006-01-02 15:04:05")}
	m.messages = append(m.messages, message)
	return message, nil
}

func (m *MemStore) Before(_ context.Context, before int64, limit int) ([]Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Message{}
	for i := len(m.messages) - 1; i >= 0 && len(out) < limit; i-- {
		if m.messages[i].ID < before {
			out = append(out, m.messages[i])
		}
	}
	return out, nil
}

func (m *MemStore) Since(_ context.Context, after int64, limit int) ([]Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Message{}
	for _, message := range m.messages {
		if message.ID > after && len(out) < limit {
			out = append(out, message)
		}
	}
	return out, nil
}

func (m *MemStore) Latest(context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.messages)), nil
}
