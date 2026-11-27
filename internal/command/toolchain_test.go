package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Versions are compared part by part, as numbers: 1.27.10 is newer than 1.27.4.
func TestOlder(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"1.27.1", "1.27.4", true}, {"1.27.4", "1.27.4", false}, {"1.27.10", "1.27.4", false},
		{"24.22.3", "26.13.1", true}, {"1.28", "1.27.4", false}, {"1.27", "1.27.1", true},
	} {
		if got := older(c.a, c.b); got != c.want {
			t.Errorf("older(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

// An app's pins are raised to charter's where they are lower or missing, never lowered, and the rest of the file
// (comments, other tools, other tables) is as it was.
func TestAnAppsPinsAreRaised(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mise.toml")
	os.WriteFile(file, []byte(`# the app's tools
[tools]
go = "1.27.1" # old
node = "27.0.0"
jq = "1.8.2"

[settings]
experimental = true
`), 0o644)
	raised, err := raise(file, map[string]string{"go": "1.27.4", "node": "26.13.1", "github:jdx/fnox": "1.50.0"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(raised, ", ") != "go 1.27.1 → 1.27.4, github:jdx/fnox 1.50.0" {
		t.Errorf("raised: %v", raised)
	}
	got, _ := miseTools(file)
	if got["go"] != "1.27.4" || got["node"] != "27.0.0" || got["jq"] != "1.8.2" || got["github:jdx/fnox"] != "1.50.0" {
		t.Errorf("tools now: %v", got)
	}
	b, _ := os.ReadFile(file)
	for _, kept := range []string{"# the app's tools", `go = "1.27.4" # old`, "[settings]\nexperimental = true", `"github:jdx/fnox" = "1.50.0"`} {
		if !strings.Contains(string(b), kept) {
			t.Errorf("the file lost %q:\n%s", kept, b)
		}
	}
	// The new pin is in [tools], not in the table after it.
	if strings.Index(string(b), "fnox") > strings.Index(string(b), "[settings]") {
		t.Errorf("fnox went below [settings]:\n%s", b)
	}
	if again, _ := raise(file, map[string]string{"go": "1.27.4"}); len(again) != 0 {
		t.Errorf("raised again: %v", again)
	}
}
