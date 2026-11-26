package api

import (
	"context"

	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/db"
)

// Store is the notes: the table of db/migrations on the D1 binding DB, through the queries of db/queries.sql, which
// charter check --fix writes as typed Go (db/queries.gen.go). It is the log the feed follows.
type Store struct{ DB cloud.DB }

// Open is the store on the request's database: a binding belongs to the request's environment.
func Open() (Store, error) {
	conn, err := cloud.Open("DB")
	return Store{conn}, err
}

func (s Store) Create(_ context.Context, body string) (Note, error) {
	row, err := db.CreateNote(s.DB, body)
	return Note(row), err
}

// Before is the notes with an id below before, newest first, at most limit.
func (s Store) Before(_ context.Context, before int64, limit int) ([]Note, error) {
	rows, err := db.NotesBefore(s.DB, before, int64(limit))
	return notes(rows), err
}

// Since is the notes with an id above after, oldest first, at most limit.
func (s Store) Since(_ context.Context, after int64, limit int) ([]Note, error) {
	rows, err := db.NotesAfter(s.DB, after, int64(limit))
	return notes(rows), err
}

// Latest is the newest id, 0 when there are no notes.
func (s Store) Latest(context.Context) (int64, error) { return db.NewestNote(s.DB) }

// notes are the table's rows as the API's notes: the same fields.
func notes(rows []db.NotesRow) []Note {
	out := make([]Note, len(rows))
	for i, row := range rows {
		out[i] = Note(row)
	}
	return out
}
