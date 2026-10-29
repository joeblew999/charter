package dbcode

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestTablesAsCommitted: for every TypeScript example with a database, the tables its migrations make are written as
// the api/tables.ts it commits.
func TestTablesAsCommitted(t *testing.T) {
	for _, example := range []string{"notes-ts", "conformance-ts"} {
		t.Run(example, func(t *testing.T) {
			dir := filepath.Join("..", "..", "examples", example)
			committed, err := os.ReadFile(filepath.Join(dir, "api", "tables.ts"))
			if err != nil {
				t.Fatal(err)
			}
			ms, err := migrations(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := DrizzleTables(ms); err != nil || got != string(committed) {
				t.Errorf("api/tables.ts differs from the committed one (%v)\ngot:\n%s\nwant:\n%s", err, got, committed)
			}
		})
	}
}

func TestDrizzleTables(t *testing.T) {
	ts, err := DrizzleTables([]string{
		"CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT, created_at TEXT DEFAULT CURRENT_TIMESTAMP);",
		"CREATE TABLE tags (note_id INTEGER NOT NULL, name TEXT NOT NULL UNIQUE, weight REAL, PRIMARY KEY (note_id, name));",
		// A later migration changes a table: what counts is the schema once all ran.
		"CREATE TABLE notes_new (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP); DROP TABLE notes; ALTER TABLE notes_new RENAME TO notes;",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"^// Written by `charter spec`",
		`import \{ integer, primaryKey, real, sqliteTable, text \} from "drizzle-orm/sqlite-core";`,
		"export const notes = sqliteTable\\(\"notes\", \\{\n\tid: integer\\(\"id\"\\)\\.primaryKey\\(\\{ autoIncrement: true \\}\\),\n\tbody: text\\(\"body\"\\)\\.notNull\\(\\),\n\tcreated_at: text\\(\"created_at\"\\)\\.notNull\\(\\)\\.default\\(sql`CURRENT_TIMESTAMP`\\),\n\\}\\);",
		`\tname: text\("name"\)\.notNull\(\)\.unique\(\),`,
		`\}, table => \[primaryKey\(\{ columns: \[table\["note_id"\], table\["name"\]\] \}\)\]\);`,
	} {
		if !regexp.MustCompile(want).MatchString(ts) {
			t.Errorf("no %s in:\n%s", want, ts)
		}
	}
}
