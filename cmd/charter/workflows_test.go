package main

import (
	"maps"
	"path/filepath"
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
