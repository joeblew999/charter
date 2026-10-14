package main

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The flow every repo follows (tasks/repo/tasks.toml, in order), run as a person runs it, on a throwaway repo
// with files of its own at the names charter uses: charter adopt, then mise run adopt and
// mise run docs:check with the tool built from this checkout. Every step passes, the repo's own files
// do not change, and running it again changes nothing.
func TestTheFlow(t *testing.T) {
	if _, err := exec.LookPath("mise"); err != nil {
		t.Skip("the flow needs mise")
	}
	gh = func(string, ...string) (string, error) { return "", errors.New("not on GitHub") }
	t.Cleanup(func() { gh = func(string, ...string) (string, error) { return "", errors.New("no gh in tests") } })
	checkout, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if out, err := exec.Command("go", "build", "-o", filepath.Join(bin, exe("charter")), ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	root := t.TempDir()
	own := map[string]string{
		"README.md":                      "# A repo\n",
		"docs/README.md":                 "---\ntitle: Home\nnav_order: 1\npermalink: /\n---\n\n# Our docs\n\n[Our guide](guide.md). [How we write](writing.md). [Rules](rules.md).\n",
		"docs/guide.md":                  "---\ntitle: Guide\nnav_order: 2\n---\n\n# Our guide\n",
		"docs/writing.md":                "---\ntitle: How we write\nnav_order: 3\n---\n\n# How we write\n",
		".github/ISSUE_TEMPLATE/bug.yml": "name: Our bug form\n",
		".github/workflows/ci.yml":       "name: ci\n",
		"renovate.json":                  "{}\n",
		"CLAUDE.md":                      "# Our notes\n\nAlways run the tests.\n",
	}
	for file, content := range own {
		if err := write(filepath.Join(root, filepath.FromSlash(file)), content); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	env := append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "MISE_TRUSTED_CONFIG_PATHS="+root)
	step := func(program string, args ...string) {
		t.Helper()
		cmd := exec.Command(program, args...)
		cmd.Dir, cmd.Env = root, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %s: %v\n%s", program, strings.Join(args, " "), err, out)
		}
	}
	flow := func() {
		t.Helper()
		step("charter", "adopt", "-from", checkout)
		step("mise", "run", "adopt")
		step("mise", "run", "docs:check")
	}

	flow()
	for file, content := range own {
		if file == "CLAUDE.md" {
			continue // the kit's takes its place; what it said moved to docs/rules.md
		}
		if have, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(file))); string(have) != content {
			t.Errorf("the flow changed the repo's own %s:\n%s", file, have)
		}
	}
	if rules, _ := os.ReadFile(filepath.Join(root, "docs", "rules.md")); !strings.Contains(string(rules), "Always run the tests.") {
		t.Errorf("what the repo's CLAUDE.md said is not in docs/rules.md:\n%s", rules)
	}

	before := filesBelow(t, root)
	flow()
	if after := filesBelow(t, root); !maps.Equal(before, after) {
		for file := range after {
			if before[file] != after[file] {
				t.Errorf("the flow run again changed %s", file)
			}
		}
	}
}

// charter new makes a project that compiles: its Go code, against this checkout's library.
func TestNewProjectCompiles(t *testing.T) {
	checkout, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	into := filepath.Join(t.TempDir(), "billing-api")
	if err := newProject([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-into", into, "-from", checkout}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = into
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet in the new project: %v\n%s", err, out)
	}
}

// filesBelow is every file below root but .git, by its path.
func filesBelow(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			content, err := os.ReadFile(path)
			rel, _ := filepath.Rel(root, path)
			files[filepath.ToSlash(rel)] = string(content)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
