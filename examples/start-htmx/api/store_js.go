//go:build js && wasm

package api

import (
	"context"
	"errors"

	"github.com/joeblew999/charter/go/d1"
)

// D1Store is the messages table (migrations/) on the D1 binding: one awaited call per statement,
// and the rows reach Go as one string (package d1).
type D1Store struct{ DB d1.DB }

func (s D1Store) Create(_ context.Context, body string) (Message, error) {
	messages, err := d1.Query[Message](s.DB, "INSERT INTO messages (body) VALUES (?) RETURNING id, body, created_at", body)
	if err != nil {
		return Message{}, err
	}
	if len(messages) != 1 {
		return Message{}, errors.New("the insert returned no message")
	}
	return messages[0], nil
}

func (s D1Store) Before(_ context.Context, before int64, limit int) ([]Message, error) {
	return d1.Query[Message](s.DB, "SELECT id, body, created_at FROM messages WHERE id < ? ORDER BY id DESC LIMIT ?", before, limit)
}

func (s D1Store) Since(_ context.Context, after int64, limit int) ([]Message, error) {
	return d1.Query[Message](s.DB, "SELECT id, body, created_at FROM messages WHERE id > ? ORDER BY id LIMIT ?", after, limit)
}

func (s D1Store) Latest(context.Context) (int64, error) {
	newest, err := d1.Query[Message](s.DB, "SELECT COALESCE(MAX(id), 0) AS id FROM messages")
	if err != nil || len(newest) != 1 {
		return 0, err
	}
	return newest[0].ID, nil
}
