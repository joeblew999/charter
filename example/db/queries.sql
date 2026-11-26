-- The notes' queries, against schema.sql. charter check --fix writes them as Go functions over
-- charter/cloud (queries.gen.go), their types worked out by sqlc.

-- name: CreateNote :one
INSERT INTO notes (body) VALUES (?) RETURNING *;

-- name: Note :one
-- Note is one note, by its id.
SELECT * FROM notes WHERE id = ?;

-- name: NotesBefore :many
-- NotesBefore is a page of notes, newest first, before a note id.
SELECT * FROM notes WHERE id < ? ORDER BY id DESC LIMIT ?;

-- name: NotesAfter :many
-- NotesAfter is the notes after a note id, oldest first: the log a stream follows.
SELECT * FROM notes WHERE id > ? ORDER BY id LIMIT ?;

-- name: NewestNote :one
-- NewestNote is the id of the newest note, 0 if there is none.
SELECT CAST(COALESCE(MAX(id), 0) AS INTEGER) AS id FROM notes;
