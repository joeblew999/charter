-- The conformance API's queries, against the schema in migrations/. npm run spec writes them as Go
-- functions over go/lib/d1 (api/queries.go), their types worked out by sqlc.

-- name: NotesPage :many
-- NotesPage is a page of notes, in the order they were stored.
SELECT seq, id, body FROM notes ORDER BY seq LIMIT ? OFFSET ?;

-- name: AddNote :one
-- AddNote stores a note unless its id is taken, and then returns no row (d1.ErrNoRows).
INSERT INTO notes (id, body) VALUES (?, ?) ON CONFLICT (id) DO NOTHING RETURNING seq, id, body;

-- name: NoteByID :one
-- NoteByID is the stored note of an id.
SELECT seq, id, body FROM notes WHERE id = ?;

-- name: NotesAfter :many
-- NotesAfter is the notes stored after a position, in order: the log a stream follows.
SELECT seq, id, body FROM notes WHERE seq > ? ORDER BY seq LIMIT ?;

-- name: NewestSeq :one
-- NewestSeq is the position of the newest note, 0 if there is none.
SELECT CAST(COALESCE(MAX(seq), 0) AS INTEGER) AS seq FROM notes;
