package api

import (
	"context"
	"net/http"

	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/db"
)

// Store is the notes: what a page's Props and the handlers ask for. On a Worker it is D1 (the table of
// db/schema.sql, through the queries of db/queries.sql); a test gives one of its own.
type Store interface {
	// Create keeps a note, by whoever wrote it (their id and name).
	Create(ctx context.Context, body, authorID, author string) (Note, error)
	// Remove takes a note away.
	Remove(ctx context.Context, id int64) error
	// Get is the note with that id.
	Get(ctx context.Context, id int64) (Note, error)
	// Before is the notes with an id below before, newest first, at most limit.
	Before(ctx context.Context, before int64, limit int) ([]Note, error)
	// Since is the notes with an id above after, oldest first, at most limit: the log the feed follows.
	Since(ctx context.Context, after int64, limit int) ([]Note, error)
	// Latest is the newest id, 0 when there are no notes.
	Latest(ctx context.Context) (int64, error)
}

// OpenStore is the store for a request: the D1 binding DB, which belongs to the request's environment. It is the
// app's request arg (app.App's RequestArgs), so a page declares a Store and never opens one.
func OpenStore(*http.Request) (Store, error) { return open() }

// StoreOn is the store on a D1 database that is open already: for the seed, which is handed one.
func StoreOn(conn cloud.DB) Store { return d1{conn} }

func open() (Store, error) {
	conn, err := cloud.Open("DB")
	return d1{conn}, err
}

type d1 struct{ db cloud.DB }

func (s d1) Create(_ context.Context, body, authorID, author string) (Note, error) {
	row, err := db.CreateNote(s.db, body, authorID, author)
	return Note{row}, err
}

func (s d1) Remove(_ context.Context, id int64) error {
	_, err := db.RemoveNote(s.db, id)
	return err
}

func (s d1) Get(_ context.Context, id int64) (Note, error) {
	row, err := db.Note(s.db, id)
	return Note{row}, err
}

func (s d1) Before(_ context.Context, before int64, limit int) ([]Note, error) {
	rows, err := db.NotesBefore(s.db, before, int64(limit))
	return notes(rows), err
}

func (s d1) Since(_ context.Context, after int64, limit int) ([]Note, error) {
	rows, err := db.NotesAfter(s.db, after, int64(limit))
	return notes(rows), err
}

func (s d1) Latest(context.Context) (int64, error) { return db.NewestNote(s.db) }

// notes are the table's rows as the API's notes.
func notes(rows []db.NotesRow) []Note {
	out := make([]Note, len(rows))
	for i, row := range rows {
		out[i] = Note{row}
	}
	return out
}
