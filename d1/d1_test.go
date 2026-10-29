//go:build !(js && wasm)

package d1_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/joeblew999/charter/d1"
)

// TestOffWorkers: off Workers a binding is SQLite in memory, migrated as charter.toml says, and the calls behave as on
// D1: rows as JSON objects, RETURNING, no rows, one database shared by every Open.
func TestOffWorkers(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"charter.toml":        "name = \"t\"\ncompatibility_date = \"2026-09-11\"\n[[d1]]\nbinding = \"DB\"\nmigrations = \"migrations\"\n",
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
		t.Error("a binding charter.toml doesn't have opened")
	}
}
