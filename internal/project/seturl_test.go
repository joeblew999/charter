package project

import (
	"os"
	"path/filepath"
	"testing"
)

// An environment's address goes into [urls]: a new table in a file without one, a new line in one that has it.
func TestSetURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "charter.toml")
	os.WriteFile(path, []byte("health = \"/healthz\"\n\n[test]\nrun = [\"go test ./test\"]\n"), 0o644)
	if err := SetURL(dir, "preview", "https://app-preview.example.workers.dev"); err != nil {
		t.Fatal(err)
	}
	if err := SetURL(dir, "staging", "https://staging.example.com"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, Wrangler), []byte(`{"name":"app","main":"build/worker.mjs"}`), 0o644)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.URLs["preview"] != "https://app-preview.example.workers.dev" || p.URLs["staging"] != "https://staging.example.com" || p.Health != "/healthz" || len(p.Test.Run) != 1 {
		b, _ := os.ReadFile(path)
		t.Fatalf("urls = %v\n%s", p.URLs, b)
	}
}
