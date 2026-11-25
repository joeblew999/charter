package seo

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestCommands holds everything that describes the commands to commands, the one source of their lines, word for
// word: the template's tasks file has exactly its tasks, in its order, each task's description what it does; its skill
// and its docs page each have a row for every task, | `mise run <task>` | <when> | <does> |; and show-help names each.
// misecheck (template:check-mise) holds the tasks to the Go they run and the skill to the tasks' names and flags.
func TestCommands(t *testing.T) {
	b, err := os.ReadFile("../../templates/.mise/tasks/mise-seo.toml")
	if err != nil {
		t.Fatal(err)
	}
	var tasks []string
	descs := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^\["([^"]+)"\]\ndescription = "([^"]*)"`).FindAllStringSubmatch(string(b), -1) {
		tasks = append(tasks, m[1])
		descs[m[1]] = m[2]
	}
	var ours []string
	for _, c := range commands {
		ours = append(ours, c.task)
		if descs[c.task] != c.does {
			t.Errorf("the tasks file's %s says %q; commands says it %q: make them one", c.task, descs[c.task], c.does)
		}
	}
	if !slices.Equal(tasks, ours) {
		t.Errorf("the tasks file's tasks are %v; commands has %v: the same, in the same order", tasks, ours)
	}
	for _, page := range []string{"../../templates/.claude/skills/seo/SKILL.md", "../../templates/docs/seo/README.md"} {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range commands {
			if row := "| `mise run " + c.task + "` | " + c.when + " | " + c.does + " |"; !strings.Contains(string(b), row) {
				t.Errorf("%s has no row %s", page, row)
			}
		}
	}
	for _, c := range commands {
		if !strings.Contains(help, "mise run "+c.task+" ") {
			t.Errorf("show-help doesn't name %s", c.task)
		}
	}
}
