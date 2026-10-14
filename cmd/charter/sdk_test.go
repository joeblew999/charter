package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The projects of this repo, by path from its root: the examples (what a project starts from) and
// the conformance projects (every Fern feature, end to end).
func examples(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, parent := range []string{"examples", "conformance"} {
		dirs, err := projectsBelow(filepath.Join("../..", parent))
		if err != nil {
			t.Fatal(err)
		}
		for _, dir := range dirs {
			names = append(names, parent+"/"+filepath.Base(dir))
		}
	}
	if want := []string{"examples/notes-go", "examples/notes-ts", "examples/start-datastar", "examples/start-go", "examples/start-htmx", "examples/start-ts", "conformance/showcase-go", "conformance/showcase-ts"}; !slices.Equal(names, want) {
		t.Fatalf("the repo has the projects %v, want %v", names, want)
	}
	return names
}

// What marks a project, and that a command finds it from any folder inside it.
func TestAProjectIsAMiseFileBesideAFernFolder(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"repo/one/fern", "repo/one/api/deep", "repo/two"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"repo/mise.toml", "repo/one/mise.toml", "repo/two/mise.toml"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("[tasks.check]\nrun = \"true\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for start, want := range map[string]string{"repo/one": "repo/one", "repo/one/api/deep": "repo/one", "repo/two": "", "repo": ""} {
		got, ok := project(filepath.Join(root, start))
		if want == "" && ok || want != "" && got != filepath.Join(root, want) {
			t.Errorf("from %s: project %q (%v), want %q", start, got, ok, want)
		}
	}
	below, err := projectsBelow(filepath.Join(root, "repo"))
	if err != nil || len(below) != 1 || below[0] != filepath.Join(root, "repo/one") {
		t.Errorf("projects below repo: %v (%v), want only repo/one", below, err)
	}
	if !tasksOf(filepath.Join(root, "repo/one/mise.toml"))["check"] {
		t.Error("tasksOf does not find [tasks.check]")
	}
}

// sdk-ready generates a group again when the Fern folder changed since it was generated (#41).
func TestAGeneratedSDKIsStaleWhenItsSpecsChange(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, dir := range []string{fernDir, "sdk/out/go"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	spec := filepath.Join(fernDir, "openapi.json")
	os.WriteFile(spec, []byte(`{"a":1}`), 0o644)
	if sdkFresh("go") {
		t.Error("fresh without a note of what it was made from")
	}
	source, _ := sdkSourceHash()
	os.WriteFile(sdkStamp("go"), []byte(source+"\n"), 0o644)
	if !sdkFresh("go") {
		t.Error("stale right after it was made")
	}
	os.WriteFile(spec, []byte(`{"a":2}`), 0o644)
	if sdkFresh("go") {
		t.Error("fresh after the spec changed")
	}
}

// sdkCommon are the groups every project's generators.yml defines, in this order.
var sdkCommon = []string{"go", "typescript", "typescript-dist"}

// taskMatches are the tasks a task file defines, as [_, name] pairs (tomlTasks, for the loops here).
func taskMatches(content []byte) [][]string {
	names, _ := tomlTasks(content)
	pairs := make([][]string, len(names))
	for i, name := range names {
		pairs[i] = []string{"", name}
	}
	return pairs
}
