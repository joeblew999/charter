package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The format of charter.toml: what it takes, and what it refuses rather than ignores.
func TestCharterToml(t *testing.T) {
	config, err := parseCharterToml(`# a comment
description = "An API: \"quoted\" too"
topics = ["api", "cloudflare-workers"]

homepage = "https://example.com/"
projects = []
`)
	if err != nil {
		t.Fatal(err)
	}
	if config.Description != `An API: "quoted" too` || !slices.Equal(config.Topics, []string{"api", "cloudflare-workers", "charter"}) || !config.Renovate ||
		config.Homepage != "https://example.com/" || config.Docs != "docs" || config.Projects == nil || len(config.Projects) != 0 {
		t.Errorf("parsed %+v", config)
	}
	for text, why := range map[string]string{
		`topics = []`:                                                            "no description",
		"description = \"a\"\ncolor = \"red\"":                                   "an unknown key",
		"description = \"a\"\ndescription = \"b\"":                               "a key twice",
		`description = a`:                                                        "a string without quotes",
		"description = \"a\"\ntopics = \"api\"":                                  "a string for a list",
		"description = \"a\"\ntopics = [\"Big Topic\"]":                          "a topic GitHub refuses",
		"description = \"a\"\ndocs = \"site\"":                                   "a folder Pages cannot serve",
		"description = \"a\"\n[section]":                                         "a table",
		"description = \"a\"\ntopics = [\"a\" \"b\"]":                            "a list without commas",
		"description = \"a\" # what it is":                                       "a comment after a value",
		"description = \"a\"\nprojects = [\"x\",, \"y\"]":                        "an empty item",
		"description = \"a\"\nrenovate = \"no\"":                                 "a string for true or false",
		"description = \"a\"\ntopics = [" + strings.Repeat(`"t", `, 19) + `"t"]`: "20 topics and charter's",
	} {
		if _, err := parseCharterToml(text); err == nil {
			t.Errorf("%s: no error for %q", why, text)
		}
	}
	// This repo's own, and what charter new writes.
	own, err := os.ReadFile("../../charter.toml")
	if err != nil {
		t.Fatal(err)
	}
	if config, err := parseCharterToml(string(own)); err != nil || len(config.Projects) == 0 {
		t.Errorf("charter.toml: %+v, %v", config, err)
	} else {
		for _, project := range config.Projects {
			if !isProject(filepath.Join("../..", project)) {
				t.Errorf("charter.toml lists %s, which is not a project", project)
			}
		}
	}
	if config, err := parseCharterToml("description = \"a\"\nrenovate = false"); err != nil || config.Renovate {
		t.Errorf("renovate = false: %+v, %v", config, err)
	}
	if config, err := parseCharterToml(charterTomlFor("billing-api: an API")); err != nil || config.Description != "billing-api: an API" || !slices.Equal(config.Topics, []string{"charter"}) {
		t.Errorf("what new writes: %+v, %v", config, err)
	}
}

// fakeGitHub is a repo on GitHub as far as the gh commands charter repo runs can see it.
type fakeGitHub struct {
	description, homepage string
	topics                []string
	labels                map[string][2]string // name: color, description
	used                  map[string]bool      // labels an issue or pull request has
	pages                 *[3]string           // build type, branch, path; nil: off
	writes                []string             // the commands that changed something
}

func (f *fakeGitHub) gh(dir string, args ...string) (string, error) {
	command := strings.Join(args, " ")
	flag := func(name string) string {
		if i := slices.Index(args, name); i >= 0 && i+1 < len(args) {
			return args[i+1]
		}
		return ""
	}
	write := func() { f.writes = append(f.writes, command) }
	switch {
	case strings.HasPrefix(command, "repo view"):
		topics := []map[string]string{}
		for _, topic := range f.topics {
			topics = append(topics, map[string]string{"name": topic})
		}
		out, err := json.Marshal(map[string]any{"nameWithOwner": "Zeta/billing-api", "name": "billing-api", "description": f.description,
			"homepageUrl": f.homepage, "repositoryTopics": topics, "defaultBranchRef": map[string]string{"name": "main"}})
		return string(out), err
	case strings.HasPrefix(command, "repo edit Zeta/billing-api"):
		write()
		if args[3] == "--description" {
			f.description = args[4]
		}
		if args[3] == "--homepage" {
			f.homepage = args[4]
		}
		if add := flag("--add-topic"); add != "" {
			f.topics = append(f.topics, strings.Split(add, ",")...)
		}
		for _, topic := range strings.Split(flag("--remove-topic"), ",") {
			f.topics = slices.DeleteFunc(f.topics, func(t string) bool { return t == topic })
		}
		return "", nil
	case strings.HasPrefix(command, "label list -R Zeta/billing-api"):
		var labels []map[string]string
		for name, label := range f.labels {
			labels = append(labels, map[string]string{"name": name, "color": strings.ToUpper(label[0]), "description": label[1]})
		}
		out, err := json.Marshal(labels)
		return string(out), err
	case strings.HasPrefix(command, "label create"):
		write()
		f.labels[args[2]] = [2]string{flag("--color"), flag("--description")}
		return "", nil
	case strings.HasPrefix(command, "label delete"):
		write()
		delete(f.labels, args[2])
		return "", nil
	case strings.HasPrefix(command, "issue list") || strings.HasPrefix(command, "pr list"):
		if f.used[flag("--label")] && args[0] == "issue" {
			return `[{"number":1}]`, nil
		}
		return "[]", nil
	case command == "api repos/Zeta/billing-api/pages":
		if f.pages == nil {
			return "", errors.New("gh api: exit status 1: gh: Not Found (HTTP 404)")
		}
		out, err := json.Marshal(map[string]any{"build_type": f.pages[0], "source": map[string]string{"branch": f.pages[1], "path": f.pages[2]}})
		return string(out), err
	case strings.HasPrefix(command, "api -X POST repos/Zeta/billing-api/pages"), strings.HasPrefix(command, "api -X PUT repos/Zeta/billing-api/pages"):
		write()
		f.pages = &[3]string{"legacy"}
		for _, arg := range args {
			if branch, ok := strings.CutPrefix(arg, "source[branch]="); ok {
				f.pages[1] = branch
			}
			if path, ok := strings.CutPrefix(arg, "source[path]="); ok {
				f.pages[2] = path
			}
		}
		return "", nil
	}
	return "", errors.New("the fake has no " + command)
}

// charter repo on a new repo: -check changes nothing and fails; then it brings everything in line;
// then -check passes and a second run changes nothing.
func TestRepoKeepsTheRepo(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"api/fern", "web"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "api", "mise.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	github := &fakeGitHub{description: "old", topics: []string{"stale", "api"}, used: map[string]bool{"question": true},
		labels: map[string][2]string{"bug": {"ff0000", "old"}, "enhancement": {"a2eeef", ""}, "question": {"d876e3", ""}, "area:web": {"c5def5", "ours"}}}
	gh = github.gh
	t.Cleanup(func() {
		gh = func(string, ...string) (string, error) { return "", errors.New("no gh in tests") }
	})
	keep := func(check bool) error {
		t.Helper()
		return keepRepo(root, check)
	}

	if err := keep(true); err == nil {
		t.Fatal("-check passed on a repo without charter.toml")
	}
	toml := "description = \"Billing\"\ntopics = [\"api\", \"billing\"]\nprojects = [\"web\"]\n"
	if err := os.WriteFile(filepath.Join(root, "charter.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := keep(true); err == nil || !strings.Contains(err.Error(), "not a project") {
		t.Fatalf("a listed folder that is not a project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "charter.toml"), []byte(strings.Replace(toml, "web", "api", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := keep(true); err == nil {
		t.Fatal("-check passed on a repo that differs")
	}
	if len(github.writes) > 0 || exists(filepath.Join(root, "docs")) || exists(filepath.Join(root, ".github")) {
		t.Fatalf("-check changed something: %v", github.writes)
	}

	if err := keep(false); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"docs/_config.yml", "docs/writing.md", "docs/llms.txt", ".github/labels.tsv", ".github/ISSUE_TEMPLATE/bug.yml", ".github/ISSUE_TEMPLATE/plan.yml", ".github/workflows/check.yml"} {
		if !exists(filepath.Join(root, file)) {
			t.Errorf("no %s", file)
		}
	}
	var renovate struct{ Extends []string }
	if raw, err := os.ReadFile(filepath.Join(root, "renovate.json")); err != nil || json.Unmarshal(raw, &renovate) != nil ||
		!slices.Equal(renovate.Extends, []string{"github>joeblew999/charter//" + renovatePresetPath}) {
		t.Errorf("renovate.json: %+v, %v", renovate, err)
	}
	if config, _ := os.ReadFile(filepath.Join(root, "docs", "_config.yml")); !strings.Contains(string(config), `description: "Billing"`) {
		t.Error("the docs site does not have charter.toml's description")
	}
	slices.Sort(github.topics)
	if github.description != "Billing" || github.homepage != "https://zeta.github.io/billing-api/" || !slices.Equal(github.topics, []string{"api", "billing", "charter"}) {
		t.Errorf("the repo is %q, %q, %v", github.description, github.homepage, github.topics)
	}
	if *github.pages != [3]string{"legacy", "main", "/docs"} {
		t.Errorf("Pages: %v", *github.pages)
	}
	rows, _ := labelRows()
	for _, row := range rows {
		if github.labels[row[0]] != [2]string{row[1], row[2]} {
			t.Errorf("label %s: %v", row[0], github.labels[row[0]])
		}
	}
	if _, ok := github.labels["enhancement"]; ok {
		t.Error("an unused default label was kept")
	}
	for _, kept := range []string{"question", "area:web"} {
		if _, ok := github.labels[kept]; !ok {
			t.Errorf("label %s was removed: a default one in use, or the repo's own", kept)
		}
	}

	github.writes = nil
	if err := keep(true); err != nil {
		t.Fatalf("-check after a run: %v", err)
	}
	if err := keep(false); err != nil || len(github.writes) > 0 {
		t.Fatalf("a second run changed %v (%v)", github.writes, err)
	}

	// A repo without projects gets no workflows; Pages built from a workflow is put back.
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "charter.toml"), []byte(`description = "Billing"`), 0o644); err != nil {
		t.Fatal(err)
	}
	github.pages = &[3]string{"workflow", "main", "/"}
	if err := keep(false); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(root, ".github", "workflows")) || !exists(filepath.Join(root, ".github", "labels.tsv")) {
		t.Error("without projects: the issue forms and labels, and no workflows")
	}
	if *github.pages != [3]string{"legacy", "main", "/docs"} || !slices.Equal(github.topics, []string{"charter"}) {
		t.Errorf("Pages %v, topics %v", *github.pages, github.topics)
	}

	// renovate = false: no renovate.json, and one the repo has is its own.
	if err := os.WriteFile(filepath.Join(root, "charter.toml"), []byte("description = \"Billing\"\nrenovate = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "renovate.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := keep(true); err != nil {
		t.Errorf("renovate = false and a renovate.json of the repo's own: %v", err)
	}
}

// The preset renovate.json takes: valid JSON, with the managers for every kind of pin catalog reads,
// and a release's renovate.json takes it at that release.
func TestRenovatePreset(t *testing.T) {
	var preset struct {
		EnabledManagers []string
		CustomManagers  []struct{ MatchStrings []string }
	}
	if err := json.Unmarshal(renovatePreset, &preset); err != nil {
		t.Fatal(err)
	}
	for _, manager := range []string{"mise", "gomod", "npm", "custom.regex"} {
		if !slices.Contains(preset.EnabledManagers, manager) {
			t.Errorf("the preset has no %s manager", manager)
		}
	}
	if !exists("../../" + renovatePresetPath + ".json") {
		t.Errorf("renovate.json extends %s.json, which is not there", renovatePresetPath)
	}
	if want := `"extends": ["github>joeblew999/charter//cmd/charter/renovate/charter#v1.2.3"]`; !strings.Contains(string(renovateConfig("v1.2.3")), want) {
		t.Errorf("renovateConfig: %s", renovateConfig("v1.2.3"))
	}
	include := regexp.MustCompile(strings.ReplaceAll(strings.ReplaceAll(preset.CustomManagers[0].MatchStrings[0], "(?<", "(?P<"), `\"`, `"`))
	m := include.FindStringSubmatch(`"git::https://github.com/joeblew999/charter.git//tasks/shared?ref=v0.9.3"`)
	if m == nil || m[1] != "joeblew999/charter" || m[2] != "v0.9.3" {
		t.Errorf("the include manager reads %q", m)
	}
}
