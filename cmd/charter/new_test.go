package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scaffold makes a project from this checkout and returns its folder and what new printed.
func scaffold(t *testing.T, more ...string) (into, repo, said string) {
	t.Helper()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	into = filepath.Join(t.TempDir(), "billing-api")
	started = t.TempDir()
	said = printed(t, func() error {
		return newProject(append([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-into", into, "-from", repo}, more...))
	})
	return into, repo, said
}

// A project made from this checkout is the example under its own names, and it builds.
// (Its full check, with TinyGo and workerd, is run by hand before a release: docs/reference/charter.md.)
func TestNewProjectIsTheExampleUnderItsOwnName(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project and builds it")
	}
	into, repo, said := scaffold(t)
	// The first line is the tool's version and where the project gets the tool and the library; the
	// end says what the placeholder URL needs, and the cost.
	if first, _, _ := strings.Cut(said, "\n"); !strings.HasPrefix(first, "charter (not a release: built from a checkout), copying from "+repo+": the tasks run the tool from that checkout") {
		t.Errorf("first line: %q", first)
	}
	for _, want := range []string{"https://billing-api.your-subdomain.workers.dev", "mise.local.toml as API_URL", "mise run spec", "under 1 ms of CPU", replaceGuide} {
		if !strings.Contains(said, want) {
			t.Errorf("new does not say %q:\n%s", want, said)
		}
	}
	workerURL(t, into, "billing-api.your-subdomain.workers.dev")
	for file, want := range map[string]string{
		"go.mod":                        "module github.com/zeta/billing-api\n",
		"go.mod ":                       "replace " + libraryModule + " => " + filepath.Join(repo, "go"),
		"go.work":                       "\t" + repo + "\n",
		"worker.mjs":                    `import { goWorker } from "./build/go.mjs";`,
		"api/handlers.go":               `"` + libraryModule + `/humaworkers"`,
		"main.go":                       `"github.com/zeta/billing-api/api"`,
		"cloudflare.config.ts":          "`billing-api-${ctx.mode}` : \"billing-api\"",
		"package.json":                  `"name": "billing-api"`,
		"fern/fern.config.json":         `"organization": "billing-api"`,
		"fern/generators.yml":           "path: github.com/zeta/billing-api/sdk/go\n",
		"test/soak-go/main.go":          `notes "github.com/zeta/billing-api/sdk/go"`,
		"test/soak-go/go.mod":           "github.com/zeta/billing-api/sdk/go => ../../sdk/go",
		"mise.toml":                     `run = "go run $CHARTER/cmd/charter wasm-build"`,
		"mise.toml ":                    "\nCHARTER = \"" + repo + "\"\n",
		".github/workflows/check.yml":   "- run: mise run check",
		".github/workflows/release.yml": "- run: mise run release:tags",
		"docs/README.md":                "# billing-api",
		"docs/README.md ":               replaceGuide,
		"docs/rules.md":                 "`mise run spec`",
		"README.md":                     "# billing-api",
	} {
		content, err := os.ReadFile(filepath.Join(into, strings.TrimSpace(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	// A plain copy: every file of the example is there (but the committed SDK, made from specs that
	// now name another Worker), and nothing in the project names the example or this repo's folders.
	listed, err := output(repo, "git", "ls-files", "--cached", "--others", "--exclude-standard", "--", exampleDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range strings.Split(listed, "\n") {
		rel := strings.TrimPrefix(file, exampleDir+"/")
		if strings.HasPrefix(rel, exampleSDK) != !exists(filepath.Join(into, rel)) {
			t.Errorf("%s: copied = %v", rel, exists(filepath.Join(into, rel)))
		}
	}
	filepath.WalkDir(into, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, _ := os.ReadFile(path)
		for _, gone := range []string{exampleWorker, exampleDir, "notes-ts", "showcase-go", "showcase-ts", exampleTool} {
			if strings.Contains(string(content), gone) {
				t.Errorf("%s mentions %q", path, gone)
			}
		}
		return nil
	})
	// From a checkout the tool is the checkout's: no release is pinned, which could be an older tool.
	tasks, _ := os.ReadFile(filepath.Join(into, "mise.toml"))
	if strings.Contains(string(tasks), `"go:`+toolPackage+`"`) {
		t.Error("mise.toml pins a release of the tool, though the project was made from a checkout")
	}
	// The same tasks as the example: nothing was filtered.
	source, _ := os.ReadFile(filepath.Join(repo, exampleDir, "mise.toml"))
	if want, got := strings.Count(string(source), "\n[tasks."), strings.Count(string(tasks), "\n[tasks."); got != want {
		t.Errorf("mise.toml has %d tasks, the example %d", got, want)
	}
	// The library and its Worker glue are required, not copied.
	for _, gone := range []string{"go", "worker", "build"} {
		if exists(filepath.Join(into, gone)) {
			t.Errorf("%s is in the project: it is the library's", gone)
		}
	}
	// The module path above sorts after charter's among the imports: the project must still be formatted.
	if out, err := exec.Command("gofmt", "-l", into).Output(); err != nil || len(out) > 0 {
		t.Errorf("gofmt -l in the new project: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./api", "-run", "TestHello|TestCreateAndList|TestTheWebSocketChannel"}, {"run", filepath.Join(repo, "cmd", "charter"), "sdk-list"}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = into
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
	into, repo, said := scaffold(t, "-subdomain", "acme")
	if !strings.Contains(said, "The Worker's URL is https://billing-api.acme.workers.dev in mise.toml and in the specs") || strings.Contains(said, placeholderSubdomain) {
		t.Errorf("new says:\n%s", said)
	}
	workerURL(t, into, "billing-api.acme.workers.dev")
	if err := newProject([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-subdomain", "acme.workers.dev", "-into", into + "2", "-from", repo}); err == nil {
		t.Error("-subdomain acme.workers.dev was accepted: it is the one word before .workers.dev")
	}
}

// From a release (its tag, cloned), the project pins that release of the tool under [tools], and
// its tasks run `charter`, which mise then has on the path.
func TestNewProjectFromAReleasePinsTheTool(t *testing.T) {
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	into := t.TempDir()
	if _, err := copyExample(repo, into, "billing-api", "github.com/zeta/billing-api", "acme", "v1.2.3", ""); err != nil {
		t.Fatal(err)
	}
	tasks, _ := os.ReadFile(filepath.Join(into, "mise.toml"))
	for _, want := range []string{"[tools]\n# The tool every task runs: `mise up` moves to a newer release.\n\"go:" + toolPackage + "\" = \"1.2.3\"\n", `run = "charter wasm-build"`, `-run "charter migrate-local -port {port}"`} {
		if !strings.Contains(string(tasks), want) {
			t.Errorf("mise.toml: no %q", want)
		}
	}
	for _, gone := range []string{"go run ../../", "CHARTER", exampleWorker} {
		if strings.Contains(string(tasks), gone) {
			t.Errorf("mise.toml mentions %q", gone)
		}
	}
}

// What the first line says a release pins, and what a checkout does.
func TestPinned(t *testing.T) {
	for want, got := range map[string]string{
		"charter v1.2.3: pins the tool to v1.2.3 in mise.toml and the Go library to v1.2.3 in go.mod":                                                                                                 pinned("v1.2.3", ""),
		"charter v1.2.3, copying from /src: the tasks run the tool from that checkout (CHARTER in mise.toml), and go.mod builds against its library (a replace line)":                                 pinned("v1.2.3", "/src"),
		"charter (not a release: built from a checkout), copying from /src: the tasks run the tool from that checkout (CHARTER in mise.toml), and go.mod builds against its library (a replace line)": pinned("", "/src"),
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
		"mise.toml":          "default='https://" + host + "'",
		"fern/openapi.json":  `"url": "https://` + host + `"`,
		"fern/asyncapi.json": `"host": "` + host + `"`,
	} {
		if content, _ := os.ReadFile(filepath.Join(into, file)); !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	source, _ := os.ReadFile(filepath.Join("../..", exampleDir, "mise.toml"))
	owner := ownerSubdomain.FindSubmatch(source)
	if owner == nil {
		t.Fatal("the example's mise.toml names no workers.dev subdomain")
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
