package main

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The rule: every file charter writes into a repo says in its first lines that charter wrote it,
// with which command, and whose it is from then on. Held two ways, so a template added later
// without the mark fails here: every template in the tool's folders carries it, and so does every
// file the commands leave in a repo.
func TestEveryFileCharterWritesSaysSo(t *testing.T) {
	// The folders of templates, and the two files in them that are not written into a repo.
	notWritten := []string{"docs/review.md" /* the prompt docs-review hands Claude */, "renovate/charter.json" /* the preset a renovate.json extends, read from GitHub */}
	templates := 0
	for _, folder := range []string{"docs", "github", "workflows", "repoworkflows", "renovate"} {
		err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || slices.Contains(notWritten, filepath.ToSlash(path)) {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			templates++
			if charters, command, ok := markOf(content); !ok || !charters {
				t.Errorf("cmd/charter/%s: no mark in its first %d lines: Written by `charter <command>` ... don't edit (%q, %v)", filepath.ToSlash(path), markLines, command, ok)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if templates < 15 {
		t.Errorf("only %d templates found", templates)
	}
	if charters, command, ok := markOf(renovateConfig("v1.2.3")); !ok || !charters || command != "repo" {
		t.Error("renovate.json has no mark")
	}
	page, err := generatedPageText(t.TempDir(), "docs", generatedPage{Page: "tasks.md", Run: "charter docs-tasks"}, 0, "# Tasks\n\nText.\n")
	if charters, command, ok := markOf([]byte(page)); err != nil || !ok || !charters || command != "docs" {
		t.Errorf("a generated page has no mark (%v):\n%s", err, page)
	}

	// What the commands leave in a repo: adopt, then repo, in a repo that is not a project; and repo
	// in one that is, for the workflows.
	github := &fakeGitHub{labels: map[string][2]string{}}
	gh = github.gh
	t.Cleanup(func() {
		gh = func(string, ...string) (string, error) { return "", errors.New("no gh in tests") }
	})
	root, project := filepath.Join(t.TempDir(), "field-notes"), t.TempDir()
	seeded := map[string]string{"mise.toml": "", "fern/generators.yml": "groups:\n  go:\n", "charter.toml": "description = \"An API\"\n"}
	for file, content := range seeded {
		if err := write(filepath.Join(project, filepath.FromSlash(file)), content); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(root, 0o755)
	if err := adopt(root, "", "v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	// The mise.toml adopt wrote includes charter's tasks at the release v1.2.3, which is not one:
	// mise could not list the tasks for the tasks page. Here the include is this checkout's folder.
	checkout, _ := filepath.Abs(filepath.Join("..", ".."))
	if err := os.WriteFile(filepath.Join(root, "mise.toml"), []byte(startedBy("adopt", "mise.toml", adoptedTasks("field-notes", "", tasksInclude(repoTasksFolder, "", checkout), true))), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, project} {
		if err := keepRepo(dir, false); err != nil {
			t.Fatal(err)
		}
	}
	started := []string{"AGENTS.md", "CLAUDE.md", "charter.toml", "docs/README.md", "docs/rules.md", "mise.toml", "docs/_generated.toml"}
	found := map[string]bool{}
	for _, dir := range []string{root, project} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			rel, _ := filepath.Rel(dir, path)
			rel = filepath.ToSlash(rel)
			if _, mine := seeded[rel]; err != nil || entry.IsDir() || (dir == project && mine) {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			charters, command, ok := markOf(content)
			found[rel] = true
			switch once := dir == root && slices.Contains(started, rel); {
			case !ok:
				t.Errorf("%s, which charter wrote, does not say so in its first %d lines:\n%.300s", rel, markLines, content)
			case once && (charters || command != "adopt"):
				t.Errorf("%s is written once by adopt, and says charter's: %v, by %q", rel, charters, command)
			case !once && !charters:
				t.Errorf("%s is charter's to write again, and says it is the repo's", rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range append(started, ".github/labels.tsv", "renovate.json", "docs/writing.md", "docs/llms.txt", "docs/_config.yml", "docs/_sass/custom/custom.scss", ".github/ISSUE_TEMPLATE/bug.yml", ".github/workflows/repo-check.yml", ".github/workflows/check.yml") {
		if !found[file] {
			t.Errorf("the commands wrote no %s", file)
		}
	}
	// The marks change nothing a file is read for: the labels are the labels, charter.toml parses.
	rows, err := labelRows()
	if err != nil || len(rows) < 10 || slices.ContainsFunc(rows, func(row [3]string) bool { return row[0] == "name" || strings.HasPrefix(row[0], "#") }) {
		t.Errorf("the labels, read past the mark: %v, %v", rows, err)
	}
	if text, _ := os.ReadFile(filepath.Join(root, "charter.toml")); !strings.HasPrefix(string(text), "# Started by `charter adopt` (github.com/joeblew999/charter): yours to edit.\n") {
		t.Errorf("charter.toml starts:\n%s", text)
	}
	if text, _ := os.ReadFile(filepath.Join(root, "docs", "rules.md")); !strings.HasPrefix(string(text), "---\n# Started by `charter adopt` (github.com/joeblew999/charter): yours to edit.\ntitle: Rules\n") {
		t.Errorf("docs/rules.md starts:\n%.200s", text)
	}

	// charter files: every one of them, with whose it is and whether it is what charter writes now.
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(root, "docs", "writing.md"), []byte(docsWriting+"\nMine.\n"), 0o644)
	os.Remove(filepath.Join(root, "renovate.json"))
	write(filepath.Join(root, ".github", "ISSUE_TEMPLATE", "bug.yml"), "name: Our own\n")
	write(filepath.Join(root, "notes.md"), "# Notes\n")
	lines, err := filesReport(root)
	if err != nil {
		t.Fatal(err)
	}
	report := strings.Join(lines, "\n")
	for path, state := range map[string]string{
		".github/labels.tsv": "charter's  matches ", ".github/workflows/repo-check.yml": "charter's  matches ", "docs/_config.yml": "charter's  matches ",
		"docs/writing.md": "charter's  differs ", "renovate.json": "charter's  missing ", "AGENTS.md": "yours      started ", "mise.toml": "yours      started ",
		".github/ISSUE_TEMPLATE/bug.yml": "yours      own form",
	} {
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, state+"  "+path+" ") }) {
			t.Errorf("charter files: no %q for %s in:\n%s", state, path, report)
		}
	}
	if strings.Contains(report, "notes.md") {
		t.Errorf("charter files lists a file with no mark:\n%s", report)
	}
}
