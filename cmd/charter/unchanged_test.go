package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The generator unchanged runs in the test: this test binary, which writes what CHARTER_WRITE says
// (name=content, comma-separated; no content removes the file) and exits.
func init() {
	if spec, ok := os.LookupEnv("CHARTER_WRITE"); ok {
		for _, file := range strings.Split(spec, ",") {
			name, content, _ := strings.Cut(file, "=")
			if content == "" {
				os.Remove(name)
			} else {
				os.WriteFile(name, []byte(content), 0o644)
			}
		}
		os.Exit(0)
	}
}

func TestUnchangedFailsWhenTheGeneratorChangesACommittedFile(t *testing.T) {
	generator, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ writes, names string }{
		{"a.x.go=a,c.go=new,node_modules/d.x.go=new", ""}, // the same, or not a generated file
		{"a.x.go=new", "a.x.go\n"},
		{"e.x.go=new", "e.x.go (new)"},
		{"b.x.go=", "b.x.go (removed)"},
	} {
		t.Chdir(t.TempDir())
		for name, content := range map[string]string{"a.x.go": "a", "b.x.go": "b", "c.go": "c", "node_modules/d.x.go": "d"} {
			if err := write(name, content); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("CHARTER_WRITE", c.writes)
		err := unchanged([]string{"-files", "*.x.go", generator})
		switch {
		case c.names == "" && err != nil:
			t.Errorf("%s: %v", c.writes, err)
		case c.names != "" && (err == nil || !strings.Contains(err.Error()+"\n", c.names)):
			t.Errorf("%s: %v, want it to name %q", c.writes, err, c.names)
		}
	}
}
