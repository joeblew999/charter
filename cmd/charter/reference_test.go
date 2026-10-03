package main

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The reference pages are written by hand, so they are held to the code in both directions: the
// docs lint fails on a page that names a command or task that does not exist, and this fails on a
// command or task that exists but has no row in its reference page.

// rows are the names in the first column of a page's tables: | `name` ... |
func rows(t *testing.T, page string) map[string]bool {
	t.Helper()
	content, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^\\| `([^`]+)`").FindAllStringSubmatch(string(content), -1) {
		for _, name := range strings.Split(m[1], "`, `") {
			names[strings.Fields(name)[0]] = true
		}
	}
	// A row may name several: | `api:dev`, `api:spec` ... |
	for _, m := range regexp.MustCompile("(?m)^\\| ([^|]*)\\|").FindAllStringSubmatch(string(content), -1) {
		for _, name := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(m[1], -1) {
			names[strings.Fields(name[1])[0]] = true
		}
	}
	return names
}

func TestEveryCommandIsInTheReference(t *testing.T) {
	documented := rows(t, "../../docs/reference/charter.md")
	for name := range commands {
		if !documented[name] {
			t.Errorf("charter %s has no row in docs/reference/charter.md", name)
		}
	}
}

func TestEveryTaskIsInTheReference(t *testing.T) {
	documented := rows(t, "../../docs/reference/tasks.md")
	for _, file := range []string{"../../mise.toml", "../../examples/notes-go/mise.toml", "../../examples/notes-ts/mise.toml", "../../examples/start-datastar/mise.toml", "../../examples/start-go/mise.toml", "../../examples/start-htmx/mise.toml", "../../examples/start-ts/mise.toml", "../../conformance/showcase-go/mise.toml", "../../conformance/showcase-ts/mise.toml"} {
		var missing []string
		for task := range tasksOf(file) {
			if !documented[task] {
				missing = append(missing, task)
			}
		}
		slices.Sort(missing)
		if len(missing) > 0 {
			t.Errorf("%s: tasks with no row in docs/reference/tasks.md: %s", strings.TrimPrefix(file, "../../"), strings.Join(missing, ", "))
		}
	}
}
