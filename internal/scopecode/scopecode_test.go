package scopecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A scope made where it is declared (a package's variable, alone or in a group) is as it should be. One made
// anywhere else is found: in a function, in another value, under any name the package is imported by.
func TestLint(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("api/scopes.go", "package api\n\nimport charter \"github.com/joeblew999/charter/api\"\n\nvar Read = charter.NewScope(\"read\")\n\nvar (\n\tWrite    = charter.NewScope(\"write\")\n\tModerate = charter.NewScope(\"moderate\")\n)\n")
	write("pages/pages.go", "package pages\n\nimport charter \"github.com/joeblew999/charter/api\"\n\ntype Post struct{}\n\nfunc (Post) Needs() []charter.Scope { return []charter.Scope{charter.NewScope(\"wrtie\")} }\n")
	write("pages/roles.go", "package pages\n\nimport \"github.com/joeblew999/charter/app\"\n\nvar roles = map[string][]app.Scope{\"member\": {app.NewScope(\"read\")}}\n")
	write("pages/other.go", "package pages\n\ntype charter struct{}\n\nfunc (charter) NewScope(string) int { return 0 }\n\nvar x = charter{}.NewScope(\"read\")\n")
	got, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := "pages/pages.go:7: charter.NewScope\npages/roles.go:5: app.NewScope"; strings.Join(got, "\n") != want {
		t.Errorf("found:\n%s\nwant:\n%s", strings.Join(got, "\n"), want)
	}
}
