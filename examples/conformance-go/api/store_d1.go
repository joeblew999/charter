//go:build js && wasm

package api

import (
	"context"
	"errors"

	"github.com/joeblew999/charter/d1"
)

// D1Store is the notes table (migrations/) on the D1 binding, through the queries of
// api/queries.sql, which npm run spec writes as typed Go (api/queries.go): what runs on Cloudflare.
type D1Store struct{ DB d1.DB }

func (s D1Store) Page(_ context.Context, offset, limit int) ([]Entry, error) {
	rows, err := NotesPage(s.DB, int64(limit), int64(offset))
	return entries(rows), err
}

// Put stores the note unless its id is taken; then the stored one is read.
func (s D1Store) Put(_ context.Context, note Note) (Entry, bool, error) {
	added, err := AddNote(s.DB, note.ID, note.Body)
	if err == nil {
		return Entry(added), true, nil
	}
	if !errors.Is(err, d1.ErrNoRows) {
		return Entry{}, false, err
	}
	stored, err := NoteByID(s.DB, note.ID)
	return Entry(stored), false, err
}

func (s D1Store) Since(_ context.Context, after int64, limit int) ([]Entry, error) {
	rows, err := NotesAfter(s.DB, after, int64(limit))
	return entries(rows), err
}

func (s D1Store) Latest(context.Context) (int64, error) {
	return NewestSeq(s.DB)
}

// entries are the table's rows as the log's entries: the same fields.
func entries(rows []NotesRow) []Entry {
	out := make([]Entry, len(rows))
	for i, row := range rows {
		out[i] = Entry(row)
	}
	return out
}
