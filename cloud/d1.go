// D1: statements on a Cloudflare D1 database.
//
// A value that crosses between Go and JavaScript costs CPU, and so does every promise Go waits
// for. database/sql over workers-go's driver reads a result value by value, and starts a goroutine
// for every database opened. Here a statement is one awaited call, and its rows come over as one
// string of JSON, whatever their number:
//
//	db, err := Open("DB") // per request: a binding belongs to the request's environment
//	notes, err := Query[Note](db, "SELECT id, body FROM notes WHERE id < ? LIMIT ?", before, limit)
//
// A request that writes several statements that must all happen, or none, sends them as one Batch: D1 has no
// transaction a Worker holds open, and a batch is one (it is rolled back when a statement fails):
//
//	err := Batch(db,
//		Stmt("UPDATE boards SET version = version + 1 WHERE topic = ?", topic),
//		Stmt("DELETE FROM notes WHERE topic = ?", topic))
//
// ExecRows is Exec that also says how many rows the statement changed.
//
// charter check --fix writes a project's queries (db/queries.sql) as functions over these, typed by sqlc.

package cloud

import "errors"

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

// Statement is one statement of a Batch: its SQL and the arguments bound to its ? in order.
type Statement struct {
	Query string
	Args  []any
}

// Stmt is a Statement, written as a call to Exec is: Stmt("DELETE FROM notes WHERE id = ?", id).
func Stmt(query string, args ...any) Statement { return Statement{Query: query, Args: args} }

// OrNull is a nullable argument as a statement binds it: its value, or nil (NULL).
func OrNull[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}
