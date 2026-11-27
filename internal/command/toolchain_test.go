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
		{"1.48.0", "v1.50.0", true}, {"v1.50.0", "1.50.0", false}, {"1.50.0", "v1.50.0", false},
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

// A number is remarked on when it has grown by more than it varies by itself, or is near its limit; not otherwise.
func TestRemark(t *testing.T) {
	ms := func(v float64) string { return millis(v) }
	if got := remark(165, 148, true, startGrew, startLimit, ms); got != "" {
		t.Errorf("a start 11%% over the usual was remarked on: %q", got)
	}
	if got := remark(240, 148, true, startGrew, startLimit, ms); !strings.Contains(got, "up from 148 ms") {
		t.Errorf("a start 62%% over the usual: %q", got)
	}
	if got := remark(850, 800, true, startGrew, startLimit, ms); !strings.Contains(got, "85% of the 1000 ms") {
		t.Errorf("a start near its limit: %q", got)
	}
	if got := remark(7.5e6, 6.7e6, true, sizeGrew, paidSize, megabytes); !strings.Contains(got, "up from 6.70 MB") {
		t.Errorf("a size 12%% over the usual: %q", got)
	}
	if got := remark(240, 0, false, startGrew, startLimit, ms); got != "" {
		t.Errorf("with nothing usual known yet: %q", got)
	}
}
