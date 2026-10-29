//go:build js && wasm

package api

import (
	"context"

	"github.com/joeblew999/charter/d1"
)

// D1Store is the notes table (the project's migrations/) on the D1 binding itself, through the
// queries of api/queries.sql, which npm run spec writes as typed Go (api/queries.go): one awaited
// call per statement, and the rows reach Go as one string (package d1). It is what runs on
// Cloudflare.
type D1Store struct{ DB d1.DB }

func (s D1Store) Create(_ context.Context, body string) (Note, error) {
	row, err := CreateNote(s.DB, body)
	return Note(row), err
}

func (s D1Store) Before(_ context.Context, before int64, limit int) ([]Note, error) {
	rows, err := NotesBefore(s.DB, before, int64(limit))
	return notes(rows), err
}

func (s D1Store) Since(_ context.Context, after int64, limit int) ([]Note, error) {
	rows, err := NotesAfter(s.DB, after, int64(limit))
	return notes(rows), err
}

func (s D1Store) Latest(context.Context) (int64, error) {
	return NewestNote(s.DB)
}

// notes are the table's rows as the API's notes: the same fields.
func notes(rows []NotesRow) []Note {
	out := make([]Note, len(rows))
	for i, row := range rows {
		out[i] = Note(row)
	}
	return out
}
