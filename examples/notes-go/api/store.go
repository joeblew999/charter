package api

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/joeblew999/charter/go/hub"
)

// Store is the log: D1 on Cloudflare (D1Store, store_js.go), memory for `go run .` and the tests (MemStore).
type Store interface {
	Create(ctx context.Context, body string) (Note, error)
	// Before returns notes with id < before, newest first, at most limit.
	Before(ctx context.Context, before int64, limit int) ([]Note, error)
	// Since returns notes with id > after, oldest first, at most limit.
	Since(ctx context.Context, after int64, limit int) ([]Note, error)
	// Latest is the newest id, 0 when there are no notes.
	Latest(ctx context.Context) (int64, error)
}

// Hub is the live fan-out of notes: only a wake-up signal for followers (docs/guides/streaming.md, rule 2).
// A feed of another type has its own: hub.Hub[T].
type Hub = hub.Hub[Note]

// SQLStore is the notes table (migrations/ at the repo root) through database/sql: for a native
// build with a SQLite driver. The Worker does not use it: over workers-go's d1 driver a list of 20
// notes cost 1 ms of CPU more on Cloudflare than D1Store's, because the driver brings every value
// of every row from JavaScript to Go on its own.
type SQLStore struct{ DB *sql.DB }

func (s SQLStore) Create(ctx context.Context, body string) (Note, error) {
	rows, err := s.query(ctx, "INSERT INTO notes (body) VALUES (?) RETURNING id, body, created_at", body)
	if err != nil {
		return Note{}, err
	}
	if len(rows) != 1 {
		return Note{}, sql.ErrNoRows
	}
	return rows[0], nil
}

func (s SQLStore) Before(ctx context.Context, before int64, limit int) ([]Note, error) {
	return s.query(ctx, "SELECT id, body, created_at FROM notes WHERE id < ? ORDER BY id DESC LIMIT ?", before, limit)
}

func (s SQLStore) Since(ctx context.Context, after int64, limit int) ([]Note, error) {
	return s.query(ctx, "SELECT id, body, created_at FROM notes WHERE id > ? ORDER BY id LIMIT ?", after, limit)
}

func (s SQLStore) Latest(ctx context.Context) (int64, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM notes").Scan(&id)
	return id, err
}

func (s SQLStore) query(ctx context.Context, query string, args ...any) ([]Note, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notes := []Note{}
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.Body, &note.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

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
