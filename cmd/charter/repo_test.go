package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	if len(github.writes) > 0 {
		t.Fatalf("-check changed something: %v", github.writes)
	}

	if err := keep(false); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"docs", ".github", "renovate.json", "AGENTS.md"} {
		if exists(filepath.Join(root, file)) {
			t.Errorf("charter repo wrote %s: it writes no file (charter adopt writes the kit)", file)
		}
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

// charter repo in a repo that is not a project and lists none (what charter adopt leaves): the docs
// site and charter's pages, the issue forms, renovate.json and the one workflow any repo takes
// (charter.yml, unless workflow = false), beside the repo's own workflows and forms and none of a
// project's. A file at a name the host fixes (docs/_config.yml) without charter's mark is the repo's:
// kept, and not drift. In charter's own places, an edited file is written again.
func TestKitInARepoThatIsNotAProject(t *testing.T) {
	forms := formFiles()
	for name, content := range forms {
		if charters, _, ok := markOf(content); !ok || !charters {
			t.Errorf("cmd/charter/kit/.github/%s has no mark: a repo's own form is known by lacking it", name)
		}
		if name != "ISSUE_TEMPLATE/config.yml" && !strings.HasPrefix(name, "ISSUE_TEMPLATE/charter-") {
			t.Errorf("cmd/charter/kit/.github/%s: charter's forms are named charter-*.yml, beside the repo's own", name)
		}
		// YAML ends an unquoted value at a colon and a space, and GitHub then offers no such form.
		for _, m := range regexp.MustCompile(`(?m)^ *(?:description|label|name|title): ([^"'|\n].*)$`).FindAllStringSubmatch(string(content), -1) {
			if strings.Contains(m[1], ": ") {
				t.Errorf("cmd/charter/kit/.github/%s: a colon and a space in an unquoted value: %s", name, m[1])
			}
		}
	}
	root := t.TempDir()
	github := &fakeGitHub{labels: map[string][2]string{}}
	gh = github.gh
	t.Cleanup(func() {
		gh = func(string, ...string) (string, error) { return "", errors.New("no gh in tests") }
	})
	edited := "# Written by `charter repo`: don't edit.\nname: Mine now\n"
	for file, content := range map[string]string{
		"charter.toml":                               "description = \"Field notes\"\n",
		"mise.toml":                                  "[tasks.test]\nrun = \"true\"\n",
		".github/ISSUE_TEMPLATE/charter-bug.yml":     "name: no mark\n",
		".github/ISSUE_TEMPLATE/charter-feature.yml": edited,
		".github/ISSUE_TEMPLATE/bug.yml":             "name: Ours\n", // the repo's, at the name charter once used
		".github/workflows/stages.yml":               "name: stages\n",
		"docs/_config.yml":                           "title: our own site\n",
	} {
		if err := write(filepath.Join(root, filepath.FromSlash(file)), content); err != nil {
			t.Fatal(err)
		}
	}
	if err := kitKeep(root, false); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"docs/charter/README.md", "docs/charter/writing.md", "docs/charter/tasks.md", "renovate.json", ".github/workflows/charter.yml", ".github/workflows/stages.yml", ".github/ISSUE_TEMPLATE/config.yml", ".github/ISSUE_TEMPLATE/charter-plan.yml", ".github/ISSUE_TEMPLATE/charter-upstream.yml"} {
		if !exists(filepath.Join(root, filepath.FromSlash(file))) {
			t.Errorf("no %s", file)
		}
	}
	for _, file := range []string{".github/labels.tsv", ".github/workflows/repo-check.yml", "docs/writing.md", "docs/repo.md"} {
		if exists(filepath.Join(root, filepath.FromSlash(file))) {
			t.Errorf("charter wrote %s, where it writes nothing now", file)
		}
	}
	for _, file := range []string{"AGENTS.md", "CLAUDE.md"} {
		if have, _ := os.ReadFile(filepath.Join(root, file)); string(have) != kitFile(file) {
			t.Errorf("the kit's %s was not written, the same in every repo", file)
		}
	}
	for _, file := range []string{".github/workflows/charter-check.yml", ".github/workflows/charter-deploy.yml", ".github/workflows/charter-release.yml", ".github/workflows/charter-sdk-check.yml", "fern"} {
		if exists(filepath.Join(root, filepath.FromSlash(file))) {
			t.Errorf("the kit wrote %s in a repo that is not a project", file)
		}
	}
	for file, want := range map[string]string{".github/ISSUE_TEMPLATE/charter-bug.yml": string(forms["ISSUE_TEMPLATE/charter-bug.yml"]), ".github/ISSUE_TEMPLATE/bug.yml": "name: Ours\n", "docs/_config.yml": "title: our own site\n"} {
		if have, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(file))); string(have) != want {
			t.Errorf("%s:\n%s", file, have)
		}
	}
	if have, _ := os.ReadFile(filepath.Join(root, ".github", "ISSUE_TEMPLATE", "charter-feature.yml")); string(have) != string(forms["ISSUE_TEMPLATE/charter-feature.yml"]) {
		t.Error("a form charter wrote, edited by hand, was not written again")
	}
	if err := kitKeep(root, true); err != nil {
		t.Errorf("-check with files of the repo's own at charter's paths: %v", err)
	}
	// The workflow: one job on Linux, on pull requests and pushes to main, every step a mise task of
	// tasks/repo or an action pinned to a release, reading GitHub with the run's token, writing nothing.
	workflow, _ := os.ReadFile(filepath.Join(root, ".github", "workflows", repoWorkflow))
	for _, want := range []string{"name: charter\n", "    branches: [main]\n", "  pull_request:\n", "\npermissions:\n  contents: read\n  issues: read\n  pull-requests: read\n  pages: read\n",
		"GH_TOKEN: ${{ github.token }}", "    runs-on: ubuntu-24.04\n", "- run: mise run repo:ci\n"} {
		if !strings.Contains(string(workflow), want) {
			t.Errorf("%s: no %q in\n%s", repoWorkflow, want, workflow)
		}
	}
	if strings.Count(string(workflow), "runs-on:") != 1 || strings.Contains(string(workflow), ": write") || strings.Contains(string(workflow), "uses: joeblew999/") {
		t.Errorf("%s: more than one job, a permission to write, or a call to another workflow:\n%s", repoWorkflow, workflow)
	}
	for _, m := range regexp.MustCompile(`- run: (.*)`).FindAllStringSubmatch(string(workflow), -1) {
		if task, ok := strings.CutPrefix(m[1], "mise run "); !ok || !slices.Contains(repoTasks, task) {
			t.Errorf("%s runs %q, which is not a task of tasks/repo", repoWorkflow, m[1])
		}
	}
	// workflow = false: charter writes none, and one that is there is the repo's.
	if err := os.WriteFile(filepath.Join(root, "charter.toml"), []byte("description = \"Field notes\"\nworkflow = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(root, ".github", "workflows", repoWorkflow))
	if err := kitKeep(root, false); err != nil || exists(filepath.Join(root, ".github", "workflows", repoWorkflow)) {
		t.Errorf("workflow = false: %v, and the workflow is there: %v", err, exists(filepath.Join(root, ".github", "workflows", repoWorkflow)))
	}
	// A project gets its own workflows, whose check holds its docs, and not this one.
	project := t.TempDir()
	for file, content := range map[string]string{"charter.toml": "description = \"An API\"\n", "mise.toml": "", "fern/generators.yml": "groups:\n  go:\n"} {
		if err := write(filepath.Join(project, filepath.FromSlash(file)), content); err != nil {
			t.Fatal(err)
		}
	}
	if err := kitKeep(project, false); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(project, ".github", "workflows", repoWorkflow)) || !exists(filepath.Join(project, ".github", "workflows", "charter-check.yml")) {
		t.Error("a project: its own workflows, and no charter.yml")
	}
	for _, file := range []string{"AGENTS.md", "CLAUDE.md", "docs/README.md", "docs/rules.md"} {
		if exists(filepath.Join(project, filepath.FromSlash(file))) {
			t.Errorf("the kit wrote %s in a project, whose own it is", file)
		}
	}

	// Deleted, the form is charter's again.
	os.Remove(filepath.Join(root, ".github", "ISSUE_TEMPLATE", "charter-bug.yml"))
	if err := kitKeep(root, true); err == nil {
		t.Error("-check passed without the bug form")
	}
}

// charter's own places are exactly what the release writes: a page in docs/charter/ or a form
// charter-*.yml that it does not write is removed, and one edited or without the mark is written
// again. A form of the repo's own, under any other name, stays.
func TestKitOwnsItsPlaces(t *testing.T) {
	root := t.TempDir()
	github := &fakeGitHub{labels: map[string][2]string{}}
	gh = github.gh
	t.Cleanup(func() {
		gh = func(string, ...string) (string, error) { return "", errors.New("no gh in tests") }
	})
	for file, content := range map[string]string{
		"charter.toml":                           "description = \"Field notes\"\n",
		"mise.toml":                              "[tasks.test]\nrun = \"true\"\n",
		"docs/charter/old.md":                    "# a page an earlier release wrote\n",
		"docs/charter/rules.md":                  "# no mark\n",
		".github/ISSUE_TEMPLATE/charter-old.yml": "name: Old\n",
		".github/ISSUE_TEMPLATE/bug.yml":         "name: Ours\n",
	} {
		if err := write(filepath.Join(root, filepath.FromSlash(file)), content); err != nil {
			t.Fatal(err)
		}
	}
	if err := kitKeep(root, true); err == nil {
		t.Error("-check passed with what charter does not write in its own places")
	}
	if err := kitKeep(root, false); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"docs/charter/old.md", ".github/ISSUE_TEMPLATE/charter-old.yml"} {
		if exists(filepath.Join(root, filepath.FromSlash(gone))) {
			t.Errorf("%s is still in charter's place", gone)
		}
	}
	if have, _ := os.ReadFile(filepath.Join(root, "docs", "charter", "rules.md")); string(have) != kitFile("docs/charter/rules.md") {
		t.Error("docs/charter/rules.md is not charter's")
	}
	if have, _ := os.ReadFile(filepath.Join(root, ".github", "ISSUE_TEMPLATE", "bug.yml")); string(have) != "name: Ours\n" {
		t.Error("the repo's own form was changed")
	}
	if err := kitKeep(root, true); err != nil {
		t.Errorf("-check after a run: %v", err)
	}
}

// The kit's renovate.json: valid JSON, with the managers for every kind of pin catalog reads.
func TestRenovatePreset(t *testing.T) {
	var preset struct {
		EnabledManagers []string
		CustomManagers  []struct{ MatchStrings []string }
	}
	if err := json.Unmarshal([]byte(kitFile("renovate.json")), &preset); err != nil {
		t.Fatal(err)
	}
	for _, manager := range []string{"mise", "gomod", "npm", "custom.regex"} {
		if !slices.Contains(preset.EnabledManagers, manager) {
			t.Errorf("the preset has no %s manager", manager)
		}
	}
	include := regexp.MustCompile(strings.ReplaceAll(strings.ReplaceAll(preset.CustomManagers[0].MatchStrings[0], "(?<", "(?P<"), `\"`, `"`))
	m := include.FindStringSubmatch(`"git::https://github.com/joeblew999/charter.git//tasks/shared?ref=v0.9.3"`)
	if m == nil || m[1] != "joeblew999/charter" || m[2] != "v0.9.3" {
		t.Errorf("the include manager reads %q", m)
	}
}

// kitKeep is charter adopt's part that writes the kit: with check, an error if a file differs.
func kitKeep(dir string, check bool) error {
	changed, _, _, err := writeKit(dir, check)
	if err == nil && check && len(changed) > 0 {
		return fmt.Errorf("the kit differs: %v", changed)
	}
	return err
}
