package api

import (
	"context"
	"sync"

	"github.com/joeblew999/charter/go/lib/hub"
)

// Entry is a note in the log. Seq is its place there: the only position a follower keeps
// (docs/self/streaming.md, rule 1). The note's id is the API's, and the Idempotency-Key when the
// creator sent one.
type Entry struct {
	Seq  int64  `json:"seq"`
	ID   string `json:"id"`
	Body string `json:"body"`
}

func (e Entry) Position() int64 { return e.Seq }
func (e Entry) Note() Note      { return Note{ID: e.ID, Body: e.Body} }

// Store is the notes' log: D1 on Cloudflare (D1Store, store_d1.go), memory natively and in the
// tests (MemStore). It starts with the five notes migrations/0001_init.sql seeds.
type Store interface {
	// Page returns the notes in the log's order from offset, at most limit.
	Page(ctx context.Context, offset, limit int) ([]Entry, error)
	// Put stores the note under its id if there is none: it returns the one stored under the id,
	// and whether it is new.
	Put(ctx context.Context, note Note) (Entry, bool, error)
	// Since returns the entries after the position after, oldest first, at most limit.
	Since(ctx context.Context, after int64, limit int) ([]Entry, error)
	// Latest is the newest position, 0 when the log is empty.
	Latest(ctx context.Context) (int64, error)
}

// Hub is the live fan-out of new notes: only a wake-up signal for followers (docs/self/streaming.md, rule 2).
type Hub = hub.Hub[Entry]

// seeds are the notes every log starts with: migrations/0001_init.sql writes the same into D1.
var seeds = []Note{{"n1", "note n1"}, {"n2", "note n2"}, {"n3", "note n3"}, {"n4", "note n4"}, {"n5", "note n5"}}

// MemStore is a Store and a Hub in one process: for `go run .` and the tests.
type MemStore struct {
	hub.Memory[Entry]
	mu      sync.Mutex
	entries []Entry
}

// NewMemStore is a log with the seeded notes.
func NewMemStore() *MemStore {
	m := &MemStore{}
	for _, note := range seeds {
		m.Put(context.Background(), note)
	}
	return m
}

func (m *MemStore) Page(_ context.Context, offset, limit int) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	start := min(offset, len(m.entries))
	return append([]Entry{}, m.entries[start:min(start+limit, len(m.entries))]...), nil
}

func (m *MemStore) Put(_ context.Context, note Note) (Entry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.entries {
		if entry.ID == note.ID {
			return entry, false, nil
		}
	}
	entry := Entry{Seq: int64(len(m.entries) + 1), ID: note.ID, Body: note.Body}
	m.entries = append(m.entries, entry)
	return entry, true, nil
}

func (m *MemStore) Since(_ context.Context, after int64, limit int) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Entry{}
	for _, entry := range m.entries {
		if entry.Seq > after && len(out) < limit {
			out = append(out, entry)
		}
	}
	return out, nil
}

func (m *MemStore) Latest(context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.entries)), nil
}
