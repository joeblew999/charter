//go:build !(js && wasm)

package d1_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joeblew999/charter/d1"
)

// TestOffWorkers: off Workers a binding is SQLite in memory, migrated as wrangler.jsonc says, and the calls behave as on
// D1: rows as JSON objects, RETURNING, no rows, one database shared by every Open.
func TestOffWorkers(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"wrangler.jsonc":      `{"name": "t", "d1_databases": [{"binding": "DB", "database_name": "t"}]}`,
		"migrations/0001.sql": "CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL, data BLOB);\n",
		"migrations/0002.sql": "INSERT INTO notes (body) VALUES ('first');\n",
	} {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	t.Chdir(dir)
	type note struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		Data []byte `json:"-"`
	}
	db, err := d1.Open("DB")
	if err != nil {
		t.Fatal(err)
	}
	added, err := d1.One[note](db, "INSERT INTO notes (body) VALUES (?) RETURNING id, body", "second")
	if err != nil || added.ID != 2 || added.Body != "second" {
		t.Fatalf("insert: %+v, %v", added, err)
	}
	again, _ := d1.Open("DB")
	notes, err := d1.Query[note](again, "SELECT id, body FROM notes ORDER BY id")
	if err != nil || len(notes) != 2 || notes[0].Body != "first" {
		t.Fatalf("query: %+v, %v", notes, err)
	}
	if _, err := d1.One[note](db, "SELECT id, body FROM notes WHERE id = ?", 99); !errors.Is(err, d1.ErrNoRows) {
		t.Errorf("no row: %v", err)
	}
	if err := d1.Exec(db, "UPDATE notes SET data = ? WHERE id = 1", []byte{1, 2}); err != nil {
		t.Fatal(err)
	}
	blobs, err := d1.Query[struct {
		Data []int `json:"data"`
	}](db, "SELECT data FROM notes WHERE id = 1")
	if err != nil || len(blobs) != 1 || len(blobs[0].Data) != 2 || blobs[0].Data[1] != 2 {
		t.Errorf("blob: %+v, %v", blobs, err)
	}
	if _, err := d1.Open("OTHER"); err == nil {
		t.Error("a binding wrangler.jsonc doesn't have opened")
	}
}

// project writes a wrangler.jsonc with one D1 binding and its migrations into a new folder, and runs the test there.
func project(t *testing.T, binding string, migrations ...string) d1.DB {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "migrations"), 0o755)
	os.WriteFile(filepath.Join(dir, "wrangler.jsonc"), []byte(`{"name": "t", "d1_databases": [{"binding": "`+binding+`", "database_name": "t"}]}`), 0o644)
	for i, sql := range migrations {
		os.WriteFile(filepath.Join(dir, "migrations", fmt.Sprintf("%04d.sql", i+1)), []byte(sql), 0o644)
	}
	t.Chdir(dir)
	db, err := d1.Open(binding)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// TestForeignKeys: a row for a parent that isn't there is refused, as D1 refuses it.
func TestForeignKeys(t *testing.T) {
	db := project(t, "FK", `CREATE TABLE boards (topic TEXT PRIMARY KEY);
CREATE TABLE notes (id INTEGER PRIMARY KEY, topic TEXT NOT NULL REFERENCES boards(topic));
INSERT INTO boards (topic) VALUES ('go');`)
	if err := d1.Exec(db, "INSERT INTO notes (topic) VALUES (?)", "go"); err != nil {
		t.Fatalf("a note of a board that exists: %v", err)
	}
	if err := d1.Exec(db, "INSERT INTO notes (topic) VALUES (?)", "none"); err == nil {
		t.Error("a note of a board that doesn't exist was stored")
	}
}

// TestExecRowsAndBatch: ExecRows counts the rows a statement changed; a Batch happens whole, or not at all.
func TestExecRowsAndBatch(t *testing.T) {
	db := project(t, "BATCH", `CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL UNIQUE);
INSERT INTO notes (body) VALUES ('a'), ('b'), ('c');`)
	count := func() int64 {
		n, err := d1.One[struct {
			N int64 `json:"n"`
		}](db, "SELECT COUNT(*) AS n FROM notes")
		if err != nil {
			t.Fatal(err)
		}
		return n.N
	}
	if changed, err := d1.ExecRows(db, "DELETE FROM notes WHERE body IN (?, ?)", "a", "b"); changed != 2 || err != nil {
		t.Errorf("deleting two: %d, %v", changed, err)
	}
	if changed, err := d1.ExecRows(db, "DELETE FROM notes WHERE body = ?", "none"); changed != 0 || err != nil {
		t.Errorf("deleting none: %d, %v", changed, err)
	}
	if err := d1.Batch(db, d1.Stmt("INSERT INTO notes (body) VALUES (?)", "d"), d1.Stmt("INSERT INTO notes (body) VALUES (?)", "e")); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 3 {
		t.Errorf("after a batch of two inserts: %d notes, want 3", n)
	}
	// The second statement breaks UNIQUE, so the first is undone with it.
	if err := d1.Batch(db, d1.Stmt("INSERT INTO notes (body) VALUES (?)", "f"), d1.Stmt("INSERT INTO notes (body) VALUES (?)", "c")); err == nil {
		t.Error("a batch with a failing statement returned no error")
	}
	if n := count(); n != 3 {
		t.Errorf("after a failed batch: %d notes, want 3 (its first insert stayed)", n)
	}
	if err := d1.Batch(db); err != nil {
		t.Errorf("an empty batch: %v", err)
	}
}

// entry is a keyed value in a last-writer-wins sync: the newer stamp wins, and a deletion is an entry too (a
// tombstone), so it wins over an older write and loses to a newer one.
type entry struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Stamp   int64  `json:"stamp"`
	Deleted bool   `json:"deleted"`
}

// mergeSQL takes a client's entries in one statement, whatever their number: json_each reads them from one JSON
// argument, and ON CONFLICT keeps the stored entry unless the incoming one is newer.
const mergeSQL = `INSERT INTO entries (key, value, stamp, deleted)
SELECT json_extract(value, '$.key'), json_extract(value, '$.value'), json_extract(value, '$.stamp'), json_extract(value, '$.deleted')
FROM json_each(?) WHERE true
ON CONFLICT (key) DO UPDATE SET value = excluded.value, stamp = excluded.stamp, deleted = excluded.deleted
WHERE excluded.stamp > entries.stamp`

// TestSync: a last-writer-wins sync over D1. Two clients' entries, merged by the one SQL statement in either order,
// give what merging them in Go gives.
func TestSync(t *testing.T) {
	db := project(t, "SYNC", `CREATE TABLE entries (key TEXT PRIMARY KEY, value TEXT NOT NULL, stamp INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0);`)
	one := []entry{{"a", "one's a", 1, false}, {"b", "one's b", 5, false}, {"c", "", 4, true}}
	two := []entry{{"a", "two's a", 2, false}, {"b", "", 3, true}, {"c", "two's c", 2, false}, {"d", "two's d", 1, false}}
	want := map[string]entry{}
	for _, e := range append(slices.Clone(one), two...) {
		if kept, ok := want[e.Key]; !ok || e.Stamp > kept.Stamp {
			want[e.Key] = e
		}
	}
	for _, order := range [][][]entry{{one, two}, {two, one}} {
		if err := d1.Exec(db, "DELETE FROM entries"); err != nil {
			t.Fatal(err)
		}
		for _, batch := range order {
			b, _ := json.Marshal(batch)
			if err := d1.Exec(db, mergeSQL, string(b)); err != nil {
				t.Fatal(err)
			}
		}
		rows, err := d1.Query[struct {
			Key     string `json:"key"`
			Value   string `json:"value"`
			Stamp   int64  `json:"stamp"`
			Deleted int64  `json:"deleted"`
		}](db, "SELECT key, value, stamp, deleted FROM entries ORDER BY key")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != len(want) {
			t.Fatalf("%d entries, want %d", len(rows), len(want))
		}
		for _, row := range rows {
			if got := (entry{row.Key, row.Value, row.Stamp, row.Deleted != 0}); got != want[row.Key] {
				t.Errorf("%s: SQL kept %+v, Go keeps %+v", row.Key, got, want[row.Key])
			}
		}
	}
}
