//go:build !(js && wasm)

package d1

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/joeblew999/charter/project"
	_ "modernc.org/sqlite" // SQLite in pure Go, as D1 is SQLite
)

// DB is a database standing in for D1: SQLite in memory, the binding's, for the life of the process.
type DB struct{ sql *sql.DB }

var open = struct {
	mu  sync.Mutex
	dbs map[string]*sql.DB
}{dbs: map[string]*sql.DB{}}

// Open is the database standing in for the D1 binding called name: SQLite in memory, made on the first Open and
// migrated with the *.sql files charter.toml's [[d1]] of that binding names (found from the working directory), in
// name order. Later Opens in the process share it.
func Open(name string) (DB, error) {
	open.mu.Lock()
	defer open.mu.Unlock()
	if db, ok := open.dbs[name]; ok {
		return DB{db}, nil
	}
	p, err := project.Load(".")
	if err != nil {
		return DB{}, fmt.Errorf("d1: %s: %w", name, err)
	}
	i := slices.IndexFunc(p.D1, func(d project.D1) bool { return d.Binding == name })
	if i < 0 {
		return DB{}, fmt.Errorf("d1: charter.toml has no [[d1]] binding %s", name)
	}
	db, err := sql.Open("sqlite", "file:charter-d1-"+name+"?mode=memory&cache=shared")
	if err != nil {
		return DB{}, err
	}
	db.SetMaxOpenConns(1) // one connection: the database lives as long as it does
	if dir := p.D1[i].Migrations; dir != "" {
		files, err := filepath.Glob(filepath.Join(p.Root, dir, "*.sql"))
		if err != nil {
			return DB{}, err
		}
		slices.Sort(files)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				return DB{}, err
			}
			if _, err := db.Exec(string(b)); err != nil {
				return DB{}, fmt.Errorf("d1: %s: migration %s: %w", name, filepath.Base(f), err)
			}
		}
	}
	open.dbs[name] = db
	return DB{db}, nil
}

// Query runs one statement and returns its rows, decoded from JSON as on Workers: an object per row with the column
// names as its keys (a BLOB is an array of numbers, as D1 sends it).
func Query[T any](db DB, query string, args ...any) ([]T, error) {
	rows, err := db.sql.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("d1: %w", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("d1: %w", err)
	}
	objects := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("d1: %w", err)
		}
		row := map[string]any{}
		for i, c := range columns {
			if blob, ok := values[i].([]byte); ok {
				numbers := make([]int, len(blob))
				for j, b := range blob {
					numbers[j] = int(b)
				}
				values[i] = numbers
			}
			row[c] = values[i]
		}
		objects = append(objects, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("d1: %w", err)
	}
	b, err := json.Marshal(objects)
	if err != nil {
		return nil, fmt.Errorf("d1: %w", err)
	}
	out := []T{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("d1: a row is not a %T: %w", *new(T), err)
	}
	return out, nil
}

// Exec runs one statement whose rows, if it has any, are not wanted.
func Exec(db DB, query string, args ...any) error {
	if _, err := db.sql.Exec(query, args...); err != nil {
		return fmt.Errorf("d1: %w", err)
	}
	return nil
}
