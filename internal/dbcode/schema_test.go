package dbcode

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const notesV1 = "CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL);\n"

func diff(t *testing.T, from, to string) (string, string, []string) {
	t.Helper()
	statements, name, notes, err := Diff(from, to)
	if err != nil {
		t.Fatal(err)
	}
	// What was worked out does what it says: the migrations so far and these make the schema, exactly.
	if again, _, _, err := Diff(from+strings.Join(statements, "\n"), to); err != nil || len(again) != 0 {
		t.Fatalf("after the migration the schema still differs: %v %v\n%s", again, err, strings.Join(statements, "\n"))
	}
	return strings.Join(statements, "\n"), name, notes
}

func TestNothingToDo(t *testing.T) {
	if sql, _, _ := diff(t, notesV1, "CREATE TABLE notes (\n  id INTEGER PRIMARY KEY AUTOINCREMENT, -- example: 42\n  body TEXT NOT NULL\n);"); sql != "" {
		t.Fatalf("comments and spacing are no change:\n%s", sql)
	}
}

func TestANewTableAndIndex(t *testing.T) {
	sql, name, _ := diff(t, notesV1, notesV1+"CREATE TABLE tags (id INTEGER PRIMARY KEY, name TEXT NOT NULL);\nCREATE INDEX tags_name ON tags (name);")
	if !strings.Contains(sql, "CREATE TABLE tags") || !strings.Contains(sql, "CREATE INDEX tags_name") || name != "create_tags" {
		t.Fatalf("%s: %s", name, sql)
	}
}

func TestAColumnAddedInPlace(t *testing.T) {
	sql, name, _ := diff(t, notesV1, "CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL, pinned INTEGER NOT NULL DEFAULT 0);")
	if sql != "ALTER TABLE notes ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;" || name != "notes_add_pinned" {
		t.Fatalf("%s: %s", name, sql)
	}
}

// What SQLite cannot add in place (a default that is worked out), and a column gone: the table is made again.
func TestTheTableMadeAgain(t *testing.T) {
	sql, name, notes := diff(t, "CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL, old TEXT);",
		"CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);")
	for _, want := range []string{"CREATE TABLE notes_rows AS SELECT * FROM notes;", "DROP TABLE notes;", "INSERT INTO notes (id, body) SELECT id, body FROM notes_rows;", "DROP TABLE notes_rows;"} {
		if !strings.Contains(sql, want) {
			t.Errorf("no %q in:\n%s", want, sql)
		}
	}
	if name != "change_notes" || len(notes) != 1 || !strings.Contains(notes[0], "notes.old is dropped") {
		t.Errorf("%s %v", name, notes)
	}
}

// A renamed column says what it was, and keeps its rows' values.
func TestARenamedColumnKeepsItsData(t *testing.T) {
	sql, _, notes := diff(t, notesV1, "CREATE TABLE notes (\n  id INTEGER PRIMARY KEY AUTOINCREMENT,\n  text TEXT NOT NULL -- was: body\n);")
	if !strings.Contains(sql, "INSERT INTO notes (id, text) SELECT id, body FROM notes_rows;") || len(notes) != 0 {
		t.Fatalf("%v\n%s", notes, sql)
	}
}

func TestANewRequiredColumnNeedsADefault(t *testing.T) {
	_, _, _, err := Diff("CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT, old TEXT);", "CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT, owner TEXT NOT NULL);")
	if err == nil || !strings.Contains(err.Error(), "notes.owner is NOT NULL without a DEFAULT") {
		t.Fatalf("%v", err)
	}
}

func TestTagsFromTheSchemasComments(t *testing.T) {
	tags := Tags("CREATE TABLE notes (\n  id INTEGER PRIMARY KEY, -- example: 42\n  body TEXT NOT NULL, -- example: \"Buy milk\"; doc: What it says\n  plain TEXT\n);")
	if tags["notes"]["id"] != `example:"42"` || tags["notes"]["body"] != `example:"Buy milk" doc:"What it says"` || tags["notes"]["plain"] != "" {
		t.Fatalf("%v", tags)
	}
}

// A view is dropped and made again when its text changes, and around a table that is made again.
func TestAViewFollowsItsText(t *testing.T) {
	from := notesV1 + "CREATE VIEW recent AS SELECT id FROM notes;"
	sql, _, _ := diff(t, from, notesV1+"CREATE VIEW recent AS SELECT id, body FROM notes;")
	if !strings.HasPrefix(sql, "DROP VIEW recent;") || !strings.HasSuffix(sql, "CREATE VIEW recent AS SELECT id, body FROM notes;") {
		t.Fatalf("%s", sql)
	}
	if sql, _, _ := diff(t, from, from); sql != "" {
		t.Fatalf("no change, but:\n%s", sql)
	}
}

// A table that is made again has its views taken down first and put back after.
func TestATableMadeAgainUnderAView(t *testing.T) {
	view := "CREATE VIEW recent AS SELECT id FROM notes;"
	sql, _, _ := diff(t, notesV1+view, "CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);"+view)
	if !strings.HasPrefix(sql, "DROP VIEW recent;") || !strings.HasSuffix(sql, view) || !strings.Contains(sql, "DROP TABLE notes_rows;") {
		t.Fatalf("%s", sql)
	}
}

// A table that other tables' rows point at is made again with those rows there, and the database still holds:
// D1 checks foreign keys when a migration ends (they are deferred, not off), so the rows must be back under the
// table's own name by then. The first form of this, a new table given the name afterwards, failed on the first
// database that had people in it.
func TestATableMadeAgainUnderRowsThatPointAtIt(t *testing.T) {
	from := "CREATE TABLE users (id TEXT PRIMARY KEY, scopes TEXT NOT NULL);\nCREATE TABLE tokens (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users (id));\n"
	to := "CREATE TABLE users (\n  id TEXT PRIMARY KEY,\n  role TEXT NOT NULL -- was: scopes\n);\nCREATE TABLE tokens (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users (id));\n"
	migration, _, _ := diff(t, from, to)
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, st := range []string{from, "INSERT INTO users VALUES ('u1', 'read write');", "INSERT INTO tokens (user_id) VALUES ('u1');"} {
		if _, err := db.Exec(st); err != nil {
			t.Fatal(err)
		}
	}
	// As D1 runs a migration: one transaction, foreign keys on.
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(migration); err != nil {
		t.Fatalf("%v\n%s", err, migration)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("the migration does not hold with rows that point at the table: %v\n%s", err, migration)
	}
	var role string
	if err := db.QueryRow("SELECT role FROM users JOIN tokens ON tokens.user_id = users.id").Scan(&role); err != nil || role != "read write" {
		t.Errorf("the rows after: %q %v", role, err)
	}
}
