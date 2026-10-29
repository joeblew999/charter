package scaffold

import (
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/joeblew999/charter/project"
)

// TestInit: a module whose mise.toml pins charter (mise use github:joeblew999/charter, as a user starts) gets a
// charter.toml that loads, named after it, build/ ignored and the mise tasks; again, it changes nothing it made.
func TestInit(t *testing.T) {
	t.Chdir(t.TempDir())
	os.WriteFile("go.mod", []byte("module github.com/you/My_App\n\ngo 1.26\n"), 0o644)
	os.WriteFile("mise.toml", []byte("[tools]\n\"github:joeblew999/charter\" = \"0.20.0\"\n"), 0o644)
	Init([]string{"-compat", "2026-09-25"})
	p, err := project.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "my-app" || p.CompatibilityDate != "2026-09-25" || p.Local.Vars["APP_ENV"] != "local" {
		t.Errorf("charter.toml: %+v", p)
	}
	var tasks map[string]any
	if _, err := toml.DecodeFile(TasksFile, &tasks); err != nil || tasks["charter:deploy"] == nil {
		t.Errorf("tasks: %v %v", err, tasks)
	}
	os.WriteFile(project.File, []byte(strings.Replace(mustRead(t, project.File), `name = "my-app"`, `name = "kept"`, 1)), 0o644)
	Init(nil)
	if g := mustRead(t, ".gitignore"); strings.Count(g, "/build/") != 1 {
		t.Errorf(".gitignore: %q", g)
	}
	if !strings.Contains(mustRead(t, project.File), `name = "kept"`) {
		t.Error("init again replaced charter.toml")
	}
	for _, f := range []string{".mise/tasks/charter-builds", ".claude/skills/charter/SKILL.md", ".github/workflows/charter.yml"} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("init wrote no %s", f)
		}
	}
	p, err = project.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	if problems := Problems(p); len(problems) != 0 {
		t.Errorf("a fresh init has problems: %v", problems)
	}
	os.WriteFile(TasksFile, []byte("# changed\n"), 0o644)
	os.Chmod(".mise/tasks/charter-builds", 0o644)
	problems := Problems(p)
	if len(problems) != 2 || !strings.Contains(problems[0], "isn't this charter's") || !strings.Contains(problems[1], "isn't executable") {
		t.Errorf("problems: %q", problems)
	}
	if workerName("example.com/x/Go.HTMX_4") != "go-htmx-4" {
		t.Errorf("workerName = %s", workerName("example.com/x/Go.HTMX_4"))
	}
}

// TestSkew: a released charter refuses a go.mod that requires another version of its library, which it builds the
// Worker from; a checkout's charter, a replaced library or none are no problem.
func TestSkew(t *testing.T) {
	gomod := "module example.com/app\n\nrequire (\n\tgithub.com/joeblew999/charter v0.20.0\n)\n"
	if s := skew(gomod, "v0.20.0", true); s != "" {
		t.Errorf("same version: %s", s)
	}
	if s := skew(gomod, "v0.21.0", true); !strings.Contains(s, "requires github.com/joeblew999/charter v0.20.0, but this charter is v0.21.0") {
		t.Errorf("another version: %q", s)
	}
	if s := skew(gomod, "(devel)", false); s != "" {
		t.Errorf("a checkout's charter: %s", s)
	}
	if s := skew(gomod+"\nreplace github.com/joeblew999/charter => ../charter\n", "v0.21.0", true); s != "" {
		t.Errorf("replaced: %s", s)
	}
	if s := skew("module example.com/app\n", "v0.21.0", true); s != "" {
		t.Errorf("no library: %s", s)
	}
}

// TestUnpinned: without charter in its mise.toml, an app is told how to pin it.
func TestUnpinned(t *testing.T) {
	t.Chdir(t.TempDir())
	os.WriteFile("mise.toml", []byte("[tools]\ngo = \"1.27.3\"\n"), 0o644)
	if p := ownPins(true); len(p) != 1 || !strings.Contains(p[0], "mise use github:joeblew999/charter@<version>") {
		t.Errorf("ownPins: %q", p)
	}
}

func mustRead(t *testing.T, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
