//go:build js && wasm

package d1

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/charter/internal/promise"
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

// Exec runs one statement whose rows, if it has any, are not wanted.
func Exec(db DB, query string, args ...any) error {
	if _, err := promise.Await(db.binding.Call("prepare", query).Call("bind", args...).Call("run")); err != nil {
		return fmt.Errorf("d1: %w", err)
	}
	return nil
}
