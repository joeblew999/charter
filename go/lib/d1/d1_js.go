//go:build js && wasm

// Package d1 runs statements on a Cloudflare D1 database from a Go Worker.
//
// A value that crosses between Go and JavaScript costs CPU, and so does every promise Go waits
// for. database/sql over workers-go's driver reads a result value by value, and starts a goroutine
// for every database opened. Here a statement is one awaited call, and its rows come over as one
// string of JSON, whatever their number:
//
//	db, err := d1.Open("DB") // per request: a binding belongs to the request's environment
//	notes, err := d1.Query[Note](db, "SELECT id, body FROM notes WHERE id < ? LIMIT ?", before, limit)
//
// npm run spec writes a project's queries (api/queries.sql) as functions over these, typed by sqlc
// (charter.config.ts: db: "sqlc").
package d1

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/charter/go/lib/internal/promise"
)

// DB is a D1 database: the binding itself. There is no connection to open or close.
type DB struct{ binding js.Value }

// Open is the database bound as name (cloudflare.config.ts).
func Open(name string) (DB, error) {
	binding := cloudflare.GetBinding(name)
	if !binding.Truthy() {
		return DB{}, fmt.Errorf("%s is not bound (cloudflare.config.ts)", name)
	}
	return DB{binding}, nil
}

var stringify = js.Global().Get("JSON").Get("stringify")

// Query runs one statement and returns its rows: those of a SELECT or of a RETURNING clause, and
// none for any other statement. A row is decoded from JSON, an object with the column names as its
// keys, so T is a struct whose json tags name the columns. An INTEGER or a REAL is a number, TEXT
// a string, NULL null, and a BLOB an array of numbers.
//
// The arguments are bound to the statement's ? in order: strings, numbers, bools and nil.
func Query[T any](db DB, query string, args ...any) ([]T, error) {
	result, err := promise.Await(db.binding.Call("prepare", query).Call("bind", args...).Call("all"))
	if err != nil {
		return nil, fmt.Errorf("d1: %w", err)
	}
	rows := []T{}
	if err := json.Unmarshal([]byte(stringify.Invoke(result.Get("results")).String()), &rows); err != nil {
		return nil, fmt.Errorf("d1: a row is not a %T: %w", *new(T), err)
	}
	return rows, nil
}

// ErrNoRows is One's error for a statement that returned no row.
var ErrNoRows = errors.New("d1: no rows")

// One runs one statement and returns its first row: a SELECT that finds it, or a statement with a
// RETURNING clause. No row is ErrNoRows.
func One[T any](db DB, query string, args ...any) (T, error) {
	rows, err := Query[T](db, query, args...)
	if err != nil {
		return *new(T), err
	}
	if len(rows) == 0 {
		return *new(T), ErrNoRows
	}
	return rows[0], nil
}

// Exec runs one statement whose rows, if it has any, are not wanted.
func Exec(db DB, query string, args ...any) error {
	if _, err := promise.Await(db.binding.Call("prepare", query).Call("bind", args...).Call("run")); err != nil {
		return fmt.Errorf("d1: %w", err)
	}
	return nil
}

// OrNull is a nullable argument as a statement binds it: its value, or nil (NULL).
func OrNull[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}
