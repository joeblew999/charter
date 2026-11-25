package deps

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestHolds: a hold names a dependency or, ending in *, every one that starts so.
func TestHolds(t *testing.T) {
	h := Holds{"@moq/*": "draft-18", "github.com/syumai/workers-go": "exactly v0.36.0"}
	for name, want := range map[string]bool{"@moq/watch": true, "@moq/publish": true, "leaflet": false,
		"github.com/syumai/workers-go": true, "github.com/syumai/workers-go/x": false} {
		if _, held := h.Why(name); held != want {
			t.Errorf("Why(%q) held = %v, want %v", name, held, want)
		}
	}
}

// TestVersions: what is newer, and the commit a pseudo-version names.
func TestVersions(t *testing.T) {
	if !Newer("v0.4.3", "v0.4.0") || Newer("v0.4.0", "v0.4.3") || Newer("v2.24.24", "v2.24.25-0.20261028213736-f562a9186eaf") {
		t.Error("Newer")
	}
	if got := Commit("v0.0.0-20261030163719-65f4813ea8be"); got != "65f4813ea8be" {
		t.Errorf("Commit = %q", got)
	}
	if got := Commit("v0.4.3"); got != "" {
		t.Errorf("Commit of a tag = %q", got)
	}
}

// TestMisePin: a pin is read and moved in place, quoted key or not, the rest of mise.toml as it was.
func TestMisePin(t *testing.T) {
	root := t.TempDir()
	toml := "[tools]\n\"go:github.com/gsxhq/gsxui/cmd/gsxui\" = \"65f4813e\" # a commit\nworkerd = \"1.20261019.1\"\n"
	if err := os.WriteFile(filepath.Join(root, "mise.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, ok := MisePin(root, "go:github.com/gsxhq/gsxui/cmd/gsxui"); !ok || v != "65f4813e" {
		t.Errorf("MisePin gsxui = %q, %v", v, ok)
	}
	if err := SetMisePin(root, "workerd", "1.20261030.1"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "mise.toml"))
	if want := "[tools]\n\"go:github.com/gsxhq/gsxui/cmd/gsxui\" = \"65f4813e\" # a commit\nworkerd = \"1.20261030.1\"\n"; string(b) != want {
		t.Errorf("after SetMisePin:\n%s", b)
	}
	if SetMisePin(root, "nope", "1") == nil {
		t.Error("SetMisePin of a tool mise.toml doesn't pin succeeded")
	}
}

// TestMoveCopies: a pin's copy moves with it, and a copy deps.toml names that doesn't write the old version fails.
func TestMoveCopies(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "v.go"), []byte("const Version = \"1.1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Config{Copies: map[string][]string{"workerd": {"v.go"}}}
	if err := c.MoveCopies(root, "workerd", "1.1", "1.2"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "v.go")); string(b) != "const Version = \"1.2\"\n" {
		t.Errorf("after MoveCopies: %s", b)
	}
	if c.MoveCopies(root, "workerd", "1.1", "1.3") == nil {
		t.Error("a copy without the old version moved silently")
	}
}

// TestConfig: deps.toml's holds, copies and skipped folders; none is no file.
func TestConfig(t *testing.T) {
	root := t.TempDir()
	if c, err := Read(root); err != nil || len(c.Hold) != 0 || len(c.Skip) != 0 {
		t.Errorf("no deps.toml: %+v, %v", c, err)
	}
	os.WriteFile(filepath.Join(root, "deps.toml"), []byte("skip = [\"sdk/go\"]\n[hold]\n\"a/b\" = \"why\"\n[copies]\n\"*\" = [\"pins.toml\"]\n"), 0o644)
	c, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if why, held := c.Hold.Why("a/b"); !held || why != "why" || len(c.Skip) != 1 || c.Skip[0] != "sdk/go" || c.Copies["*"][0] != "pins.toml" {
		t.Errorf("read %+v", c)
	}
}

// TestPinFiles: a file written as mise.toml is ([copies] "*") moves the tools it pins, and is left alone for one it
// doesn't.
func TestPinFiles(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "pins.toml"), []byte("[tools]\ntinygo = \"0.44.0\"\nworkerd = \"133\"\n\"aqua:WebAssembly/binaryen\" = \"133\"\n"), 0o644)
	c := Config{Copies: map[string][]string{"*": {"pins.toml"}}}
	if err := c.MoveCopies(root, "aqua:WebAssembly/binaryen", "133", "134"); err != nil {
		t.Fatal(err)
	}
	if err := c.MoveCopies(root, "gh", "2.1", "2.2"); err != nil {
		t.Errorf("a tool the file doesn't pin: %v", err)
	}
	// Only binaryen's line moved, though workerd's has the same version.
	if b, _ := os.ReadFile(filepath.Join(root, "pins.toml")); string(b) != "[tools]\ntinygo = \"0.44.0\"\nworkerd = \"133\"\n\"aqua:WebAssembly/binaryen\" = \"134\"\n" {
		t.Errorf("after the move:\n%s", b)
	}
}

// TestModules: every go.mod git tracks but the skipped folders', with what each requires directly, for a tool, and
// replaces.
func TestModules(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755)
		os.WriteFile(filepath.Join(root, name), []byte(content), 0o644)
	}
	write("go.mod", "module example.com/app\n\ngo 1.27\n\ntool example.com/tool/cmd/t\n\nrequire (\n\texample.com/direct v1.2.0\n\texample.com/indirect v0.1.0 // indirect\n\texample.com/tool v0.3.0 // indirect\n\texample.com/app/sub v0.0.0\n)\n\nreplace example.com/app/sub => ./sub\n")
	write("sub/go.mod", "module example.com/app/sub\n\ngo 1.27\n")
	write("sdk/go/go.mod", "module example.com/app/sdk/go\n\ngo 1.27\n\nrequire example.com/generated v1.0.0\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	mods, err := Modules(root, "sdk/go")
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Fatalf("%d modules, want the app and its sub (not the skipped SDK)", len(mods))
	}
	app := mods[0]
	if app.Path != "example.com/app" || app.Requires["example.com/direct"] != "v1.2.0" || app.Requires["example.com/tool"] != "v0.3.0" {
		t.Errorf("the app's requires: %+v", app.Requires)
	}
	if _, ok := app.Requires["example.com/indirect"]; ok {
		t.Error("an indirect requirement is listed")
	}
	if !app.Replaced["example.com/app/sub"] {
		t.Error("the replaced module isn't marked")
	}
}
