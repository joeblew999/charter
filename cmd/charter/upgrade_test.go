package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// upgrade moves the pin and every include together, adds what mise needs to take the includes, moves
// the TypeScript library, and writes charter's pointer files again only from the release it names.
func TestUpgradeMovesEverythingTogether(t *testing.T) {
	root := t.TempDir()
	put := func(name, content string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		content, _ := os.ReadFile(filepath.Join(root, name))
		return string(content)
	}
	put("mise.toml", "[tools]\n\"github:joeblew999/charter\" = \"1.2.3\"\n\n[task_config]\nincludes = [\"git::https://github.com/joeblew999/charter.git//tasks/repo?ref=v1.2.3\", \"git::https://github.com/joeblew999/charter.git//tasks/ts?ref=v1.2.3\", \"tasks.toml\"]\n")
	put("package.json", `{"dependencies": {"@charter/ts": "https://github.com/joeblew999/charter/releases/download/v1.2.3/charter-ts-1.2.3.tgz"}}`)
	put("AGENTS.md", "<!-- Written by `charter repo` (github.com/joeblew999/charter): don't edit. -->\n\nan old text\n")
	put("CLAUDE.md", "mine, not charter's\n")
	null, _ := os.Open(os.DevNull)
	defer null.Close()

	// From another release: the pins move, and the pointer files wait for that release to write them.
	if err := upgrade(root, "v1.3.0", false, null); err != nil {
		t.Fatal(err)
	}
	tasks := read("mise.toml")
	for _, want := range []string{`"github:joeblew999/charter" = "1.3.0"`, "tasks/repo?ref=v1.3.0", "tasks/ts?ref=v1.3.0", `"tasks.toml"`, "\nexperimental = true\n"} {
		if !strings.Contains(tasks, want) {
			t.Errorf("mise.toml: no %q in\n%s", want, tasks)
		}
	}
	if strings.Contains(tasks, "1.2.3") {
		t.Errorf("mise.toml still names 1.2.3:\n%s", tasks)
	}
	if !strings.Contains(read("package.json"), "/releases/download/v1.3.0/charter-ts-1.3.0.tgz") {
		t.Errorf("package.json: @charter/ts did not move: %s", read("package.json"))
	}
	if !strings.Contains(read("AGENTS.md"), "an old text") {
		t.Error("AGENTS.md was written by a release other than the one named")
	}

	// From the release itself: charter's pointer file is written again, a file of the repo's is not.
	if err := upgrade(root, "v1.3.0", true, null); err != nil {
		t.Fatal(err)
	}
	if read("AGENTS.md") != string(agentFiles()["AGENTS.md"]) {
		t.Errorf("AGENTS.md is not as this release writes it:\n%s", read("AGENTS.md"))
	}
	if read("CLAUDE.md") != "mine, not charter's\n" {
		t.Error("upgrade wrote over a CLAUDE.md without charter's mark")
	}
	// Again: nothing changes.
	before := read("mise.toml")
	if err := upgrade(root, "v1.3.0", true, null); err != nil || read("mise.toml") != before {
		t.Errorf("a second upgrade changed mise.toml (%v)", err)
	}
}

// A release whose upload stopped part way has checksums.txt and not every archive it names: the
// tool is not there yet, and publishing again must not be skipped.
func TestReleaseHasTheToolOnlyWhenEveryArchiveIsThere(t *testing.T) {
	sums := "abc  charter_darwin_arm64.tar.gz\ndef  charter_linux_amd64.tar.gz\n"
	all := []asset{{Name: "checksums.txt"}, {Name: "charter_darwin_arm64.tar.gz"}, {Name: "charter_linux_amd64.tar.gz"}, {Name: "charter-ts-1.2.3.tgz"}}
	if !hasEveryArchive(sums, all) {
		t.Error("every archive is there, and it says not")
	}
	if hasEveryArchive(sums, all[:2]) {
		t.Error("charter_linux_amd64.tar.gz is missing, and it says the tool is there")
	}
	if hasEveryArchive("", all) {
		t.Error("an empty checksums.txt says the tool is there")
	}
}

// A path inside a git submodule exists only once it is fetched and set up: docs-lint cannot say it
// is wrong in a fresh clone, as CI has.
func TestDocsLintTrustsPathsInSubmodules(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".gitmodules"), []byte("[submodule \"app\"]\n\tpath = vendor/app\n\turl = https://example.com/app.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{"vendor/app": true, "vendor/app/node_modules": true, "vendor/application": false, "vendor": false} {
		if got := inSubmodule(repo, path); got != want {
			t.Errorf("inSubmodule(%q) = %v, want %v", path, got, want)
		}
	}
}
