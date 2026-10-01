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
	said := printed(t, func() error {
		return newProject([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-into", into, "-from", repo})
	})
	// The first line is the tool's version and what it pinned; the end says what the placeholder URL needs, and the cost.
	if first, _, _ := strings.Cut(said, "\n"); !strings.HasPrefix(first, "dev (not a release: built from a checkout), copying from "+repo+": pins the dev tool to latest in mise.toml") {
		t.Errorf("first line: %q", first)
	}
	for _, want := range []string{"https://billing-api.your-subdomain.workers.dev", "mise.local.toml as API_GO_URL", "mise run api-go:spec", "2 to 7 ms of CPU", replaceGuide} {
		if !strings.Contains(said, want) {
			t.Errorf("new does not say %q:\n%s", want, said)
		}
	}
	workerURL(t, into, "billing-api.your-subdomain.workers.dev")
	for file, want := range map[string]string{
		"api-go/go.mod":                       "module github.com/zeta/billing-api/api-go",
		"api-go/main.go":                      `"github.com/zeta/billing-api/api-go/api"`,
		"api-go/cloudflare.config.ts":         `name: "billing-api"`,
		"sdk/fern/apis/api-go/generators.yml": "namespaceExport: BillingApi",
		"test/soak-go/main.go":                `billingapi "github.com/zeta/billing-api/sdk/go"`,
		"test/soak-go/go.mod":                 "github.com/zeta/billing-api/sdk/go => ../../sdk/go",
		"mise.toml":                           `"go:github.com/joeblew999/orpc-api/dev" = "latest"`,
		"go.work":                             "use ./api-go",
		"docs/README.md":                      "# billing-api",
		"docs/README.md ":                     replaceGuide,
	} {
		content, err := os.ReadFile(filepath.Join(into, strings.TrimSpace(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	// The Go SDK's module path is the project's, so another repo can go get what sdk:publish commits.
	generators, _ := os.ReadFile(filepath.Join(into, "sdk/fern/apis/api-go/generators.yml"))
	if want := "path: github.com/zeta/billing-api/sdk/go\n"; !strings.Contains(string(generators), want) || strings.Contains(string(generators), "example.com") {
		t.Errorf("generators.yml: the Go SDK's module path is not %q", want)
	}
	tasks, _ := os.ReadFile(filepath.Join(into, "mise.toml"))
	for _, want := range []string{`[tasks."sdk:publish"]`, `[tasks."sdk:publish:check"]`, "dev sdk-publish api-go", "dev release-tags api-go sdk/go"} {
		if !strings.Contains(string(tasks), want) {
			t.Errorf("mise.toml: no %q", want)
		}
	}
	for _, gone := range []string{"api:check", "sdk:harness", "sdk:demo", "showcase", "go run ./dev", "go run ../dev", "dev@", "orpc-api-go", "dev/vX.Y.Z"} {
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

// With -subdomain the project's Worker is on that workers.dev subdomain, everywhere it is named.
func TestNewProjectOnYourSubdomain(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project")
	}
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	into := filepath.Join(t.TempDir(), "billing-api")
	started = t.TempDir()
	said := printed(t, func() error {
		return newProject([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-subdomain", "acme", "-into", into, "-from", repo})
	})
	if !strings.Contains(said, "The Worker's URL is https://billing-api.acme.workers.dev in mise.toml and in the specs") || strings.Contains(said, placeholderSubdomain) {
		t.Errorf("new says:\n%s", said)
	}
	workerURL(t, into, "billing-api.acme.workers.dev")
	if err := newProject([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-subdomain", "acme.workers.dev", "-into", into + "2", "-from", repo}); err == nil {
		t.Error("-subdomain acme.workers.dev was accepted: it is the one word before .workers.dev")
	}
}

// What the first line says a release pins, and what a checkout does.
func TestPinned(t *testing.T) {
	for want, got := range map[string]string{
		"dev v1.2.3: pins the dev tool to v1.2.3 in mise.toml and the Go packages to v1.2.3 in api-go/go.mod":                                  pinned("v1.2.3", false, "/tmp/clone"),
		"dev v1.2.3, copying from /src: pins the dev tool to v1.2.3 in mise.toml; api-go/go.mod builds against that checkout (a replace line)": pinned("v1.2.3", true, "/src"),
	} {
		if got != want {
			t.Errorf("got  %s\nwant %s", got, want)
		}
	}
}

// workerURL fails unless the project's default URL and both committed specs name host (the spec
// check compares them), and no file of the project names this repo owner's subdomain.
func workerURL(t *testing.T, into, host string) {
	t.Helper()
	for file, want := range map[string]string{
		"mise.toml":                          "default='https://" + host + "'",
		"sdk/fern/apis/api-go/openapi.json":  `"url": "https://` + host + `"`,
		"sdk/fern/apis/api-go/asyncapi.json": `"host": "` + host + `"`,
	} {
		if content, _ := os.ReadFile(filepath.Join(into, file)); !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	source, _ := os.ReadFile("../mise.toml")
	owner := ownerSubdomain.FindSubmatch(source)
	if owner == nil {
		t.Fatal("this repo's mise.toml names no workers.dev subdomain")
	}
	filepath.WalkDir(into, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if content, _ := os.ReadFile(path); strings.Contains(string(content), string(owner[1])) {
			t.Errorf("%s names this repo's subdomain (%s)", path, owner[1])
		}
		return nil
	})
}

// printed runs f and returns what it wrote to standard output.
func printed(t *testing.T, f func() error) string {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = file
	err = f()
	os.Stdout = stdout
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(file.Name())
	return string(out)
}
