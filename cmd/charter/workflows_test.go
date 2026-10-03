package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The two kinds of repo the templates serve: one project at the root, and several in examples/.
func repoKinds(t *testing.T) map[string]string {
	t.Helper()
	single, several := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(several, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"a project": single, "a repo with examples/": several}
}

// The rules the templates keep, for either kind of repo: pinned actions and runners, and no logic
// in YAML (a step that does work is one mise task, so it runs the same locally).
func TestWorkflowTemplates(t *testing.T) {
	pinned := regexp.MustCompile(`^[\w.-]+/[\w.-]+@v\d+\.\d+\.\d+$`)
	for kind, dir := range repoKinds(t) {
		workflows, err := workflowsFor(dir)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for name := range workflows {
			names = append(names, name)
		}
		slices.Sort(names)
		if want := []string{"check.yml", "deploy.yml", "release.yml", "sdk-check.yml"}; !slices.Equal(names, want) {
			t.Fatalf("the workflows are %v, want %v", names, want)
		}
		for name, workflow := range workflows {
			if want := "name: " + strings.TrimSuffix(name, ".yml") + "\n"; !strings.Contains(string(workflow), "\n"+want) {
				t.Errorf("%s: no %q", name, want)
			}
			for number, line := range strings.Split(string(workflow), "\n") {
				line = strings.TrimSpace(line)
				at := func(format string, a ...any) {
					t.Errorf("%s for %s, line %d: "+format, append([]any{name, kind, number + 1}, a...)...)
				}
				if run, ok := strings.CutPrefix(line, "- run: "); ok && !strings.HasPrefix(run, "mise run ") {
					at("a step must be `mise run <task>`, not %q", run)
				}
				if line == "run: |" || line == "- run: |" || line == "shell:" {
					at("no shell blocks: add a mise task")
				}
				if uses, ok := strings.CutPrefix(line, "- uses: "); ok && !pinned.MatchString(uses) {
					at("pin the action to an exact version (owner/repo@vX.Y.Z), not %q", uses)
				}
				if runner, ok := strings.CutPrefix(line, "runs-on: "); ok && strings.Contains(runner, "latest") {
					at("pin the runner (ubuntu-24.04), not %q", runner)
				}
				if strings.HasPrefix(line, "# if-dir") || line == "# else" || line == "# end" {
					at("a marker line was written: %q", line)
				}
			}
		}
	}
}

// A project's workflows run its tasks at the root; a repo with examples runs each example's in its
// folder, and has the jobs for the tool and the library.
func TestWorkflowsFitTheRepo(t *testing.T) {
	kinds := repoKinds(t)
	project, err := workflowsFor(kinds["a project"])
	if err != nil {
		t.Fatal(err)
	}
	repo, err := workflowsFor(kinds["a repo with examples/"])
	if err != nil {
		t.Fatal(err)
	}
	for name, workflow := range project {
		for _, gone := range []string{"examples", "matrix.example", "charter:check", "go:check", "ts:check", "charter:release"} {
			if strings.Contains(string(workflow), gone) {
				t.Errorf("%s for a project mentions %q", name, gone)
			}
		}
	}
	for _, want := range []string{"mise run charter:check", "mise run go:check", "mise run ts:check", "working-directory: examples/${{ matrix.example }}", "mise run check", "  tool-windows:\n    runs-on: windows-2025\n", "  example-windows:\n"} {
		if !strings.Contains(string(repo["check.yml"]), want) {
			t.Errorf("check.yml for a repo with examples: no %q", want)
		}
	}
	// Both kinds are also checked on Windows.
	if !strings.Contains(string(project["check.yml"]), "  check-windows:\n    runs-on: windows-2025\n") {
		t.Error("check.yml for a project: no check-windows job")
	}
	if !strings.Contains(string(project["release.yml"]), "mise run release:tags") || !strings.Contains(string(repo["release.yml"]), "mise run charter:release") {
		t.Error("release.yml: a project tags its modules after the SDKs, this repo after the tool's release")
	}
}

// Every task a workflow runs exists where it runs it: in each example it names (this repo's own
// workflows), or in the example a new project is a copy of.
func TestWorkflowsRunTasksThatExist(t *testing.T) {
	step := regexp.MustCompile(`- run: mise run ([a-z][a-z0-9:-]*)`)
	list := regexp.MustCompile(`example: \[([a-z, -]+)\]`)
	all := examples(t)
	kinds := repoKinds(t)
	tasks := func(dir string) map[string]bool { return tasksOf(filepath.Join(dir, "mise.toml")) }
	root := tasks("../..")
	for kind, dir := range kinds {
		workflows, err := workflowsFor(dir)
		if err != nil {
			t.Fatal(err)
		}
		for name, workflow := range workflows {
			// A job's steps run at the root, or in the examples its matrix (or its input) lists.
			for _, job := range strings.Split(string(workflow), "\n    runs-on: ")[1:] {
				where := []string{"notes-go"}
				if m := list.FindStringSubmatch(job); m != nil {
					where = strings.Split(m[1], ", ")
				} else if kind != "a project" && !strings.Contains(job, "working-directory") {
					where = nil // the repo's own tasks
				}
				if name == "deploy.yml" && kind != "a project" {
					where = strings.Split(list.FindStringSubmatch(strings.ReplaceAll(string(workflow), "options:", "example:"))[1], ", ")
				}
				for _, m := range step.FindAllStringSubmatch(job, -1) {
					if where == nil && !root[m[1]] {
						t.Errorf("%s for %s: mise run %s is not a task of the repo's mise.toml", name, kind, m[1])
					}
					for _, example := range where {
						if !slices.Contains(all, example) {
							t.Errorf("%s for %s: no example %s", name, kind, example)
						} else if !tasks(filepath.Join("../../examples", example))[m[1]] {
							t.Errorf("%s for %s: mise run %s is not a task of examples/%s", name, kind, m[1], example)
						}
					}
				}
			}
		}
	}
}

func TestVersionTags(t *testing.T) {
	for tag, ok := range map[string]bool{
		"v1.2.3": true, "v0.0.0-ci-test": true, "v1.2.3-rc.1": true,
		"1.2.3": false, "v1.2": false, "v01.2.3": false, "go/v1.2.3": false, "v1.2.3+build": false, "": false,
	} {
		if version.MatchString(tag) != ok {
			t.Errorf("%q: version tag = %v, want %v", tag, !ok, ok)
		}
	}
}
