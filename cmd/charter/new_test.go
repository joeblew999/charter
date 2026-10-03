package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
// (Its full check, with TinyGo and workerd, is run by hand before a release: docs/contributing.md.)
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
		"fern/generators.yml ":          "env: BILLING_API_ACCESS_CLIENT_ID\n",
		"test/soak-go/main.go":          `notes "github.com/zeta/billing-api/sdk/go"`,
		"test/soak-go/go.mod":           "github.com/zeta/billing-api/sdk/go => ../../sdk/go",
		"mise.toml":                     `CHARTER_TOOL = "go run {{env.CHARTER}}/cmd/charter"`,
		"mise.toml   ":                  `includes = ["` + filepath.ToSlash(repo) + `/tasks/shared", "` + filepath.ToSlash(repo) + `/tasks/go"]`,
		"mise.toml ":                    "\nCHARTER = \"" + filepath.ToSlash(repo) + "\"\n",
		".gitattributes":                "* text=auto eol=lf\n",
		".github/workflows/check.yml":   "- run: mise run check",
		".github/workflows/release.yml": "- run: mise run release:tags",
		"docs/README.md":                "# billing-api",
		"docs/README.md ":               replaceGuide,
		"docs/rules.md":                 "`mise run spec`",
		"README.md":                     "# billing-api",
		"charter.toml":                  "description = \"billing-api: a contract-first API on Cloudflare Workers\"\ntopics = []\n",
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
	if strings.Contains(string(tasks), toolRelease) {
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
	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./api", "-run", "TestHello|TestCreateAndList|TestTheWebSocketChannel"}, {"run", filepath.ToSlash(repo) + "/cmd/charter", "sdk-list"}} { // the tool as the project's tasks run it
		cmd := exec.Command("go", args...)
		cmd.Dir = into
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s in the new project: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

// -empty: the start project (one route) under the project's names, with nothing of the notes
// example in it, and it builds and passes its tests.
func TestNewEmptyProject(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project and builds it")
	}
	into, repo, said := scaffold(t, "-empty")
	for _, want := range []string{"The API is one route, GET /api/hello", "http://localhost:5177/api/hello", replaceGuide} {
		if !strings.Contains(said, want) {
			t.Errorf("new does not say %q:\n%s", want, said)
		}
	}
	workerURL(t, into, "billing-api.your-subdomain.workers.dev")
	for file, want := range map[string]string{
		"go.mod":                   "module github.com/zeta/billing-api\n",
		"main.go":                  `"github.com/zeta/billing-api/api"`,
		"cloudflare.config.ts":     "`billing-api-${ctx.mode}` : \"billing-api\"",
		"fern/generators.yml":      "path: github.com/zeta/billing-api/sdk/go\n",
		"mise.toml":                `includes = ["` + filepath.ToSlash(repo) + `/tasks/shared", "` + filepath.ToSlash(repo) + `/tasks/go"]`,
		"test/live-test.mjs":       "/api/hello",
		"test/mcp-test.mjs":        `"billing-api"`,
		"docs/README.md":           "one route, `GET /api/hello`",
		"README.md":                "`charter new -empty`",
		"migrations/0001_init.sql": "",
	} {
		content, err := os.ReadFile(filepath.Join(into, strings.TrimSpace(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	listed, err := output(repo, "git", "ls-files", "--cached", "--others", "--exclude-standard", "--", startDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range strings.Split(listed, "\n") {
		if rel := strings.TrimPrefix(file, startDir+"/"); !exists(filepath.Join(into, rel)) {
			t.Errorf("%s was not copied", rel)
		}
	}
	notMentioned(t, into, "notes", "charter-start", startDir, exampleTool)
	// No CLI: no cli group, so no Rust, zig or cargo-zigbuild, and the pages do not promise one.
	if hasCLIIn(t, into) {
		t.Error("an -empty project has the cli group")
	}
	if err := cliPinsAgree(into); err != nil {
		t.Error(err)
	}
	for _, file := range []string{"README.md", "docs/README.md"} {
		if content, _ := os.ReadFile(filepath.Join(into, file)); strings.Contains(string(content), "CLI from") || strings.Contains(string(content), ", cli)") {
			t.Errorf("%s promises a CLI", file)
		}
	}
	// The tool sees the tasks the project includes from the checkout (docs-lint asks it).
	if tasks := tasksOf(filepath.Join(into, "mise.toml")); !tasks["spec"] || !tasks["sdk:gen"] {
		t.Errorf("tasksOf does not find the included tasks: %v", tasks)
	}
	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = into
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s in the new project: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

// hasCLIIn reports whether the project in dir has the cli group.
func hasCLIIn(t *testing.T, dir string) bool {
	t.Helper()
	groups, err := sdkGroups(filepath.Join(dir, generators))
	if err != nil {
		t.Fatal(err)
	}
	return slices.Contains(groups, "cli")
}

// -empty -cli: the start project with the notes example's CLI: its cli group, binary named after
// the project, and the pins it needs. -cli goes only with -empty.
func TestNewEmptyProjectWithACLI(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project")
	}
	into, repo, _ := scaffold(t, "-empty", "-cli", "-lang", "ts")
	if !hasCLIIn(t, into) {
		t.Fatal("no cli group")
	}
	if err := cliPinsAgree(into); err != nil {
		t.Error(err)
	}
	groups, _ := sdkGroups(filepath.Join(into, generators))
	if want := append(slices.Clone(sdkCommon), "cli"); !slices.Equal(groups, want) {
		t.Errorf("groups %v, want %v", groups, want)
	}
	example, _ := os.ReadFile(filepath.Join(repo, exampleDir, "mise.toml"))
	for file, want := range map[string]string{
		"fern/generators.yml": "        config:\n          binaryName: billing-api\n",
		"mise.toml":           "node = \"26.10.0\"\n" + string(cliPinLines.Find(example)),
		"README.md":           "Fern generates SDKs\nand a CLI from them.",
		"docs/README.md":      "(go, typescript, typescript-dist, cli)",
	} {
		if content, _ := os.ReadFile(filepath.Join(into, file)); !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	notMentioned(t, into, "notes", "charter-start", startDirTS)
	if err := newProject([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-into", into + "2", "-from", repo, "-cli"}); err == nil || !strings.Contains(err.Error(), "-cli goes with -empty") {
		t.Errorf("new -cli without -empty: %v", err)
	}
}

// -empty -ui htmx: the start project with server-rendered pages (gsx, htmx 4) under the project's
// names; it builds, passes its tests, and its committed gsx output is what gsx generates for it.
func TestNewEmptyHTMXProject(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project and builds it")
	}
	into, repo := pagesProject(t, "htmx", startDirHTMX, "5179", "live with htmx 4")
	// -ui is htmx or datastar, and only with -empty in Go.
	for _, args := range [][]string{{"-ui", "htmx"}, {"-empty", "-ui", "htmx", "-lang", "ts"}, {"-empty", "-ui", "react"}} {
		if err := newProject(append([]string{"-name", "billing-api", "-module", "github.com/zeta/billing-api", "-into", into + "2", "-from", repo}, args...)); err == nil || !strings.Contains(err.Error(), "-ui") {
			t.Errorf("new %s: %v", strings.Join(args, " "), err)
		}
	}
}

// -empty -ui datastar: the same with Datastar.
func TestNewEmptyDatastarProject(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project and builds it")
	}
	pagesProject(t, "datastar", startDirDatastar, "5180", "live with Datastar")
}

// pagesProject makes a project with -empty -ui ui, from the example in dir (local port port), and
// checks it.
func pagesProject(t *testing.T, ui, dir, port, live string) (into, repo string) {
	t.Helper()
	into, repo, said := scaffold(t, "-empty", "-ui", ui)
	for _, want := range []string{"the page at / shows them live", pagesGuide, "http://localhost:" + port + "/api/hello", replaceGuide} {
		if !strings.Contains(said, want) {
			t.Errorf("new does not say %q:\n%s", want, said)
		}
	}
	workerURL(t, into, "billing-api.your-subdomain.workers.dev")
	for file, want := range map[string]string{
		"go.mod":               "\ntool github.com/gsxhq/gsx/cmd/gsx\n",
		"main.go":              `"github.com/zeta/billing-api/pages"`,
		"pages/home.gsx":       `"github.com/zeta/billing-api/api"`,
		"pages/home.x.go":      `"github.com/zeta/billing-api/api"`,
		"cloudflare.config.ts": `HUB: bindings.durableObject({ worker: name, exportName: "Hub" })`,
		"mise.toml":            `unchanged -files "*.x.go" go tool gsx generate`,
		"README.md":            "`charter new -empty -ui " + ui + "`",
		"README.md ":           "`mise run ui:gen`",
		"docs/README.md":       "| The pages | `pages/*.gsx`",
		"docs/README.md ":      pagesGuide,
		"docs/README.md  ":     live,
	} {
		content, err := os.ReadFile(filepath.Join(into, strings.TrimSpace(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	listed, err := output(repo, "git", "ls-files", "--cached", "--others", "--exclude-standard", "--", dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range strings.Split(listed, "\n") {
		if rel := strings.TrimPrefix(file, dir+"/"); !exists(filepath.Join(into, rel)) {
			t.Errorf("%s was not copied", rel)
		}
	}
	notMentioned(t, into, "notes", "charter-start", dir, exampleTool)
	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./..."}, {"run", filepath.ToSlash(repo) + "/cmd/charter", "unchanged", "-files", "*.x.go", "go", "tool", "gsx", "generate", "--no-cache", "-q"}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = into
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s in the new project: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	return into, repo
}

// -empty -lang ts: the TypeScript start project, with its own tests (none of the Go example's).
func TestNewEmptyTypeScriptProject(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project and locks its npm packages")
	}
	into, repo, said := scaffold(t, "-empty", "-lang", "ts", "-subdomain", "acme")
	if !strings.Contains(said, "The API is one route, GET /api/hello: add yours to src/contract.ts") || !strings.Contains(said, "http://localhost:5178/api/hello") {
		t.Errorf("new says:\n%s", said)
	}
	workerURL(t, into, "billing-api.acme.workers.dev")
	for file, want := range map[string]string{
		"src/contract.ts":    `path: "/api/hello"`,
		"package.json":       `"@charter/ts": "file:` + filepath.ToSlash(filepath.Join(repo, "ts")) + `"`,
		"package-lock.json":  `"name": "billing-api"`,
		"mise.toml":          `run = "npm ci --no-fund --no-audit --prefix {{env.CHARTER}}/ts && npm run build --prefix {{env.CHARTER}}/ts && npm ci --no-fund --no-audit"`,
		"mise.toml ":         `run = 'node test/live-test.mjs {{env.API_URL}}'`,
		"test/live-test.mjs": "/api/hello",
		"docs/README.md":     "| The server | `src/index.ts` |",
		"README.md":          "`charter new -empty -lang ts`",
	} {
		content, err := os.ReadFile(filepath.Join(into, strings.TrimSpace(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(into, "test"))
	if len(entries) != 1 {
		t.Errorf("test/ has %d files, want only live-test.mjs", len(entries))
	}
	for _, file := range []string{"go.mod", "api", "worker.mjs", "src/hub.ts", "public"} {
		if exists(filepath.Join(into, file)) {
			t.Errorf("%s is in the project", file)
		}
	}
	notMentioned(t, into, "notes", "charter-start", startDirTS)
}

// notMentioned fails for every file of the project (but the lockfile, which holds the relative path
// to this checkout's library) that mentions one of gone, in any case.
func notMentioned(t *testing.T, into string, gone ...string) {
	t.Helper()
	filepath.WalkDir(into, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(path) == "package-lock.json" {
			return err
		}
		content, _ := os.ReadFile(path)
		for _, word := range gone {
			if strings.Contains(strings.ToLower(string(content)), strings.ToLower(word)) {
				t.Errorf("%s mentions %q", path, word)
			}
		}
		return nil
	})
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
	if _, err := copyExample(repo, into, "billing-api", "github.com/zeta/billing-api", "acme", "v1.2.3", "", false, false, ""); err != nil {
		t.Fatal(err)
	}
	tasks, _ := os.ReadFile(filepath.Join(into, "mise.toml"))
	for _, want := range []string{"[tools]\n# The tool every task runs, the release's binary: `mise up --bump github:joeblew999/charter` moves to a newer one.\n\"github:joeblew999/charter\" = \"1.2.3\"\n", "\nCHARTER_TOOL = \"charter\"\n", `includes = ["git::https://github.com/joeblew999/charter.git//tasks/shared?ref=v1.2.3", "git::https://github.com/joeblew999/charter.git//tasks/go?ref=v1.2.3"]`, `-run "{{env.CHARTER_TOOL}} migrate-local -port {port}"`} {
		if !strings.Contains(string(tasks), want) {
			t.Errorf("mise.toml: no %q", want)
		}
	}
	for _, gone := range []string{"go run ../../", "\nCHARTER =", "../../tasks", exampleWorker} {
		if strings.Contains(string(tasks), gone) {
			t.Errorf("mise.toml mentions %q", gone)
		}
	}
}

// -lang ts: the TypeScript example under the project's names, with the tests it shares with the Go
// one copied in, and the TypeScript library from this checkout; its lockfile is the project's own.
func TestNewTypeScriptProject(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a project and locks its npm packages")
	}
	into, repo, said := scaffold(t, "-lang", "ts", "-subdomain", "acme")
	if first, _, _ := strings.Cut(said, "\n"); !strings.Contains(first, "package.json takes its TypeScript library") {
		t.Errorf("first line: %q", first)
	}
	workerURL(t, into, "billing-api.acme.workers.dev")
	for file, want := range map[string]string{
		"src/contract.ts":             "@orpc/contract",
		"package.json":                `"@charter/ts": "file:` + filepath.ToSlash(filepath.Join(repo, "ts")) + `"`,
		"package-lock.json":           `"name": "billing-api"`,
		"package-lock.json ":          `"node_modules/@charter/ts": {`,
		"mise.toml":                   `run = "npm ci --no-fund --no-audit --prefix {{env.CHARTER}}/ts && npm run build --prefix {{env.CHARTER}}/ts && npm ci --no-fund --no-audit"`,
		"mise.toml ":                  `run = '{{env.CHARTER_TOOL}} exec -secrets node test/live-test.mjs {{env.API_URL}}`,
		"mise.toml  ":                 `includes = ["` + filepath.ToSlash(repo) + `/tasks/shared", "` + filepath.ToSlash(repo) + `/tasks/ts"]`,
		"go.work":                     "use " + filepath.ToSlash(repo) + "\n",
		"fern/generators.yml":         "path: github.com/zeta/billing-api/sdk/go\n",
		"fern/generators.yml ":        "env: BILLING_API_ACCESS_CLIENT_ID\n",
		"test/live-test.mjs":          "",
		"test/access.mjs":             "accessHeaders",
		"test/soak-go/main.go":        `notes "github.com/zeta/billing-api/sdk/go"`,
		".github/workflows/check.yml": "- run: mise run check",
		"docs/README.md":              "`src/contract.ts` (oRPC and Zod)",
		"docs/rules.md":               "`src/contract.ts`",
		"README.md":                   "oRPC and Zod",
	} {
		content, err := os.ReadFile(filepath.Join(into, strings.TrimSpace(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
	if lock, _ := os.ReadFile(filepath.Join(into, "package-lock.json")); strings.Contains(string(lock), `"../../ts"`) {
		t.Error("package-lock.json still links the example's ../../ts")
	}
	for _, file := range []string{"go.mod", "api", "worker.mjs"} {
		if exists(filepath.Join(into, file)) {
			t.Errorf("%s is in a TypeScript project", file)
		}
	}
	filepath.WalkDir(into, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, _ := os.ReadFile(path)
		for _, gone := range []string{"charter-notes", "notes-go", "notes-ts", "../../", "TinyGo", "Huma"} {
			// (The lockfile holds the path to this checkout's library, relative: from a checkout only.)
			if strings.Contains(string(content), gone) && !strings.Contains(filepath.ToSlash(path), "/test/") && !(gone == "../../" && filepath.Base(path) == "package-lock.json") {
				t.Errorf("%s mentions %q", path, gone)
			}
		}
		return nil
	})
}

// From a release, a TypeScript project installs the library from the release's package, and its
// setup is the npm install alone.
func TestNewTypeScriptProjectFromARelease(t *testing.T) {
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	into := t.TempDir()
	if _, err := copyExample(repo, into, "billing-api", "github.com/zeta/billing-api", "acme", "v1.2.3", "", true, false, ""); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{
		"package.json": `"@charter/ts": "https://github.com/joeblew999/charter/releases/download/v1.2.3/charter-ts-1.2.3.tgz"`,
		"mise.toml":    "\"github:joeblew999/charter\" = \"1.2.3\"\n",
		"mise.toml ":   "[tasks.setup]\ndescription = \"Install the npm packages: cf (Cloudflare's CLI), Fern's CLI, what the tests import, and @charter/ts (the TypeScript library)\"\nrun = \"npm ci --no-fund --no-audit\"\n",
		"mise.toml  ":  "\nCHARTER_TOOL = \"charter\"\n",
		"mise.toml   ": `"git::https://github.com/joeblew999/charter.git//tasks/ts?ref=v1.2.3"]`,
	} {
		content, _ := os.ReadFile(filepath.Join(into, strings.TrimSpace(file)))
		if !strings.Contains(string(content), want) {
			t.Errorf("%s: no %q", file, want)
		}
	}
}

// What the first line says a release pins, and what a checkout does.
func TestPinned(t *testing.T) {
	for want, got := range map[string]string{
		"charter v1.2.3: pins the tool to v1.2.3 in mise.toml and the Go library to v1.2.3 in go.mod":                                                                                                 pinned("v1.2.3", "", false),
		"charter v1.2.3, copying from /src: the tasks run the tool from that checkout (CHARTER in mise.toml), and go.mod builds against its library (a replace line)":                                 pinned("v1.2.3", "/src", false),
		"charter (not a release: built from a checkout), copying from /src: the tasks run the tool from that checkout (CHARTER in mise.toml), and go.mod builds against its library (a replace line)": pinned("", "/src", false),
		"charter v1.2.3: pins the tool to v1.2.3 in mise.toml and the TypeScript library to v1.2.3 in package.json":                                                                                   pinned("v1.2.3", "", true),
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
