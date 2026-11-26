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

// The first deploy's address replaces a new app's placeholder url, and is added to a file without one.
func TestSetTopURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "charter.toml")
	os.WriteFile(path, []byte("# about\nurl = \"https://app.example.workers.dev\"\n\n[pack]\nname = \"App\"\n"), 0o644)
	if err := SetTopURL(dir, "https://app.me.workers.dev"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "# about\nurl = \"https://app.me.workers.dev\"\n\n[pack]\nname = \"App\"\n" {
		t.Fatalf("%s", b)
	}
	os.WriteFile(path, []byte("[pack]\nname = \"App\"\n"), 0o644)
	SetTopURL(dir, "https://app.me.workers.dev")
	if b, _ := os.ReadFile(path); string(b) != "url = \"https://app.me.workers.dev\"\n[pack]\nname = \"App\"\n" {
		t.Fatalf("%s", b)
	}
}
