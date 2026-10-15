-- The messages' queries, against the schema in migrations/. npm run spec writes them as Go functions
-- over go/lib/d1 (api/queries.go), their types worked out by sqlc.

-- name: CreateMessage :one
INSERT INTO messages (body) VALUES (?) RETURNING id, body, created_at;

-- name: MessagesBefore :many
-- MessagesBefore is a page of messages, newest first, before a message id.
SELECT id, body, created_at FROM messages WHERE id < ? ORDER BY id DESC LIMIT ?;

-- name: MessagesAfter :many
-- MessagesAfter is the messages after a message id, oldest first: the log a stream follows.
SELECT id, body, created_at FROM messages WHERE id > ? ORDER BY id LIMIT ?;

-- name: NewestMessage :one
-- NewestMessage is the id of the newest message, 0 if there is none.
SELECT CAST(COALESCE(MAX(id), 0) AS INTEGER) AS id FROM messages;
