package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A project made from this checkout must build, and must carry its own names, not this repo's.
// (Its full check, with TinyGo and workerd, is run by hand before a release: docs/dev.md.)
func TestNewProjectBuildsUnderItsOwnName(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project and builds it")
	}
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	into := filepath.Join(t.TempDir(), "billing-api")
	started = t.TempDir()
	if err := newProject([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-into", into, "-from", repo}); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{
		"api-go/go.mod":                       "module github.com/zeta/billing-api/api-go",
		"api-go/main.go":                      `"github.com/zeta/billing-api/api-go/api"`,
		"api-go/cloudflare.config.ts":         `name: "billing-api"`,
		"sdk/fern/apis/api-go/generators.yml": "namespaceExport: BillingApi",
		"mise.toml":                           `depends = ["api-go:check", "dev:check"]`,
		"go.work":                             "use ./api-go",
		"docs/README.md":                      "# billing-api",
	} {
		content, err := os.ReadFile(filepath.Join(into, file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	tasks, _ := os.ReadFile(filepath.Join(into, "mise.toml"))
	for _, gone := range []string{"api:check", "sdk:harness", "sdk:demo", "showcase", "go run ./dev", "go run ../dev", "orpc-api-go"} {
		if strings.Contains(string(tasks), gone) {
			t.Errorf("mise.toml still mentions %q", gone)
		}
	}
	if exists(filepath.Join(into, "api-go/api/surface_test.go")) {
		t.Error("the oRPC same-surface test was copied")
	}
	// The module path above sorts after orpc-api's among the imports: the project must still be formatted.
	if out, err := exec.Command("gofmt", "-l", filepath.Join(into, "api-go"), filepath.Join(into, "test")).Output(); err != nil || len(out) > 0 {
		t.Errorf("gofmt -l in the new project: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./api", "-run", "TestHello|TestCreateAndList|TestTheWebSocketChannel"}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = filepath.Join(into, "api-go")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s in the new project: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}
