package main

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The two kinds of repo the templates serve: one project at the root, and several in examples/.
func repoKinds(t *testing.T) map[string]string {
	t.Helper()
	single, plain, several := t.TempDir(), t.TempDir(), t.TempDir()
	cli := "groups:\n  go:\n  cli:\n"
	for file, content := range map[string]string{
		filepath.Join(single, generators):                          cli,
		filepath.Join(plain, generators):                           "groups:\n  go:\n",
		filepath.Join(several, "examples", "notes-go", generators): cli,
	} {
		if err := write(file, content); err != nil {
			t.Fatal(err)
		}
	}
	return map[string]string{"a project": single, "a project without a CLI": plain, "a repo with examples/": several}
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
				if strings.HasPrefix(line, "# if-dir") || line == "# if-cli" || line == "# else" || line == "# end" {
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
		for _, gone := range []string{"examples", "matrix.project", "charter:check", "go:check", "ts:check", "ts:dist", "charter:release"} {
			if strings.Contains(string(workflow), gone) {
				t.Errorf("%s for a project mentions %q", name, gone)
			}
		}
	}
	for _, want := range []string{"mise run charter:check", "mise run go:check", "mise run ts:check", "working-directory: ${{ matrix.project }}", "mise run check", "  tool-windows:\n    needs: scope\n", "    runs-on: windows-2025\n", "  example-windows:\n", "run: mise run ci:scope", "fromJSON(needs.scope.outputs.examples)"} {
		if !strings.Contains(string(repo["check.yml"]), want) {
			t.Errorf("check.yml for a repo with examples: no %q", want)
		}
	}
	// Both kinds are also checked on Windows.
	if !strings.Contains(string(project["check.yml"]), "  check-windows:\n    runs-on: windows-2025\n") {
		t.Error("check.yml for a project: no check-windows job")
	}
	if !strings.Contains(string(project["check.yml"]), "  check-macos:\n    runs-on: macos-15\n") {
		t.Error("check.yml for a project: no check-macos job")
	}
	if !strings.Contains(string(project["release.yml"]), "mise run release:tags") || !strings.Contains(string(repo["release.yml"]), "mise run charter:release") || !strings.Contains(string(repo["release.yml"]), "mise run ts:dist") {
		t.Error("release.yml: a project tags its modules after the SDKs, this repo after the tool's release, and it packs the TypeScript library")
	}
}

// Every task a workflow runs exists where it runs it: in each example it names (this repo's own
// workflows), or in the example a new project is a copy of.
func TestWorkflowsRunTasksThatExist(t *testing.T) {
	step := regexp.MustCompile(`- run: mise run ([a-z][a-z0-9:-]*)`)
	list := regexp.MustCompile(`(example|project): \[([a-z/, -]+)\]`)
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
				where := []string{"examples/notes-go", "examples/notes-ts"} // what a project is a copy of
				if m := list.FindStringSubmatch(job); m != nil {
					where = strings.Split(m[2], ", ")
					if m[1] == "example" { // names of folders in examples/
						for i := range where {
							where[i] = "examples/" + where[i]
						}
					}
				} else if !strings.HasPrefix(kind, "a project") && !strings.Contains(job, "working-directory") {
					where = nil // the repo's own tasks
				}
				if name == "deploy.yml" && !strings.HasPrefix(kind, "a project") {
					m := list.FindStringSubmatch(strings.ReplaceAll(string(workflow), "options:", "example:"))
					where = strings.Split(m[2], ", ")
					if m[1] == "example" { // names of folders in examples/
						for i := range where {
							where[i] = "examples/" + where[i]
						}
					}
				}
				for _, m := range step.FindAllStringSubmatch(job, -1) {
					if where == nil && !root[m[1]] {
						t.Errorf("%s for %s: mise run %s is not a task of the repo's mise.toml", name, kind, m[1])
					}
					for _, example := range where {
						if !slices.Contains(all, example) {
							t.Errorf("%s for %s: no example %s", name, kind, example)
						} else if !tasks(filepath.Join("../..", example))[m[1]] {
							t.Errorf("%s for %s: mise run %s is not a task of %s", name, kind, m[1], example)
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

// A release is cut on a developer's machine; the workflows check its tag afterwards on every OS, add
// what the Release lacks, and start the Windows CLI on Windows, if the repo has a CLI: a project
// without the cli group builds none.
func TestWorkflowsCheckTheTag(t *testing.T) {
	for kind, dir := range repoKinds(t) {
		workflows, err := workflowsFor(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(workflows["check.yml"]), "    tags: [\"v*\"]\n") {
			t.Errorf("check.yml for %s does not run on a version tag", kind)
		}
		release := string(workflows["release.yml"])
		if !strings.Contains(release, "- run: mise run release:publish") {
			t.Errorf("release.yml for %s: no release:publish", kind)
		}
		for _, want := range []string{"  cli:\n    runs-on: ubuntu-24.04\n", "- run: mise run sdk:dist:cli", "  cli-windows:\n    needs: cli\n    runs-on: windows-2025\n", "- run: mise run sdk:cli:smoke"} {
			if has, cli := strings.Contains(release, want), kind != "a project without a CLI"; has != cli {
				t.Errorf("release.yml for %s: %q there = %v", kind, want, has)
			}
		}
		if kind == "a project without a CLI" && strings.Contains(release, "cli") {
			t.Errorf("release.yml for %s mentions the CLI:\n%s", kind, release)
		}
		if strings.Contains(release, "- run: mise run release\n") {
			t.Errorf("release.yml for %s cuts a release: on a tag it only publishes (mise run release:publish)", kind)
		}
	}
}
