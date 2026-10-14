//go:build js && wasm

package api

import (
	"context"
	"errors"

	"github.com/joeblew999/charter/go/d1"
)

// D1Store is the notes table (migrations/ at the repo root) on the D1 binding itself: one awaited
// call per statement, and the rows reach Go as one string (package d1). It is what runs on
// Cloudflare. SQLStore is the same statements through database/sql.
type D1Store struct{ DB d1.DB }

func (s D1Store) Create(_ context.Context, body string) (Note, error) {
	notes, err := d1.Query[Note](s.DB, "INSERT INTO notes (body) VALUES (?) RETURNING id, body, created_at", body)
	if err != nil {
		return Note{}, err
	}
	if len(notes) != 1 {
		return Note{}, errors.New("the insert returned no note")
	}
	return notes[0], nil
}

func (s D1Store) Before(_ context.Context, before int64, limit int) ([]Note, error) {
	return d1.Query[Note](s.DB, "SELECT id, body, created_at FROM notes WHERE id < ? ORDER BY id DESC LIMIT ?", before, limit)
}

func (s D1Store) Since(_ context.Context, after int64, limit int) ([]Note, error) {
	return d1.Query[Note](s.DB, "SELECT id, body, created_at FROM notes WHERE id > ? ORDER BY id LIMIT ?", after, limit)
}

func (s D1Store) Latest(context.Context) (int64, error) {
	newest, err := d1.Query[Note](s.DB, "SELECT COALESCE(MAX(id), 0) AS id FROM notes")
	if err != nil || len(newest) != 1 {
		return 0, err
	}
	return newest[0].ID, nil
}
