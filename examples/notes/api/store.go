package api

import (
	"context"
	"sync"
	"time"

	"github.com/joeblew999/charter/hub"
)

// Store is the log: D1 on Cloudflare (D1Store, store_d1.go), memory for `go run .` and the tests (MemStore).
type Store interface {
	Create(ctx context.Context, body string) (Note, error)
	// Before returns notes with id < before, newest first, at most limit.
	Before(ctx context.Context, before int64, limit int) ([]Note, error)
	// Since returns notes with id > after, oldest first, at most limit.
	Since(ctx context.Context, after int64, limit int) ([]Note, error)
	// Latest is the newest id, 0 when there are no notes.
	Latest(ctx context.Context) (int64, error)
}

// Hub is the live fan-out of notes: only a wake-up signal for followers (docs/self/streaming.md, rule 2).
// A feed of another type has its own: hub.Hub[T].
type Hub = hub.Hub[Note]

// MemStore is a Store and a Hub in one process: for `go run .` and the tests.
type MemStore struct {
	hub.Memory[Note]
	mu    sync.Mutex
	notes []Note
}

func (m *MemStore) Create(_ context.Context, body string) (Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	note := Note{ID: int64(len(m.notes) + 1), Body: body, CreatedAt: time.Now().UTC().Format("2006-01-02 15:04:05")}
	m.notes = append(m.notes, note)
	return note, nil
}

func (m *MemStore) Before(_ context.Context, before int64, limit int) ([]Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Note{}
	for i := len(m.notes) - 1; i >= 0 && len(out) < limit; i-- {
		if m.notes[i].ID < before {
			out = append(out, m.notes[i])
		}
	}
	return out, nil
}

func (m *MemStore) Since(_ context.Context, after int64, limit int) ([]Note, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Note{}
	for _, note := range m.notes {
		if note.ID > after && len(out) < limit {
			out = append(out, note)
		}
	}
	return out, nil
}

func (m *MemStore) Latest(context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.notes)), nil
}
