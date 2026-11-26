-- The notes' queries, against schema.sql. charter check --fix writes them as Go functions over
-- charter/cloud (queries.gen.go), their types worked out by sqlc.

-- name: CreateNote :one
INSERT INTO notes (body, author_id, author) VALUES (?, ?, ?) RETURNING *;

-- name: RemoveNote :one
-- RemoveNote takes a note away, and says which it was.
DELETE FROM notes WHERE id = ? RETURNING *;

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

-- name: NotesBy :many
-- NotesBy is someone's notes, newest first: what they may attach a file to.
SELECT * FROM notes WHERE author_id = ? ORDER BY id DESC LIMIT ?;

-- name: NotesReport :one
-- NotesReport is how many notes there are, and by how many people.
SELECT CAST(COUNT(*) AS INTEGER) AS notes, CAST(COUNT(DISTINCT author_id) AS INTEGER) AS authors FROM notes;

-- name: NotesSince :one
-- NotesSince is how many notes were written after a time.
SELECT CAST(COUNT(*) AS INTEGER) AS notes FROM notes WHERE created_at > ?;

-- name: RecordActivity :one
INSERT INTO activity (kind, note_id, author, notes) VALUES (?, ?, ?, ?) RETURNING *;

-- name: ActivityBefore :many
-- ActivityBefore is what happened before a position, newest first.
SELECT * FROM activity WHERE id < ? ORDER BY id DESC LIMIT ?;

-- name: ActivityAfter :many
-- ActivityAfter is what happened after a position, oldest first: the log its feed follows.
SELECT * FROM activity WHERE id > ? ORDER BY id LIMIT ?;

-- name: NewestActivity :one
SELECT CAST(COALESCE(MAX(id), 0) AS INTEGER) AS id FROM activity;

-- name: AddAttachment :one
INSERT INTO attachments (object, name, type, size, note_id, author_id, author) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: Attachments :many
-- Attachments is the files attached, newest first.
SELECT * FROM attachments ORDER BY id DESC LIMIT ?;

-- name: Attachment :one
SELECT * FROM attachments WHERE id = ?;

-- name: RemoveAttachment :one
DELETE FROM attachments WHERE id = ? RETURNING *;
