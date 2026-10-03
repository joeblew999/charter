package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The projects of this repo, by path from its root: the examples (what a project starts from) and
// the conformance projects (every Fern feature, end to end).
func examples(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, parent := range []string{"examples", "conformance"} {
		dirs, err := projectsBelow(filepath.Join("../..", parent))
		if err != nil {
			t.Fatal(err)
		}
		for _, dir := range dirs {
			names = append(names, parent+"/"+filepath.Base(dir))
		}
	}
	if want := []string{"examples/notes-go", "examples/notes-ts", "examples/start-datastar", "examples/start-go", "examples/start-htmx", "examples/start-ts", "conformance/showcase-go", "conformance/showcase-ts"}; !slices.Equal(names, want) {
		t.Fatalf("the repo has the projects %v, want %v", names, want)
	}
	return names
}

// Every example has the same groups in the same order, so the same task works in each of them.
// The showcases add one: typescript-public, which shows Fern's audiences.
func TestEveryExampleHasTheCommonGroups(t *testing.T) {
	for _, name := range examples(t) {
		groups, err := sdkGroups(filepath.Join("../..", name, generators))
		if err != nil {
			t.Fatal(err)
		}
		want := sdkCommon
		if strings.HasPrefix(filepath.Base(name), "showcase-") {
			want = append(slices.Clone(sdkCommon), "typescript-public")
		}
		if !slices.Equal(groups, want) {
			t.Errorf("%s: groups %v, want %v", name, groups, want)
		}
	}
}

// A project's mise.toml is complete: it pins the tools its own tasks need. So the same tool is
// pinned in several files of this repo, and this holds them to one version each.
func TestTheExamplesPinTheSameTools(t *testing.T) {
	pin := regexp.MustCompile(`(?m)^([a-z][a-z0-9_-]*) = "([^"]+)"`)
	seen := map[string]string{} // tool -> "version (file)"
	files := []string{"../../mise.toml"}
	for _, name := range examples(t) {
		files = append(files, filepath.Join("../..", name, "mise.toml"))
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		_, tools, _ := strings.Cut(string(content), "\n[tools]\n")
		tools, _, _ = strings.Cut(tools, "\n[")
		pins := pin.FindAllStringSubmatch(tools, -1)
		if len(pins) == 0 {
			t.Errorf("%s: no [tools]", file)
		}
		for _, m := range pins {
			here := m[2] + " (" + file + ")"
			if had, ok := seen[m[1]]; ok && !strings.HasPrefix(had, m[2]+" ") {
				t.Errorf("%s is pinned to %s and to %s", m[1], had, here)
			}
			seen[m[1]] = here
		}
	}
}

// The tasks the projects share are written once, in tasks/ (what a project made by charter new
// includes from GitHub): every project includes the shared folder and its language's, and defines
// none of their tasks itself, except setup in a TypeScript project, which first builds this repo's
// TypeScript library (ts/), its local package @charter/ts. One line in each, so what is learned in
// one project holds in the others.
func TestTheProjectsIncludeTheSharedTasks(t *testing.T) {
	includedBy := func(folder string) map[string]bool {
		tasks := map[string]bool{}
		files, _ := filepath.Glob(filepath.Join("../../tasks", folder, "*.toml"))
		for _, file := range files {
			content, _ := os.ReadFile(file)
			for _, m := range includedTaskDef.FindAllStringSubmatch(string(content), -1) {
				tasks[m[1]] = true
			}
		}
		if len(tasks) == 0 {
			t.Fatalf("tasks/%s defines no tasks", folder)
		}
		return tasks
	}
	for _, name := range examples(t) {
		lang := "go"
		if strings.HasSuffix(name, "-ts") {
			lang = "ts"
		}
		file := filepath.Join("../..", name, "mise.toml")
		content, _ := os.ReadFile(file)
		if want := `includes = ["../../tasks/shared", "../../tasks/` + lang + `"]`; !strings.Contains(string(content), want) {
			t.Errorf("%s: no %s", name, want)
		}
		if !strings.Contains(string(content), "\nCHARTER_TOOL = \"go run ../../cmd/charter\"\n") {
			t.Errorf("%s: CHARTER_TOOL is not this checkout's tool", name)
		}
		shared := includedBy("shared")
		for task := range includedBy(lang) {
			shared[task] = true
		}
		for _, m := range taskDef.FindAllStringSubmatch(string(content), -1) {
			if shared[m[1]] && !(lang == "ts" && m[1] == "setup") {
				t.Errorf("%s defines %s, which tasks/ already does", name, m[1])
			}
		}
		if lang == "ts" && !strings.Contains(string(content), `run = "npm ci --no-fund --no-audit --prefix ../../ts && npm run build --prefix ../../ts && npm ci --no-fund --no-audit"`) {
			t.Errorf("%s: setup does not build the TypeScript library first", name)
		}
		if strings.Contains(string(content), "go run ../../cmd/charter ") {
			t.Errorf("%s: a task runs the tool as go run ../../cmd/charter, not {{env.CHARTER_TOOL}}", name)
		}
	}
}

// The SDK commands work on the project's one API, and say how to generate what is missing.
func TestSDKCommandsSayWhatTheyNeed(t *testing.T) {
	t.Chdir(t.TempDir())
	for want, err := range map[string]error{
		"sdk-check needs <group>":        sdkCheck(nil),
		"mise run sdk:gen go":            sdkCheck([]string{"go"}),
		"cli-build takes no arguments":   cliBuild([]string{"notes"}),
		"mise run sdk:gen cli":           cliBuild(nil),
		"sdk-gen needs <group>":          sdkGen(nil),
		"sdk-publish takes no arguments": sdkPublish([]string{"notes"}),
		"dist-sdk takes no arguments":    distSDK([]string{"notes"}),
		"dist-cli takes no arguments":    distCLI([]string{"notes"}),
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want an error with %q", err, want)
		}
	}
}

// What marks a project, and that a command finds it from any folder inside it.
func TestAProjectIsAMiseFileBesideAFernFolder(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"repo/one/fern", "repo/one/api/deep", "repo/two"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"repo/mise.toml", "repo/one/mise.toml", "repo/two/mise.toml"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("[tasks.check]\nrun = \"true\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for start, want := range map[string]string{"repo/one": "repo/one", "repo/one/api/deep": "repo/one", "repo/two": "", "repo": ""} {
		got, ok := project(filepath.Join(root, start))
		if want == "" && ok || want != "" && got != filepath.Join(root, want) {
			t.Errorf("from %s: project %q (%v), want %q", start, got, ok, want)
		}
	}
	below, err := projectsBelow(filepath.Join(root, "repo"))
	if err != nil || len(below) != 1 || below[0] != filepath.Join(root, "repo/one") {
		t.Errorf("projects below repo: %v (%v), want only repo/one", below, err)
	}
	if !tasksOf(filepath.Join(root, "repo/one/mise.toml"))["check"] {
		t.Error("tasksOf does not find [tasks.check]")
	}
}

// Every example sets its own API_URL and API_PORT, which is what `each` keeps apart between them.
func TestEveryExampleHasItsOwnSettings(t *testing.T) {
	ports := map[string]string{}
	for _, name := range examples(t) {
		file := filepath.Join("../..", name, "mise.toml")
		settings := settingsOf(file)
		if !settings["API_URL"] || !settings["API_PORT"] || settings["CLOUDFLARE_API_TOKEN"] {
			t.Errorf("%s: [env] sets %v, want API_URL and API_PORT", name, settings)
		}
		content, _ := os.ReadFile(file)
		if want := "default='https://charter-" + filepath.Base(name) + ".gedw99.workers.dev'"; !strings.Contains(string(content), want) {
			t.Errorf("%s: no API_URL with %s", name, want)
		}
		port := regexp.MustCompile(`API_PORT', default='(\d+)'`).FindStringSubmatch(string(content))
		if port == nil {
			t.Errorf("%s: no default for API_PORT", name)
			continue
		}
		if other := ports[port[1]]; other != "" {
			t.Errorf("%s: its default port %s is also that of %s", name, port[1], other)
		}
		ports[port[1]] = name
	}
}

// sdk-ready generates a group again when the Fern folder changed since it was generated (#41).
func TestAGeneratedSDKIsStaleWhenItsSpecsChange(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, dir := range []string{fernDir, "sdk/out/go"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	spec := filepath.Join(fernDir, "openapi.json")
	os.WriteFile(spec, []byte(`{"a":1}`), 0o644)
	if sdkFresh("go") {
		t.Error("fresh without a note of what it was made from")
	}
	source, _ := sdkSourceHash()
	os.WriteFile(sdkStamp("go"), []byte(source+"\n"), 0o644)
	if !sdkFresh("go") {
		t.Error("stale right after it was made")
	}
	os.WriteFile(spec, []byte(`{"a":2}`), 0o644)
	if sdkFresh("go") {
		t.Error("fresh after the spec changed")
	}
}
