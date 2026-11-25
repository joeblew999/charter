package dbcode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAsCommitted: for every Go example with queries, what sqlc works out of its schema and queries is written as the
// api/queries.go it commits. It runs sqlc (go run, the first time a download).
func TestAsCommitted(t *testing.T) {
	for _, example := range []string{"notes", "site"} {
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

// TestCommands: a function for each kind of query: :execrows returns the rows the statement changed, and a kind
// charter doesn't write is refused, saying which it does. The code builds off Workers too (no build constraint).
func TestCommands(t *testing.T) {
	request := func(queries string) *Request {
		r := &Request{}
		if err := json.Unmarshal([]byte(`{"queries":[`+queries+`]}`), r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	query := func(name, cmd string) string {
		return `{"name":"` + name + `","cmd":"` + cmd + `","text":"DELETE FROM notes WHERE id < ?","params":[{"number":1,"column":{"name":"id","not_null":true,"type":{"name":"INTEGER"}}}]}`
	}
	got, err := GoQueries(request(query("Trim", ":execrows")+","+query("Clear", ":exec")), "api")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func Trim(db d1.DB, id int64) (int64, error) {\n\treturn d1.ExecRows(db, qTrim, id)\n}",
		"func Clear(db d1.DB, id int64) error {\n\treturn d1.Exec(db, qClear, id)\n}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the code lacks:\n%s\ngot:\n%s", want, got)
		}
	}
	if strings.Contains(got, "//go:build") {
		t.Error("the code has a build constraint: it must build off Workers")
	}
	if _, err := GoQueries(request(query("Copy", ":copyfrom")), "api"); err == nil || !strings.Contains(err.Error(), ":execrows") {
		t.Errorf("a :copyfrom query: %v", err)
	}
}
