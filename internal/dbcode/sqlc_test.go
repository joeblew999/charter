package dbcode

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAsCommitted: for every Go example with queries, what sqlc works out of its schema and queries is written as the
// api/queries.go it commits. It runs
// sqlc, as mise loads it (skipped without mise).
func TestAsCommitted(t *testing.T) {
	if _, err := exec.LookPath("mise"); err != nil {
		t.Skip("no mise to load sqlc")
	}
	for _, example := range []string{"notes-go", "conformance-go", "start-htmx", "start-datastar"} {
		t.Run(example, func(t *testing.T) {
			dir := filepath.Join("..", "..", "examples", example)
			committed, err := os.ReadFile(filepath.Join(dir, "api", "queries.go"))
			if err != nil {
				t.Fatal(err)
			}
			r, err := Analyze(dir)
			if err != nil {
				t.Fatal(err)
			}
			got, err := GoQueries(r, "api")
			if err != nil {
				t.Fatal(err)
			}
			if want := string(committed); got != want {
				t.Errorf("api/queries.go differs from the committed one\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestNames: Go's names for SQL's.
func TestNames(t *testing.T) {
	for in, want := range map[string]string{"created_at": "CreatedAt", "user_id": "UserID", "html_body": "HTMLBody"} {
		if got := exported(in); got != want {
			t.Errorf("exported(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"created_at": "createdAt", "id": "id", "type": "type_", "": "arg"} {
		if got := local(in); got != want {
			t.Errorf("local(%q) = %q, want %q", in, got, want)
		}
	}
}
