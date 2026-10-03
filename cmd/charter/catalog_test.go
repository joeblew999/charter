package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeRepo is a repo as the GitHub API shows it to catalog.
type fakeRepo struct {
	topics  []string
	fork    bool
	files   map[string]string // every file in it, by path
	release string
	assets  []string
}

// fakeGitHubAPI serves the calls catalog makes, for the owner zeta.
func fakeGitHubAPI(t *testing.T, repos map[string]fakeRepo) githubClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" {
			http.Error(w, "no token", 401)
			return
		}
		reply := func(v any) { json.NewEncoder(w).Encode(v) }
		if r.URL.Path == "/users/zeta/repos" {
			var list []map[string]any
			for _, name := range sortedKeys(anyMap(repos)) {
				list = append(list, map[string]any{"full_name": "zeta/" + name, "description": name + "'s description",
					"topics": repos[name].topics, "default_branch": "main", "fork": repos[name].fork})
			}
			reply(list)
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/repos/zeta/"), "/", 2)
		repo, ok := repos[parts[0]]
		if !ok || len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		switch rest := parts[1]; {
		case rest == "git/trees/main" && r.URL.Query().Get("recursive") == "1":
			var tree []map[string]string
			for path := range repo.files {
				tree = append(tree, map[string]string{"path": path, "type": "blob"})
			}
			reply(map[string]any{"tree": tree})
		case strings.HasPrefix(rest, "contents/") && r.URL.Query().Get("ref") == "main":
			content, ok := repo.files[strings.TrimPrefix(rest, "contents/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write([]byte(content))
		case rest == "releases/latest" && repo.release != "":
			var assets []map[string]string
			for _, name := range repo.assets {
				assets = append(assets, map[string]string{"name": name})
			}
			reply(map[string]any{"tag_name": repo.release, "assets": assets})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return githubClient{server.URL, "t"}
}

// The catalog of an owner: the charter repos, what each publishes, and who pins it, read only from
// the pins in the repos.
func TestCatalog(t *testing.T) {
	client := fakeGitHubAPI(t, map[string]fakeRepo{
		"billing-api": {topics: []string{"api", "charter"}, release: "v1.2.0", assets: []string{"billing-cli-linux-amd64", "billing-specs.tar.gz"},
			files: map[string]string{
				"mise.toml":     "[tools]\n\"github:joeblew999/charter\" = \"0.9.3\"\n\n[env]\nAPI_URL = \"{{ get_env(name='API_URL', default='https://billing.example.workers.dev') }}\"\n",
				"go.mod":        "module github.com/zeta/billing-api\n\nrequire github.com/joeblew999/charter/go v0.9.3\n",
				"sdk/go/go.mod": "module github.com/zeta/billing-api/sdk/go\n", "fern/openapi.json": "{}", "fern/asyncapi.json": "{}",
				"tasks/billing/billing.toml": "", "test/soak/go.mod": "module soak\n",
			}},
		"rig": {files: map[string]string{
			"mise.toml": "[tools]\nnu = \"0.116.0\"\n# the CLI\n\"github:zeta/billing-api\" = { version = \"1.1.0\", os = [\"macos\"], bin = \"billing\" }\n\n" +
				"[task_config]\nincludes = [\"git::https://github.com/zeta/billing-api.git//tasks/billing?ref=v1.2.0\"]\n",
			"web/package.json":                      `{"name": "web", "private": true, "devDependencies": {"left-pad": "1.0.0", "billing-sdk": "github:zeta/billing-api#v1.0.0"}}`,
			"web/node_modules/billing/package.json": `{"name": "billing", "dependencies": {"x": "github:zeta/billing-api#v0.1.0"}}`,
		}},
		"vm": {files: map[string]string{
			"go.mod": "module github.com/zeta/vm\n\nrequire (\n\tgithub.com/zeta/billing-api/sdk/go v0.0.0-20261003054342-50e83ee58abf\n\tgithub.com/zeta/rig v0.1.0 // indirect\n)\n",
		}},
		"unrelated": {files: map[string]string{"go.mod": "module github.com/zeta/unrelated\n\nrequire github.com/zeta/vm v0.1.0\n"}},
		"fork":      {fork: true, files: map[string]string{"go.mod": "module x\n\nrequire github.com/zeta/billing-api/sdk/go v1.2.0\n"}},
	})
	found, err := readCatalog(client, "zeta", time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, repo := range found.Repos {
		names = append(names, repo.Repo)
	}
	// billing-api has the topic; rig and vm pin it; unrelated pins only vm, and fork is a fork.
	if !slices.Equal(names, []string{"zeta/billing-api", "zeta/rig", "zeta/vm"}) {
		t.Fatalf("charter repos: %v", names)
	}
	billing := found.Repos[0]
	want := producer{Repo: "zeta/billing-api", Description: "billing-api's description", Topic: true, Release: "v1.2.0",
		Binaries: []string{"billing-cli-linux-amd64"}, Files: []string{"billing-specs.tar.gz"},
		Specs: []string{"fern/asyncapi.json", "fern/openapi.json"}, GoModules: []string{"github.com/zeta/billing-api", "github.com/zeta/billing-api/sdk/go"},
		Tasks: []string{"tasks/billing"}, Deployed: []string{"https://billing.example.workers.dev"},
		Consumers: []pin{
			{Repo: "zeta/rig", File: "mise.toml", Kind: "tool", Name: "github:zeta/billing-api", Version: "1.1.0"},
			{Repo: "zeta/rig", File: "mise.toml", Kind: "tasks", Name: "tasks/billing", Version: "v1.2.0"},
			{Repo: "zeta/rig", File: "web/package.json", Kind: "npm", Name: "billing-sdk", Version: "v1.0.0"},
			{Repo: "zeta/vm", File: "go.mod", Kind: "go", Name: "github.com/zeta/billing-api/sdk/go", Version: "v0.0.0-20261003054342-50e83ee58abf"},
		}}
	slices.Sort(billing.Specs)
	for i := range billing.Consumers {
		billing.Consumers[i].Target = ""
	}
	got, _ := json.MarshalIndent(billing, "", " ")
	expected, _ := json.MarshalIndent(want, "", " ")
	if string(got) != string(expected) {
		t.Errorf("billing-api:\n%s\nwant\n%s", got, expected)
	}
	if len(found.Repos[1].Consumers) != 0 || found.Repos[1].Consumers == nil {
		t.Errorf("rig: a pin marked indirect is not one: %+v", found.Repos[1].Consumers)
	}

	page := catalogMarkdown(found, true)
	for _, line := range []string{
		"---\ntitle: Catalog\nnav_order: 50\n---\n",
		"from GitHub at 2026-10-03T09:00:00Z",
		"| Release | `v1.2.0` |",
		"| zeta/rig | `mise.toml` | tool `github:zeta/billing-api` | `1.1.0` | behind v1.2.0 |",
		"| zeta/rig | `mise.toml` | tasks `tasks/billing` | `v1.2.0` | latest |",
		"| zeta/vm | `go.mod` | go `github.com/zeta/billing-api/sdk/go` | `v0.0.0-20261003054342-50e83ee58abf` | a commit, not a release |",
		"## zeta/rig\n\nrig's description\n\nIt publishes nothing another repo can pin",
	} {
		if !strings.Contains(page, line) {
			t.Errorf("the page has no %q:\n%s", line, page)
		}
	}
	if strings.Contains(page, "{{") {
		t.Error("the page has two curly braces, which the docs site reads as template code")
	}
}

// spec-diff in a repo with releases: it compares with the newest version tag before HEAD's, fails on
// a breaking change unless the release is a major one, and names the consumers from the catalog.
func TestSpecDiffAgainstTheLastRelease(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	spec, err := os.ReadFile("../../examples/notes-go/fern/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := write(filepath.Join(dir, "fern", "openapi.json"), string(spec)); err != nil {
		t.Fatal(err)
	}
	git("init", "-q", "-b", "main")
	git("remote", "add", "origin", "https://github.com/zeta/billing-api.git")
	git("add", ".")
	git("commit", "-q", "-m", "one")
	git("tag", "v0.1.0")
	if err := write(filepath.Join(dir, "fern", "openapi.json"), strings.Replace(string(spec), `"/api/hello"`, `"/api/hi"`, 1)); err != nil {
		t.Fatal(err)
	}
	catalogFile := filepath.Join(dir, "catalog.json")
	catalogJSON, _ := json.Marshal(catalog{Owner: "zeta", Repos: []producer{{Repo: "zeta/billing-api", Consumers: []pin{{Repo: "zeta/vm", File: "go.mod", Kind: "go", Name: "github.com/zeta/billing-api/sdk/go", Version: "v0.1.0"}}}}})
	if err := os.WriteFile(catalogFile, catalogJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("GITHUB_REF_TYPE", "")
	if err := specDiff([]string{"-catalog", catalogFile}); err == nil || !strings.Contains(err.Error(), "1 breaking changes since v0.1.0") {
		t.Errorf("a removed operation in the working tree: %v", err)
	}
	if err := specDiff([]string{"-tag", "v0.1.1"}); err == nil {
		t.Error("a breaking change passed in a patch release")
	}
	if err := specDiff([]string{"-tag", "v0.2.0"}); err != nil {
		t.Errorf("a breaking change in a major release (v0: the minor number): %v", err)
	}
	git("commit", "-q", "-am", "two")
	git("tag", "v0.2.0")
	if err := specDiff(nil); err != nil {
		t.Errorf("nothing changed since v0.2.0: %v", err)
	}
	t.Setenv("GITHUB_REF_TYPE", "tag")
	t.Setenv("GITHUB_REF_NAME", "v0.2.0")
	if err := specDiff(nil); err != nil {
		t.Errorf("the release workflow on v0.2.0 compares with v0.1.0, a major release in v0: %v", err)
	}
	t.Setenv("GITHUB_REF_NAME", "v0.1.1")
	if err := specDiff([]string{"-from", "v0.1.0", "-to", "v0.2.0"}); err == nil {
		t.Error("v0.2.0 against v0.1.0 for a patch release")
	}
}
