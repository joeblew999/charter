//go:build js && wasm

package api

import (
	"context"

	"github.com/joeblew999/charter/d1"
)

// D1Store is the messages table (migrations/) on the D1 binding, through the queries of
// api/queries.sql, which npm run spec writes as typed Go (api/queries.go): one awaited call per
// statement, and the rows reach Go as one string (package d1).
type D1Store struct{ DB d1.DB }

func (s D1Store) Create(_ context.Context, body string) (Message, error) {
	row, err := CreateMessage(s.DB, body)
	return Message(row), err
}

func (s D1Store) Before(_ context.Context, before int64, limit int) ([]Message, error) {
	rows, err := MessagesBefore(s.DB, before, int64(limit))
	return messages(rows), err
}

func (s D1Store) Since(_ context.Context, after int64, limit int) ([]Message, error) {
	rows, err := MessagesAfter(s.DB, after, int64(limit))
	return messages(rows), err
}

func (s D1Store) Latest(context.Context) (int64, error) {
	return NewestMessage(s.DB)
}

// messages are the table's rows as the API's messages: the same fields.
func messages(rows []MessagesRow) []Message {
	out := make([]Message, len(rows))
	for i, row := range rows {
		out[i] = Message(row)
	}
	return out
}
