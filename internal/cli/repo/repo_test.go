package repo

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/project"
)

// TestCharterJobs: charter's jobs here are the ones project knows by name, which a repo's own may not take.
func TestCharterJobs(t *testing.T) {
	if got := slices.Sorted(maps.Keys(charterJobs)); !slices.Equal(got, project.CharterJobs) {
		t.Errorf("charter's jobs %v, project.CharterJobs %v: make them the same", got, project.CharterJobs)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// aRepo is a git repo with files of its own, as a person has before charter keeps it.
func aRepo(t *testing.T) string {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").Run(); err != nil {
		t.Skip("no git")
	}
	write(t, dir, "README.md", "# mine\n")
	write(t, dir, "CLAUDE.md", "# Our notes\n\nAlways run the tests.\n")
	write(t, dir, "docs/self/README.md", "---\ntitle: Home\nnav_order: 1\npermalink: /\n---\n\n# Our docs\n\n[Rules](rules.md).\n")
	write(t, dir, ".github/ISSUE_TEMPLATE/bug.yml", "name: Our bug form\ndescription: ours\nbody: []\n")
	write(t, dir, "charter.toml", "[github]\ndescription = \"A repo of ours\"\n")
	return dir
}

// TestInit: charter init writes charter's files, leaves the repo's own alone, moves what its CLAUDE.md said into
// docs/self/rules.md; again it changes nothing, and a file of charter's someone changed is a problem it writes back.
func TestInit(t *testing.T) {
	dir := aRepo(t)
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"docs/charter/README.md", "docs/charter/rules.md", "docs/charter/writing.md", "docs/self/rules.md", "docs/_config.yml", ".github/workflows/charter-repo.yml", ".github/workflows/charter-ci.yml", ".github/ISSUE_TEMPLATE/charter-plan.yml", "AGENTS.md", "renovate.json", "mise.toml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s is not written", f)
		}
	}
	if read(t, dir, "README.md") != "# mine\n" || !strings.Contains(read(t, dir, "docs/self/README.md"), "# Our docs") {
		t.Error("the repo's own files were written over")
	}
	if read(t, dir, ".github/ISSUE_TEMPLATE/bug.yml") != "name: Our bug form\ndescription: ours\nbody: []\n" {
		t.Error("the repo's own form was written over")
	}
	if !strings.Contains(read(t, dir, "docs/self/rules.md"), "Always run the tests.") || !strings.Contains(read(t, dir, "CLAUDE.md"), "Written by `charter init`") {
		t.Error("what CLAUDE.md said did not move into docs/self/rules.md, or charter's did not take its place")
	}
	if problems, err := Problems(dir); err != nil || len(problems) > 0 {
		t.Errorf("after init: %v %v", problems, err)
	}
	page := "docs/charter/rules.md"
	kept := read(t, dir, page)
	write(t, dir, page, kept+"\nA line someone added.\n")
	if problems, _ := Problems(dir); len(problems) != 1 || !strings.HasPrefix(problems[0], page) {
		t.Errorf("a changed page of charter's: %v", problems)
	}
	if err := Init(dir); err != nil || read(t, dir, page) != kept {
		t.Errorf("init did not write it back: %v", err)
	}
}

// TestJobsWorkflows: a job of the repo's own is a workflow of its own, self-<job>.yml, calling charter's runner; one
// that is gone takes its workflow with it; workflows = false writes none.
func TestJobsWorkflows(t *testing.T) {
	dir := aRepo(t)
	write(t, dir, "charter.toml", "[github]\ndescription = \"ours\"\n\n[ci.jobs.lint]\nabout = \"its own\"\non = [\"push\"]\nrun = [\"echo lint\"]\n")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	w := read(t, dir, ".github/workflows/self-lint.yml")
	if !strings.HasPrefix(w, "# Written by `charter init` from charter.toml") || !strings.Contains(w, "uses: ./.github/workflows/charter-ci.yml") {
		t.Errorf("self-lint.yml:\n%s", w)
	}
	repo := read(t, dir, ".github/workflows/charter-repo.yml")
	if !strings.Contains(repo, "permissions:\n      contents: read") {
		t.Errorf("charter-repo.yml:\n%s", repo)
	}
	write(t, dir, "charter.toml", "[github]\ndescription = \"ours\"\nworkflows = false\n")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"self-lint.yml", "charter-repo.yml", "charter-ci.yml"} {
		if _, err := os.Stat(filepath.Join(dir, ".github/workflows", f)); err == nil {
			t.Errorf("%s is still there", f)
		}
	}
}

// TestLint: the lint passes on what init wrote, and names a broken link, a page nothing links to and a command charter
// does not have.
func TestLint(t *testing.T) {
	dir := aRepo(t)
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if problems, _, err := Lint(dir); err != nil || len(problems) > 0 {
		t.Fatalf("lint of a fresh repo: %v %v", problems, err)
	}
	write(t, dir, "docs/self/guide.md", "---\ntitle: Guide\nnav_order: 3\n---\n\n# Guide\n\n[Nowhere](nowhere.md). Run `charter frobnicate`.\n")
	problems, _, _ := Lint(dir)
	joined := strings.Join(problems, "\n")
	for _, want := range []string{"docs/self/guide.md: link to nowhere.md: no such file", "docs/self/guide.md: no other page links to it", "`charter frobnicate`: charter has no such command"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no %q in:\n%s", want, joined)
		}
	}
}

// TestIssueBody: a form's labels are headings, with its hints, and the gh command that files it.
func TestIssueBody(t *testing.T) {
	b, err := files.ReadFile("files/.github/ISSUE_TEMPLATE/charter-plan.yml")
	if err != nil {
		t.Fatal(err)
	}
	body := IssueBody(string(b))
	if !strings.HasPrefix(body, `<!-- Fill this in, then: gh issue create --title "[plan] ..." --label plan,needs-triage,agent-filed`) || !strings.Contains(body, "### ") {
		t.Errorf("body:\n%s", body)
	}
}
