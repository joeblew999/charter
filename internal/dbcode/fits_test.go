package dbcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A statement written in Go is found wherever cloud is given SQL as text, under any name the package is imported
// by, and not in generated files, tests' data, or a file that is allowed.
func TestHandWritten(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("api/store.go", "package api\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nfunc f(db cloud.DB) error {\n\treturn cloud.Exec(db, \"DELETE FROM notes\")\n}\n")
	write("api/other.go", "package api\n\nimport c \"github.com/joeblew999/charter/cloud\"\n\nvar s = c.Stmt(\"DELETE FROM notes\")\n\nfunc g(db c.DB) error { return c.Batch(db, s) }\n")
	write("db/queries.gen.go", "package db\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nfunc h(db cloud.DB) error { return cloud.Exec(db, \"DELETE FROM notes\") }\n")
	write("api/admin.go", "package api\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nfunc a(db cloud.DB) error { return cloud.Exec(db, \"VACUUM\") }\n")
	write("api/batch.go", "package api\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nvar typed []cloud.Statement\n\nvar written = cloud.Statement{Query: \"DELETE FROM notes\"}\n")
	write("api/sql.go", "package api\n\nimport _ \"github.com/syumai/workers-go/cloudflare/d1\"\n")
	write("api/plain.go", "package api\n\ntype cloud struct{ Exec int }\n\nvar x = cloud{}.Exec\n")
	write("node_modules/x/x.go", "package x\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nvar s = cloud.Stmt(\"x\")\n")
	got, err := HandWritten(dir, "api/admin.go")
	if err != nil {
		t.Fatal(err)
	}
	if want := "api/batch.go:7: cloud.Statement{…}, api/other.go:5: cloud.Stmt, api/sql.go:3: imports github.com/syumai/workers-go/cloudflare/d1, api/store.go:6: cloud.Exec"; strings.Join(got, ", ") != want {
		t.Errorf("found %q, want %q", strings.Join(got, ", "), want)
	}
}

// charter holds itself to what it holds an app to: no statement in its library is written in Go, but the three of
// the admin screen that ask a database what it has (api/admin.go says why), and cloud itself, whose functions these
// are.
func TestCharterWritesNoSQLInGo(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, folder := range []string{"app", "api", "live", "intl", "seo"} {
		allowed := []string{}
		if folder == "api" {
			allowed = append(allowed, "admin.go")
		}
		got, err := HandWritten(filepath.Join(root, folder), allowed...)
		if err != nil {
			t.Fatal(err)
		}
		for _, one := range got {
			t.Errorf("%s/%s: a statement written in Go. It belongs in a queries.sql (internal/accounts has charter's)", folder, one)
		}
	}
}

// charter's own queries fit a schema that has its tables with more beside them, and do not fit one that has lost
// a column or changed a type. It runs sqlc.
func TestFits(t *testing.T) {
	const tables = "CREATE TABLE users (\n  id TEXT PRIMARY KEY,\n  name TEXT NOT NULL,\n  role TEXT NOT NULL\n);\n"
	const queries = "-- name: Person :one\nSELECT id, name, role FROM users WHERE id = ?;\n"
	app := func(schema string) string {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "db"), 0o755)
		if err := os.WriteFile(filepath.Join(dir, "db", "schema.sql"), []byte(schema), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	more := tables + "CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL);\n" + "ALTER TABLE users ADD COLUMN email TEXT;\n"
	if err := Fits(app(more), "the sign-in screen", tables, queries); err != nil {
		t.Errorf("a schema with charter's tables and more: %v", err)
	}
	gone := strings.Replace(tables, "  role TEXT NOT NULL\n", "  kind TEXT NOT NULL\n", 1)
	if err := Fits(app(gone), "the sign-in screen", tables, queries); err == nil || !strings.Contains(err.Error(), "role") {
		t.Errorf("a schema without the column role: %v", err)
	}
	other := strings.Replace(tables, "name TEXT NOT NULL", "name INTEGER", 1)
	if err := Fits(app(other), "the sign-in screen", tables, queries); err == nil || !strings.Contains(err.Error(), "Person") {
		t.Errorf("a schema whose name is a number that may be null: %v", err)
	}
}
