package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/internal/command"
)

// A command the docs, the skill or the rules tell a reader to run is one charter has: `go tool charter <command>`,
// and `charter run <task>` for a task of charter's own. One that was renamed or taken away fails here, with where
// it is still written.
func TestTheDocsNameRealCommands(t *testing.T) {
	root := filepath.Join("..", "..")
	var verbs []string
	for _, c := range commands {
		verbs = append(verbs, c.name)
	}
	tasks := command.TaskNames()
	files, _ := filepath.Glob(filepath.Join(root, "site", "docs", "*.md"))
	more, _ := filepath.Glob(filepath.Join(root, "example", "docs", "*.md"))
	files = append(append(files, more...), filepath.Join(root, "skills", "charter", "SKILL.md"), filepath.Join(root, "AGENTS.md"), filepath.Join(root, "README.md"))
	if len(files) < 15 || len(verbs) < 10 || len(tasks) < 20 {
		t.Fatalf("%d files, %d commands, %d tasks: this is not charter's repository", len(files), len(verbs), len(tasks))
	}
	verb := regexp.MustCompile("(?:go tool charter|`charter) ([a-z]+)")
	task := regexp.MustCompile(`charter run ([a-z]+(?::[a-z]+)?)\b`)
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, m := range verb.FindAllStringSubmatch(line, -1) {
				if !slices.Contains(verbs, m[1]) {
					t.Errorf("%s:%d: charter has no command %q:\n\t%s", filepath.Base(file), i+1, m[1], strings.TrimSpace(line))
				}
			}
			for _, m := range task.FindAllStringSubmatch(line, -1) {
				// A task an app declares itself (charter.toml's [run]) is not one of these: the docs name none.
				if !slices.Contains(tasks, m[1]) {
					t.Errorf("%s:%d: charter run has no task %q:\n\t%s", filepath.Base(file), i+1, m[1], strings.TrimSpace(line))
				}
			}
		}
	}
}
