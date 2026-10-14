package main

import (
	"errors"
	"maps"
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
		"charter.toml":            "description = \"field-notes\"\ntopics = []\n",
		"AGENTS.md":               "[docs/charter/rules.md](docs/charter/rules.md)",
		"AGENTS.md ":              "Written by `charter adopt`",
		"AGENTS.md  ":             "\n@docs/README.md\n@docs/charter/rules.md\n@docs/rules.md\n@docs/charter/using.md\n@docs/charter/tasks.md\n",
		"CLAUDE.md":               "@AGENTS.md\n",
		"docs/README.md":          "# field-notes\n",
		"docs/README.md ":         "\n## What is generated\n",
		"docs/rules.md":           "[Rules for every repo](charter/rules.md)",
		"docs/charter/rules.md":   "**A plan is an issue,**",
		"docs/rules.md ":          "[the list](charter/README.md#what-is-charters)",
		"docs/charter/writing.md": "# Writing docs",
		"docs/charter/README.md":  "# How this repo is kept",
		"docs/charter/using.md":   "# Using charter",
		"mise.toml":               "[tools]\n# The tool every task runs, the release's binary. `mise x github:joeblew999/charter@X.Y.Z -- charter upgrade` moves it,\n# and the includes of charter's tasks with it.\n\"github:joeblew999/charter\" = \"1.2.3\"\n",
		"mise.toml ":              "[task_config]\n",
		"mise.toml  ":             "\nincludes = [\"git::https://github.com/joeblew999/charter.git//tasks/repo?ref=v1.2.3\"]\n",
		"mise.toml   ":            "\n[settings]\n",
		"mise.toml    ":           "\nexperimental = true\n",
	} {
		if !strings.Contains(read(strings.TrimSpace(file)), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	for _, api := range []string{"TinyGo", "contract", "mise run spec", "Workers", "fern/", "deploy"} {
		for _, file := range []string{"docs/rules.md", "docs/README.md", "mise.toml"} {
			if strings.Contains(read(file), api) {
				t.Errorf("%s mentions %q, which only an API project has", file, api)
			}
		}
	}
	for _, kit := range []string{".github/workflows/charter.yml", ".github/ISSUE_TEMPLATE/charter-bug.yml", ".github/charter-labels.tsv", "renovate.json", "docs/_config.yml"} {
		if read(kit) != kitFile(kit) {
			t.Errorf("%s is not the kit's, as it is", kit)
		}
	}
	for _, not := range []string{"fern", "README.md", "docs/_generated.toml", "docs/tasks.md", "docs/writing.md", "docs/repo.md", ".github/workflows/charter-check.yml"} {
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
	// AGENTS.md is charter's, the same in every repo: a hand edit is written over.
	pointer := read("AGENTS.md")
	if err := os.WriteFile(filepath.Join(root, "docs/rules.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := adopt(root, "something else", "v1.3.0", ""); err != nil {
		t.Fatal(err)
	}
	if read("AGENTS.md") != pointer {
		t.Error("adopt left a hand-edited AGENTS.md, which is charter's to write")
	}
	if rules := read("docs/rules.md"); !strings.Contains(rules, "## What AGENTS.md said before charter adopt") || !strings.Contains(rules, "\nmine\n") {
		t.Errorf("what the repo's own AGENTS.md said was not kept in docs/rules.md:\n%s", rules)
	}
	before := map[string]string{}
	files := []string{"charter.toml", "AGENTS.md", "CLAUDE.md", "docs/README.md", "docs/rules.md", "docs/charter/writing.md", "mise.toml"}
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
	for _, want := range []string{"under [tools]:\n    \"github:joeblew999/charter\" = \"1.3.0\"", "under [task_config]", "    \"git::https://github.com/joeblew999/charter.git//tasks/repo?ref=v1.3.0\"", "under [settings] (mise takes the include from GitHub only with it):\n    experimental = true"} {
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
	if !strings.Contains(read("charter.toml"), `description = "Billing"`) || !exists(filepath.Join(root, "docs", "llms.txt")) {
		t.Error("with the repo on GitHub: no charter.toml from its description there, or no docs site")
	}
	if len(github.writes) > 0 {
		t.Errorf("adopt changed the repo on GitHub: %v", github.writes)
	}
}

// charter adopt writes charter's own files and nothing of the repo's: in a repo with pages, forms and
// a site of its own at the paths charter writes, none of them change; in a project, and in charter's
// own repo, AGENTS.md, CLAUDE.md and the start pages are the project's. Run again, it changes nothing.
func TestAdoptChangesNothingOfTheRepos(t *testing.T) {
	github := &fakeGitHub{description: "Billing"}
	gh = github.gh
	t.Cleanup(func() { gh = func(string, ...string) (string, error) { return "", errors.New("no gh in tests") } })
	snapshot := func(root string) map[string]string {
		files := map[string]string{}
		filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				content, _ := os.ReadFile(path)
				rel, _ := filepath.Rel(root, path)
				files[filepath.ToSlash(rel)] = string(content)
			}
			return err
		})
		return files
	}
	seed := func(files map[string]string) string {
		root := filepath.Join(t.TempDir(), "billing-api")
		for file, content := range files {
			if err := write(filepath.Join(root, filepath.FromSlash(file)), content); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}

	// A repo with its own docs and .github, at the names charter writes or once wrote.
	own := map[string]string{
		"docs/README.md":                 "# Our docs\n",
		"docs/writing.md":                "# How we write\n",
		"docs/repo.md":                   "# Our repo\n",
		"docs/tasks.md":                  "# Our tasks\n",
		"docs/_config.yml":               "title: our site\n",
		"renovate.json":                  "{}\n",
		".github/ISSUE_TEMPLATE/bug.yml": "name: Bug\n",
		".github/workflows/ci.yml":       "name: ci\n",
		"mise.toml":                      "[tasks.test]\nrun = \"true\"\n",
	}
	root := seed(own)
	if err := adopt(root, "", "v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	after := snapshot(root)
	for file, content := range own {
		if after[file] != content {
			t.Errorf("adopt changed the repo's own %s:\n%s", file, after[file])
		}
	}
	for file := range after {
		_, mine := own[file]
		if !mine && !strings.HasPrefix(file, "docs/charter/") && !strings.HasPrefix(file, ".github/ISSUE_TEMPLATE/charter-") && !slices.Contains([]string{"charter.toml", "AGENTS.md", "CLAUDE.md", "docs/rules.md", "docs/llms.txt", "docs/_sass/custom/custom.scss", ".github/ISSUE_TEMPLATE/config.yml", ".github/charter-labels.tsv", ".github/workflows/charter.yml"}, file) {
			t.Errorf("adopt wrote %s, outside charter's own places", file)
		}
	}
	if err := adopt(root, "", "v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	if again := snapshot(root); !maps.Equal(again, after) {
		t.Error("a second adopt changed something")
	}

	// A project, as charter's own repo is one: what it wrote for itself stays as it is.
	project := map[string]string{
		"go.mod":                 "module " + repoModule + "\n\ngo 1.26\n",
		"charter.toml":           "description = \"Billing\"\nprojects = [\"examples/api\"]\n",
		"examples/api/fern/x":    "",
		"examples/api/mise.toml": "",
		"AGENTS.md":              "# For agents\n\nOur own, with no mark.\n",
		"CLAUDE.md":              "@AGENTS.md\n",
		"docs/README.md":         "---\ntitle: Home\nnav_order: 1\npermalink: /\n---\n\n# Billing\n",
		"docs/rules.md":          "# Our rules\n",
		"mise.toml":              "[task_config]\nincludes = [\"tasks/repo\"]\n",
	}
	root = seed(project)
	if err := adopt(root, "", "v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	after = snapshot(root)
	for file, content := range project {
		if after[file] != content {
			t.Errorf("adopt changed the project's own %s:\n%s", file, after[file])
		}
	}
	for _, not := range []string{"docs/_generated.toml", "docs/tasks.md"} {
		if _, ok := after[not]; ok {
			t.Errorf("adopt wrote %s in a project", not)
		}
	}
	if !strings.Contains(after["docs/charter/README.md"], "# How this repo is kept") {
		t.Error("adopt did not write charter's pages in a project")
	}
}
