package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// charter adopt in a repo with nothing: it writes the pages every repo shares, with none of an API
// project's rules; a mise.toml that pins the tool and includes tasks/repo at its release; and no
// workflows, issue forms or Fern folder. Run again it changes nothing, and what the repo wrote
// itself is kept. Without the repo on GitHub the docs site's config waits; with it, it is written.
func TestAdopt(t *testing.T) {
	noGitHub := func(string, ...string) (string, error) { return "", errors.New("no gh in tests") }
	gh = noGitHub
	t.Cleanup(func() { gh = noGitHub })
	root := filepath.Join(t.TempDir(), "field-notes")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	read := func(file string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	if err := adopt(root, "", "v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{
		"charter.toml":    "description = \"field-notes\"\ntopics = []\n",
		"AGENTS.md":       "[docs/rules.md](docs/rules.md)",
		"CLAUDE.md":       "@AGENTS.md\n",
		"docs/README.md":  "# field-notes\n",
		"docs/README.md ": "\n## What is generated\n",
		"docs/rules.md":   "**Plans are issues,**",
		"docs/rules.md ":  "[the list](README.md#what-is-generated)",
		"docs/writing.md": "# Writing docs",
		"mise.toml":       "[tools]\n# The tool every task runs, the release's binary: `mise up --bump github:joeblew999/charter` moves to a newer one.\n\"github:joeblew999/charter\" = \"1.2.3\"\n",
		"mise.toml ":      "[task_config]\n",
		"mise.toml  ":     "\nincludes = [\"git::https://github.com/joeblew999/charter.git//tasks/repo?ref=v1.2.3\"]\n",
	} {
		if !strings.Contains(read(strings.TrimSpace(file)), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	for _, api := range []string{"TinyGo", "contract", "mise run spec", "Workers", "fern/", "deploy"} {
		for _, file := range []string{"docs/rules.md", "docs/README.md", "AGENTS.md", "mise.toml"} {
			if strings.Contains(read(file), api) {
				t.Errorf("%s mentions %q, which only an API project has", file, api)
			}
		}
	}
	for _, not := range []string{".github", "fern", "renovate.json", "docs/_config.yml", "README.md"} {
		if exists(filepath.Join(root, not)) {
			t.Errorf("adopt wrote %s", not)
		}
	}
	if _, err := parseCharterToml(read("charter.toml")); err != nil {
		t.Errorf("charter.toml: %v", err)
	}
	if isProject(root) {
		t.Error("an adopted repo is a project")
	}

	// Again: nothing changes, and what the repo wrote is kept (a release moved on, too).
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{}
	files := []string{"charter.toml", "AGENTS.md", "CLAUDE.md", "docs/README.md", "docs/rules.md", "docs/writing.md", "mise.toml"}
	for _, file := range files {
		before[file] = read(file)
	}
	if err := adopt(root, "something else", "v1.3.0", ""); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if read(file) != before[file] {
			t.Errorf("a second adopt changed %s", file)
		}
	}

	// A mise.toml the repo wrote: the lines it lacks, and none once it has them, at any release.
	pin, include := toolPin("v1.3.0"), tasksInclude(repoTasksFolder, "v1.3.0", "")
	if add := linesToAdd(read("mise.toml"), pin, include, false); len(add) > 0 {
		t.Errorf("the mise.toml adopt wrote lacks %q", add)
	}
	add := strings.Join(linesToAdd("[tools]\nnode = \"26\"\n\n[task_config]\nincludes = [\"tasks.toml\"]\n", pin, include, false), "\n")
	for _, want := range []string{"under [tools]:\n    \"github:joeblew999/charter\" = \"1.3.0\"", "under [task_config]", "    \"git::https://github.com/joeblew999/charter.git//tasks/repo?ref=v1.3.0\""} {
		if !strings.Contains(add, want) {
			t.Errorf("the lines to add: no %q in %q", want, add)
		}
	}

	// From a checkout: the tasks come from its folder, nothing is pinned, and the pages pass the lint
	// with the tasks they name.
	checkout, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(t.TempDir(), "field-notes")
	os.MkdirAll(root, 0o755)
	if err := adopt(root, "Notes from the field", "", checkout); err != nil {
		t.Fatal(err)
	}
	if tasks := read("mise.toml"); !strings.Contains(tasks, `includes = ["`+filepath.ToSlash(checkout)+`/tasks/repo"]`) || strings.Contains(tasks, "[tools]") {
		t.Errorf("mise.toml from a checkout:\n%s", tasks)
	}
	if !strings.Contains(read("charter.toml"), `description = "Notes from the field"`) {
		t.Error("charter.toml does not have -description")
	}
	offered := tasksOf(filepath.Join(root, "mise.toml"))
	for _, task := range repoTasks {
		if !offered[task] {
			t.Errorf("the adopted repo has no task %s", task)
		}
	}
	for task := range offered {
		if !slices.Contains(repoTasks, task) {
			t.Errorf("the adopted repo has the task %s, which is not one any repo takes", task)
		}
	}
	if err := docsLint([]string{"-into", root}); err != nil {
		t.Errorf("docs-lint on what adopt wrote: %v", err)
	}

	// With the repo on GitHub: what charter docs writes, from its name and description there.
	github := &fakeGitHub{description: "Billing"}
	gh = github.gh
	root = filepath.Join(t.TempDir(), "billing-api")
	os.MkdirAll(root, 0o755)
	if err := adopt(root, "", "v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read("charter.toml"), `description = "Billing"`) || !strings.Contains(read("docs/_config.yml"), `description: "Billing"`) || !exists(filepath.Join(root, "docs", "llms.txt")) {
		t.Error("with the repo on GitHub: no docs site from its description there")
	}
	if len(github.writes) > 0 {
		t.Errorf("adopt changed the repo on GitHub: %v", github.writes)
	}
}
