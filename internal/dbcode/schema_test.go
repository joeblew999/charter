package dbcode

import (
	"strings"
	"testing"
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
	for _, want := range []string{"CREATE TABLE notes_new", "INSERT INTO notes_new (id, body) SELECT id, body FROM notes;", "DROP TABLE notes;", "ALTER TABLE notes_new RENAME TO notes;"} {
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
	if !strings.Contains(sql, "INSERT INTO notes_new (id, text) SELECT id, body FROM notes;") || len(notes) != 0 {
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
