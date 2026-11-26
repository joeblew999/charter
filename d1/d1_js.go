//go:build js && wasm

package d1

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"
	workersd1 "github.com/syumai/workers-go/cloudflare/d1"

	"github.com/joeblew999/charter/internal/promise"
)

// DB is a D1 database: the binding itself. There is no connection to open or close.
type DB struct{ binding js.Value }

// Open is the database bound as name (charter.toml's [[d1]]).
func Open(name string) (DB, error) {
	binding := cloudflare.GetBinding(name)
	if !binding.Truthy() {
		return DB{}, fmt.Errorf("%s is not bound (charter.toml's [[d1]])", name)
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

// Exec runs one statement whose rows, if it has any, are not wanted.
func Exec(db DB, query string, args ...any) error {
	if _, err := promise.Await(db.binding.Call("prepare", query).Call("bind", args...).Call("run")); err != nil {
		return fmt.Errorf("d1: %w", err)
	}
	return nil
}

// ExecRows runs one statement and returns how many rows it changed (an INSERT, UPDATE or DELETE): D1's meta.changes.
func ExecRows(db DB, query string, args ...any) (int64, error) {
	result, err := promise.Await(db.binding.Call("prepare", query).Call("bind", args...).Call("run"))
	if err != nil {
		return 0, fmt.Errorf("d1: %w", err)
	}
	return int64(result.Get("meta").Get("changes").Int()), nil
}

// Batch runs the statements in order as one transaction, D1's batch: one call, and if a statement fails, none of
// them happened. No statements is no call: D1 refuses an empty batch.
func Batch(db DB, statements ...Statement) error {
	if len(statements) == 0 {
		return nil
	}
	prepared := make([]any, len(statements))
	for i, s := range statements {
		prepared[i] = db.binding.Call("prepare", s.Query).Call("bind", s.Args...)
	}
	if _, err := promise.Await(db.binding.Call("batch", js.ValueOf(prepared))); err != nil {
		return fmt.Errorf("d1: batch: %w", err)
	}
	return nil
}

// SQL is the database bound as name for database/sql, through workers-go's own driver
// (github.com/syumai/workers-go/cloudflare/d1): for code written the workers-go way. Under go test it is the same
// SQLite stand-in Open gives, so that code runs there too. The caller closes it.
func SQL(name string) (*sql.DB, error) {
	connector, err := workersd1.OpenConnector(name)
	if err != nil {
		return nil, fmt.Errorf("d1: %s: %w", name, err)
	}
	return sql.OpenDB(connector), nil
}
