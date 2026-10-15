//go:build js && wasm

package api

import (
	"context"
	"errors"

	"github.com/joeblew999/charter/go/lib/d1"
)

// D1Store is the notes table (migrations/) on the D1 binding: what runs on Cloudflare.
type D1Store struct{ DB d1.DB }

func (s D1Store) Page(_ context.Context, offset, limit int) ([]Entry, error) {
	return d1.Query[Entry](s.DB, "SELECT seq, id, body FROM notes ORDER BY seq LIMIT ? OFFSET ?", limit, offset)
}

// Put inserts the note unless its id is taken; RETURNING gives no row then, and the stored one is read.
func (s D1Store) Put(_ context.Context, note Note) (Entry, bool, error) {
	added, err := d1.Query[Entry](s.DB, "INSERT INTO notes (id, body) VALUES (?, ?) ON CONFLICT (id) DO NOTHING RETURNING seq, id, body", note.ID, note.Body)
	if err != nil {
		return Entry{}, false, err
	}
	if len(added) == 1 {
		return added[0], true, nil
	}
	stored, err := d1.Query[Entry](s.DB, "SELECT seq, id, body FROM notes WHERE id = ?", note.ID)
	if err != nil {
		return Entry{}, false, err
	}
	if len(stored) != 1 {
		return Entry{}, false, errors.New("the note is neither new nor stored")
	}
	return stored[0], false, nil
}

func (s D1Store) Since(_ context.Context, after int64, limit int) ([]Entry, error) {
	return d1.Query[Entry](s.DB, "SELECT seq, id, body FROM notes WHERE seq > ? ORDER BY seq LIMIT ?", after, limit)
}

func (s D1Store) Latest(context.Context) (int64, error) {
	newest, err := d1.Query[Entry](s.DB, "SELECT COALESCE(MAX(seq), 0) AS seq FROM notes")
	if err != nil || len(newest) != 1 {
		return 0, err
	}
	return newest[0].Seq, nil
}
