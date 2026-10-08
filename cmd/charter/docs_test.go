package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A page docs/_generated.toml lists: written from its command's output with charter's front
// matter, stale as soon as the command prints something else, and linted only for what can be fixed
// in the page's place in the site, not for its content.
func TestGeneratedPages(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"docs/_generated.toml": "# the command reference\n[[generated]]\npage = \"reference/commands.md\"\nrun = \"go run gen.go\"\n",
		// The repo's generator: it prints the page, here from a file standing in for the code.
		"gen.go":            "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tflags, _ := os.ReadFile(\"flags.txt\")\n\tfmt.Printf(\"# Commands\\n\\nEvery flag, from the code: `-%s`. Run `mise run nothing-like-it` in `cmd/none/`.\\n\", flags)\n}\n",
		"flags.txt":         "port",
		"docs/README.md":    "---\ntitle: Home\nnav_order: 1\npermalink: /\n---\n\n# A tool\n\n[Reference](reference.md)\n",
		"docs/reference.md": "---\ntitle: Reference\nnav_order: 2\n---\n\n# Reference\n\n[Commands](reference/commands.md)\n",
		"mise.toml":         "[tasks.check]\nrun = \"true\"\n",
	}
	for name, content := range files {
		if err := write(filepath.Join(root, name), content); err != nil {
			t.Fatal(err)
		}
	}
	list, err := readGenerated(filepath.Join(root, "docs"))
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	generate := func() []byte {
		t.Helper()
		pages := map[string][]byte{}
		if err := addGenerated(pages, root, "docs", list); err != nil {
			t.Fatal(err)
		}
		return pages[filepath.Join("docs", "reference", "commands.md")]
	}
	page := generate()
	want := "---\ntitle: \"Commands\"\nnav_order: 100\nparent: \"Reference\"\n---\n\n# Commands\n\n" +
		"Written by `go run gen.go` (docs/_generated.toml), never by hand: change the code it reads, then `mise run docs:setup`.\n\n" +
		"Every flag, from the code: `-port`. Run `mise run nothing-like-it` in `cmd/none/`.\n"
	if string(page) != want {
		t.Fatalf("the page:\n%s\nwant:\n%s", page, want)
	}
	if err := write(filepath.Join(root, "docs", "reference", "commands.md"), string(page)); err != nil {
		t.Fatal(err)
	}
	// Its content names a task and a path that don't exist: the lint leaves that to the code.
	if err := docsLint([]string{"-into", root}); err != nil {
		t.Errorf("the lint faults a generated page's content: %v", err)
	}
	// The same page written by hand is faulted.
	if err := os.Remove(filepath.Join(root, "docs", "_generated.toml")); err != nil {
		t.Fatal(err)
	}
	if err := docsLint([]string{"-into", root}); err == nil {
		t.Error("the lint passes a hand-written page with an unknown task and path")
	}
	// The code changes: the committed page is stale.
	if err := write(filepath.Join(root, "flags.txt"), "addr"); err != nil {
		t.Fatal(err)
	}
	if now := generate(); string(now) == string(page) || !strings.Contains(string(now), "`-addr`") {
		t.Errorf("after the code changed, the generator writes:\n%s", now)
	}
	// A command that fails, or prints no title, writes nothing.
	for run, why := range map[string]string{"go run missing.go": "fails", "go version": "no # title"} {
		if err := addGenerated(map[string][]byte{}, root, "docs", []generatedPage{{"reference/commands.md", run}}); err == nil {
			t.Errorf("a command that %s: no error", why)
		}
	}
}

// The format of docs/_generated.toml: what it takes, and what it refuses rather than ignores.
func TestGeneratedFormat(t *testing.T) {
	list, err := parseGenerated("# pages from the code\n[[generated]]\npage = \"reference/commands.md\"\nrun = \"go run ./cmd/x docs-commands\"\n\n[[generated]]\nrun = \"x \\\"two words\\\"\"\npage = \"mcp.md\"\n")
	if err != nil || len(list) != 2 || list[0] != (generatedPage{"reference/commands.md", "go run ./cmd/x docs-commands"}) ||
		!slices.Equal(splitCommand(list[1].Run), []string{"x", "two words"}) {
		t.Errorf("parsed %+v, %v", list, err)
	}
	for text, why := range map[string]string{
		"page = \"a.md\"\nrun = \"x\"":                                                             "a key before a table",
		"[[generated]]\npage = \"a.md\"":                                                           "no run",
		"[[generated]]\npage = \"../a.md\"\nrun = \"x\"":                                           "a page outside docs/",
		"[[generated]]\npage = \"a.html\"\nrun = \"x\"":                                            "a page that is not markdown",
		"[[generated]]\npage = \"a.md\"\nrun = \"x\"\ntitle = \"A\"":                               "an unknown key",
		"[[generated]]\npage = \"a.md\"\nrun = \"x\"\nrun = \"y\"":                                 "a key twice",
		"[[generated]]\npage = \"a.md\"\nrun = x":                                                  "a string without quotes",
		"[[generated]]\npage = \"a.md\"\nrun = \"x\"\n[[generated]]\npage = \"a.md\"\nrun = \"y\"": "a page twice",
	} {
		if _, err := parseGenerated(text); err == nil {
			t.Errorf("%s: no error for %q", why, text)
		}
	}
}

// A repo whose product is one task file, which other repos include from git, includes that file
// by name itself. The tasks in it are tasks a page may name, as those in an included folder are.
func TestTasksOfAnIncludedFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mise.toml", "[task_config]\nincludes = [\"tasks.toml\", \"more\"]\n\n[tasks.test]\nrun = \"true\"\n")
	write("tasks.toml", "[\"site:new\"]\nrun = \"true\"\n\n[emdash]\nrun = \"true\"\n")
	write("more/extra.toml", "[\"docs:lint\"]\nrun = \"true\"\n")
	tasks := tasksOf(filepath.Join(dir, "mise.toml"))
	for _, name := range []string{"test", "site:new", "emdash", "docs:lint"} {
		if !tasks[name] {
			t.Errorf("tasksOf does not find %s: %v", name, tasks)
		}
	}
}
