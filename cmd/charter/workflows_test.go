package main

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The workflows a repo gets from the kit, by file name: kitFor chooses them, and copies them as they are.
func kitWorkflows(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files, _, err := kitFor(dir)
	if err != nil {
		t.Fatal(err)
	}
	workflows := map[string][]byte{}
	for path, content := range files {
		if name, ok := strings.CutPrefix(filepath.ToSlash(path), ".github/workflows/"); ok {
			workflows[name] = content
		}
	}
	return workflows
}

// The kinds of repo: a project with a CLI, one without, and a repo with no project at its root.
func repoKinds(t *testing.T) map[string]string {
	t.Helper()
	cli, plain, none := t.TempDir(), t.TempDir(), t.TempDir()
	for file, content := range map[string]string{
		filepath.Join(cli, generators): "groups:\n  go:\n  cli:\n", filepath.Join(cli, "mise.toml"): "",
		filepath.Join(plain, generators): "groups:\n  go:\n", filepath.Join(plain, "mise.toml"): "",
		filepath.Join(none, "mise.toml"): "",
	} {
		if err := write(file, content); err != nil {
			t.Fatal(err)
		}
	}
	return map[string]string{"a project": cli, "a project without a CLI": plain, "a repo with no project": none}
}

// Each kind gets its workflows from the kit, copied as they are: a project its four, and the CLI's
// where it ships one; a repo with no project charter.yml alone.
func TestKitWorkflowsFitTheRepo(t *testing.T) {
	want := map[string][]string{
		"a project":               {"charter-check.yml", "charter-deploy.yml", "charter-release-cli.yml", "charter-release.yml", "charter-sdk-check.yml"},
		"a project without a CLI": {"charter-check.yml", "charter-deploy.yml", "charter-release.yml", "charter-sdk-check.yml"},
		"a repo with no project":  {"charter.yml"},
	}
	for kind, dir := range repoKinds(t) {
		workflows := kitWorkflows(t, dir)
		names := slices.Sorted(maps.Keys(workflows))
		if !slices.Equal(names, want[kind]) {
			t.Errorf("%s gets %v, want %v", kind, names, want[kind])
		}
		for name, content := range workflows {
			if string(content) != kitFile(".github/workflows/"+name) {
				t.Errorf("%s for %s is not the kit's file as it is", name, kind)
			}
		}
	}
}

// Every workflow, the kit's and charter's own: pinned actions and runners, and no logic in YAML (a
// step that does work is one mise task, so it runs the same locally).
func TestWorkflowTemplates(t *testing.T) {
	pinned := regexp.MustCompile(`^[\w.-]+/[\w.-]+@v\d+\.\d+\.\d+$`)
	all := map[string]string{}
	for path, content := range kitFiles(".github") {
		if name, ok := strings.CutPrefix(path, "workflows/"); ok {
			all["kit "+name] = string(content)
		}
	}
	own, _ := filepath.Glob("../../.github/workflows/*.yml")
	for _, path := range own {
		content, _ := os.ReadFile(path)
		all["charter's "+filepath.Base(path)] = string(content)
	}
	if len(all) < 10 {
		t.Fatalf("only %d workflows", len(all))
	}
	for name, workflow := range all {
		file := strings.TrimSuffix(name[strings.LastIndex(name, " ")+1:], ".yml")
		if want := "name: " + strings.TrimPrefix(file, "charter-") + "\n"; file != "charter" && !strings.Contains(workflow, "\n"+want) {
			t.Errorf("%s: no %q", name, want)
		}
		for number, line := range strings.Split(workflow, "\n") {
			line = strings.TrimSpace(line)
			at := func(format string, a ...any) {
				t.Errorf("%s, line %d: "+format, append([]any{name, number + 1}, a...)...)
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
			if strings.HasPrefix(line, "# if-") || line == "# else" || line == "# end" || strings.Contains(line, "__") {
				at("a template marker or placeholder: the kit is copied as it is")
			}
		}
	}
}

// Every task a workflow runs exists where it runs it: the kit's in the examples a project is a copy
// of, charter's own at its root or in the examples its matrix names.
func TestWorkflowsRunTasksThatExist(t *testing.T) {
	step := regexp.MustCompile(`- run: mise run ([a-z][a-z0-9:-]*)`)
	list := regexp.MustCompile(`(example|project): \[([a-z/, -]+)\]`)
	all := examples(t)
	tasks := func(dir string) map[string]bool { return tasksOf(filepath.Join(dir, "mise.toml")) }
	root := tasks("../..")
	for path, content := range kitFiles(".github") {
		name, ok := strings.CutPrefix(path, "workflows/")
		if !ok || name == repoWorkflow {
			continue
		}
		for _, m := range step.FindAllStringSubmatch(string(content), -1) {
			for _, example := range []string{"examples/notes-go", "examples/notes-ts"} {
				if !tasks(filepath.Join("../..", example))[m[1]] {
					t.Errorf("kit %s: mise run %s is not a task of %s", name, m[1], example)
				}
			}
		}
	}
	own, _ := filepath.Glob("../../.github/workflows/*.yml")
	for _, path := range own {
		content, _ := os.ReadFile(path)
		workflow, name := string(content), filepath.Base(path)
		for _, job := range strings.Split(workflow, "\n    runs-on: ")[1:] {
			var where []string // the repo's own tasks
			if m := list.FindStringSubmatch(job); m != nil {
				where = strings.Split(m[2], ", ")
			} else if name == "deploy.yml" {
				if m := list.FindStringSubmatch(strings.ReplaceAll(workflow, "options:", "example:")); m != nil {
					where = strings.Split(m[2], ", ")
				}
			}
			if m := regexp.MustCompile(`working-directory: (examples/[a-z-]+)\n`).FindStringSubmatch(job); where == nil && m != nil {
				where = []string{m[1]}
			}
			for i := range where {
				if !strings.Contains(where[i], "/") {
					where[i] = "examples/" + where[i]
				}
			}
			for _, m := range step.FindAllStringSubmatch(job, -1) {
				if where == nil && !root[m[1]] {
					t.Errorf("charter's %s: mise run %s is not a task of the repo's mise.toml", name, m[1])
				}
				for _, example := range where {
					if !slices.Contains(all, example) {
						t.Errorf("charter's %s: no example %s", name, example)
					} else if !tasks(filepath.Join("../..", example))[m[1]] {
						t.Errorf("charter's %s: mise run %s is not a task of %s", name, m[1], example)
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
// what the Release lacks, and start the Windows CLI on Windows where the project ships a CLI: that is
// charter-release-cli.yml, which a project without the cli group does not get.
func TestWorkflowsCheckTheTag(t *testing.T) {
	check, release, cli := kitFile(".github/workflows/charter-check.yml"), kitFile(".github/workflows/charter-release.yml"), kitFile(".github/workflows/charter-release-cli.yml")
	if !strings.Contains(check, "    tags: [\"v*\"]\n") {
		t.Error("charter-check.yml does not run on a version tag")
	}
	if !strings.Contains(release, "- run: mise run release:publish") || !strings.Contains(release, "- run: mise run release:tags") || strings.Contains(release, "cli") {
		t.Errorf("charter-release.yml: release:publish and release:tags, and nothing of the CLI:\n%s", release)
	}
	for _, want := range []string{"  cli:\n    runs-on: ubuntu-24.04\n", "- run: mise run sdk:dist:cli", "  cli-windows:\n    needs: cli\n    runs-on: windows-2025\n", "- run: mise run sdk:cli:smoke", "    tags: [\"v*\"]\n"} {
		if !strings.Contains(cli, want) {
			t.Errorf("charter-release-cli.yml: no %q", want)
		}
	}
	for name, workflow := range map[string]string{"charter-release.yml": release, "charter-release-cli.yml": cli} {
		if strings.Contains(workflow, "- run: mise run release\n") {
			t.Errorf("%s cuts a release: on a tag it only publishes (mise run release:publish)", name)
		}
	}
}
