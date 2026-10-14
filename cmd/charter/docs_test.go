package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A repo whose product is one task file, which other repos include from git, includes that file
// by name itself. The tasks in it are tasks a page may name, as those in an included folder are.
func TestTasksOfAnIncludedFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mise.toml", "[task_config]\nincludes = [\"tasks.toml\", \"more\"]\n\n[tasks.test]\nrun = \"true\"\n")
	write("tasks.toml", "[\"site:new\"]\nrun = \"true\"\n\n[emdash]\nrun = \"true\"\n")
	write("more/extra.toml", "[\"docs:lint\"]\nrun = \"true\"\n")
	tasks := tasksOf(filepath.Join(dir, "mise.toml"))
	for _, name := range []string{"test", "site:new", "emdash", "docs:lint"} {
		if !tasks[name] {
			t.Errorf("tasksOf does not find %s: %v", name, tasks)
		}
	}
}
