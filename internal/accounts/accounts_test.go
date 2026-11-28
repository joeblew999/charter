package accounts

import (
	"os"
	"testing"

	"github.com/joeblew999/charter/internal/dbcode"
)

// queries.gen.go is what schema.sql and queries.sql give, by the generator that writes an app's: so a statement
// that names a column the tables have not, or binds or returns another type, does not get as far as a Worker. It
// runs sqlc (go run, the first time a download). UPDATE=1 writes the file.
func TestAsCommitted(t *testing.T) {
	r, err := dbcode.AnalyzeFiles(t.TempDir(), "schema.sql", "queries.sql")
	if err != nil {
		t.Fatal(err)
	}
	got, err := dbcode.GoQueriesFrom(r, "accounts", nil,
		"// Code generated from schema.sql and queries.sql, with sqlc, as an app's db/queries.gen.go is. DO NOT EDIT.",
		"// Change those, then UPDATE=1 go test ./internal/accounts.")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE") != "" {
		if err := os.WriteFile("queries.gen.go", []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if committed, _ := os.ReadFile("queries.gen.go"); string(committed) != got {
		t.Errorf("queries.gen.go is not what schema.sql and queries.sql give: UPDATE=1 go test ./internal/accounts\ngot:\n%s", got)
	}
}
