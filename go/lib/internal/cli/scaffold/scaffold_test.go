package scaffold

import (
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/joeblew999/charter/go/lib/project"
)

// TestInit: a module gets a charter.toml that loads, named after it, build/ ignored and the mise tasks; again, it
// changes nothing it made.
func TestInit(t *testing.T) {
	t.Chdir(t.TempDir())
	os.WriteFile("go.mod", []byte("module github.com/you/My_App\n\ngo 1.26\n"), 0o644)
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
	if workerName("example.com/x/Go.HTMX_4") != "go-htmx-4" {
		t.Errorf("workerName = %s", workerName("example.com/x/Go.HTMX_4"))
	}
}

func mustRead(t *testing.T, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
