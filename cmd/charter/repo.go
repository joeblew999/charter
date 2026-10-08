package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// `charter repo` keeps a repo in shape from charter.toml at its root: the docs site, the issue forms
// and labels, the workflows when it has projects, the repo's description, homepage and topics on
// GitHub, and GitHub Pages. Any repo: one that is not a project and lists none gets everything but
// the workflows. An issue form of the repo's own (ownForms) is kept. Each item prints ok or changed, and running it again changes nothing.
// With -check it changes nothing and fails if an item differs, for CI. It calls the steps that
// `charter docs`, `charter workflows` and `charter labels` are on their own.

func init() {
	commands["repo"] = command{"[-check]",
		"keep the repo in shape from charter.toml at its root: docs site, issue forms, labels, workflows, description, homepage, topics, Pages; -check changes nothing and fails on drift", repoCommand}
	anywhere["repo"] = true
}

// charterToml is charter.toml: a few `key = "value"` and `key = ["a", "b"]` lines.
type charterToml struct {
	Description string   // the repo's description on GitHub, and the docs site's
	Topics      []string // the repo's topics on GitHub
	Homepage    string   // default: the GitHub Pages URL
	Docs        string   // the docs folder, default "docs"
	Projects    []string // folders that are charter projects; with any, the repo gets the workflows
	Renovate    bool     // write renovate.json, which takes charter's Renovate preset; default true
}

var (
	tomlLine   = regexp.MustCompile(`^([a-z]+)\s*=\s*(.*)$`)
	tomlString = `"(?:[^"\\]|\\.)*"`
	tomlList   = regexp.MustCompile(`^\[\s*(` + tomlString + `(?:\s*,\s*` + tomlString + `)*)?\s*,?\s*\]$`)
	tomlItem   = regexp.MustCompile(tomlString)
	topicName  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)
)

// parseCharterToml reads charter.toml. Only the standard library: the format is a small part of
// TOML, one key per line, and anything else is an error, not ignored.
func parseCharterToml(text string) (charterToml, error) {
	config := charterToml{Docs: "docs", Renovate: true}
	seen := map[string]bool{}
	for number, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		at := func(format string, a ...any) error {
			return fmt.Errorf("charter.toml, line %d: "+format, append([]any{number + 1}, a...)...)
		}
		m := tomlLine.FindStringSubmatch(line)
		if m == nil {
			return config, at(`want key = "value" or key = ["a", "b"]`)
		}
		key, value := m[1], m[2]
		if seen[key] {
			return config, at("%s twice", key)
		}
		seen[key] = true
		var text string
		var list []string
		switch key {
		case "description", "homepage", "docs":
			unquoted, err := strconv.Unquote(value)
			if err != nil || !strings.HasPrefix(value, `"`) {
				return config, at(`%s is a string in double quotes`, key)
			}
			text = unquoted
		case "renovate":
			if value != "true" && value != "false" {
				return config, at("renovate is true or false")
			}
		case "topics", "projects":
			if !tomlList.MatchString(value) {
				return config, at(`%s is a list: ["a", "b"]`, key)
			}
			list = []string{}
			for _, item := range tomlItem.FindAllString(value, -1) {
				unquoted, err := strconv.Unquote(item)
				if err != nil {
					return config, at("%s: %v", item, err)
				}
				list = append(list, unquoted)
			}
		default:
			return config, at("no key %q: description, topics, homepage, docs, projects, renovate", key)
		}
		switch key {
		case "description":
			config.Description = text
		case "homepage":
			config.Homepage = text
		case "docs":
			config.Docs = filepath.ToSlash(filepath.Clean(text))
		case "topics":
			config.Topics = list
		case "projects":
			config.Projects = list
		case "renovate":
			config.Renovate = value == "true"
		}
	}
	if config.Description == "" {
		return config, errors.New(`charter.toml needs description = "..."`)
	}
	for _, topic := range config.Topics {
		if !topicName.MatchString(topic) {
			return config, fmt.Errorf("charter.toml: topic %q: GitHub takes lower-case letters, digits and hyphens, up to 50", topic)
		}
	}
	if !slices.Contains(config.Topics, charterTopic) {
		config.Topics = append(config.Topics, charterTopic)
	}
	if len(config.Topics) > 20 {
		return config, errors.New("charter.toml: GitHub takes up to 20 topics, charter's own among them")
	}
	if config.Docs != "docs" && config.Docs != "." {
		return config, errors.New(`charter.toml: docs is "docs" or "." (GitHub Pages serves a site from one of those)`)
	}
	return config, nil
}

// charterTopic is the topic every repo charter repo keeps has: charter catalog finds them by it.
const charterTopic = "charter"

// The Renovate preset every charter repo takes (cmd/charter/renovate/charter.json), so a release of
// one repo opens a pull request in each repo that pins it.
//
//go:embed renovate/charter.json
var renovatePreset []byte

const renovatePresetPath = "cmd/charter/renovate/charter"

// renovateConfig is the renovate.json charter repo writes: charter's preset, at this tool's release
// (from a checkout: at main).
func renovateConfig(release string) []byte {
	preset := "github>" + strings.TrimPrefix(repoURL, "https://github.com/") + "//" + renovatePresetPath
	if release != "" {
		preset += "#" + release
	}
	return []byte("{\n  \"$schema\": \"https://docs.renovatebot.com/renovate-schema.json\",\n  \"extends\": [\"" + preset + "\"]\n}\n")
}

// charterTomlFor is the charter.toml that charter new and charter adopt write.
func charterTomlFor(description string) string {
	return "# The repo as charter repo keeps it (mise run repo). Topics: lower-case words, e.g. [\"api\", \"cloudflare-workers\"].\n" +
		"description = " + strconv.Quote(description) + "\ntopics = []\n"
}

// gh runs GitHub's command in dir and returns what it printed; on a failure the error says what gh
// said. Tests replace it.
var gh = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stderr bytes.Buffer
	cmd.Dir, cmd.Stderr = dir, &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh %s: %v: %s", strings.Join(args[:min(2, len(args))], " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func repoCommand(args []string) error {
	check := false
	flags("repo", args, func(f *flag.FlagSet) {
		f.BoolVar(&check, "check", false, "change nothing: fail if an item differs")
	})
	root, err := output(started, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New("repo runs in a git repo (git rev-parse --show-toplevel failed)")
	}
	return keepRepo(filepath.FromSlash(root), check)
}

// keepRepo brings the repo at root in line with its charter.toml, or with check only says what
// differs.
func keepRepo(root string, check bool) error {
	text, err := os.ReadFile(filepath.Join(root, "charter.toml"))
	if err != nil {
		return fmt.Errorf("no charter.toml at the repo's root (%s): charter adopt writes one (charter new, for a new project); it needs at least description = \"...\"", root)
	}
	config, err := parseCharterToml(string(text))
	if err != nil {
		return err
	}
	for _, project := range config.Projects {
		if !isProject(filepath.Join(root, filepath.FromSlash(project))) {
			return fmt.Errorf("charter.toml lists %s, which is not a project: %s", project, whatAProjectIs)
		}
	}
	out, err := gh(root, "repo", "view", "--json", "nameWithOwner,name,description,homepageUrl,repositoryTopics,defaultBranchRef")
	if err != nil {
		return err
	}
	var repo githubRepo
	if err := json.Unmarshal([]byte(out), &repo); err != nil {
		return err
	}
	owner, _, _ := strings.Cut(repo.NameWithOwner, "/")
	if config.Homepage == "" {
		config.Homepage = "https://" + strings.ToLower(owner) + ".github.io/" + repo.Name + "/"
	}

	edit := func(flag, have, want string) func() ([]string, error) {
		return func() ([]string, error) {
			if have == want {
				return nil, nil
			}
			if !check {
				if _, err := gh(root, "repo", "edit", repo.NameWithOwner, flag, want); err != nil {
					return nil, err
				}
			}
			return []string{fmt.Sprintf("%q, want %q", have, want)}, nil
		}
	}
	site := repo
	site.Description = config.Description // what the repo's description becomes
	type step struct {
		name string
		run  func() ([]string, error)
	}
	steps := []step{
		{"docs site (" + config.Docs + "/)", func() ([]string, error) {
			files, _, err := siteFiles(root, config.Docs, site)
			if err != nil {
				return nil, err
			}
			return syncFiles(root, files, check)
		}},
		{".github issue forms and labels.tsv", func() ([]string, error) {
			files, err := githubFiles(root, false)
			if err != nil {
				return nil, err
			}
			return syncFiles(filepath.Join(root, ".github"), files, check)
		}},
	}
	if config.Renovate {
		steps = append(steps, step{"renovate.json", func() ([]string, error) {
			return syncFiles(root, map[string][]byte{"renovate.json": renovateConfig(toolVersion())}, check)
		}})
	}
	if len(config.Projects) > 0 || isProject(root) {
		steps = append(steps, step{".github/workflows", func() ([]string, error) {
			workflows, err := workflowsFor(root)
			if err != nil {
				return nil, err
			}
			return syncFiles(filepath.Join(root, ".github", "workflows"), workflows, check)
		}})
	}
	steps = append(steps,
		step{"labels", func() ([]string, error) { return syncLabels(root, repo.NameWithOwner, check) }},
		step{"description", edit("--description", repo.Description, config.Description)},
		step{"homepage", edit("--homepage", repo.HomepageURL, config.Homepage)},
		step{"topics", func() ([]string, error) { return syncTopics(root, repo, config.Topics, check) }},
		step{"GitHub Pages", func() ([]string, error) { return syncPages(root, repo, config.Docs, check) }},
	)
	for _, form := range ownForms(root) {
		fmt.Printf("kept     .github/%s: the repo's own (it does not start with %q)\n", form, formHeader)
	}
	drift := 0
	for _, step := range steps {
		changes, err := step.run()
		switch {
		case err != nil:
			return fmt.Errorf("%s: %w", step.name, err)
		case len(changes) == 0:
			fmt.Printf("ok       %s\n", step.name)
		case check:
			drift++
			fmt.Printf("differs  %s: %s\n", step.name, strings.Join(changes, "; "))
		default:
			fmt.Printf("changed  %s: %s\n", step.name, strings.Join(changes, "; "))
		}
	}
	if drift > 0 {
		return fmt.Errorf("%d items differ from charter.toml and the tool's templates: mise run repo", drift)
	}
	return nil
}

// syncFiles writes the files (by their path below dir) that differ, or with check only names them.
func syncFiles(dir string, files map[string][]byte, check bool) ([]string, error) {
	var stale []string
	for name, content := range files {
		path := filepath.Join(dir, name)
		if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, content) {
			continue
		}
		stale = append(stale, filepath.ToSlash(name))
		if !check {
			if err := write(path, string(content)); err != nil {
				return nil, err
			}
		}
	}
	slices.Sort(stale)
	return stale, nil
}

// labelRows are the labels in labels.tsv: name, color, description.
func labelRows() ([][3]string, error) {
	rows, err := collaborationFiles.ReadFile("github/labels.tsv")
	if err != nil {
		return nil, err
	}
	var labels [][3]string
	for _, row := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		if field := strings.Split(row, "\t"); len(field) == 3 {
			labels = append(labels, [3]string{field[0], field[1], field[2]})
		}
	}
	return labels, nil
}

// The labels GitHub gives a new repo.
var githubDefaultLabels = []string{"bug", "documentation", "duplicate", "enhancement", "good first issue", "help wanted", "invalid", "question", "wontfix"}

// syncLabels makes the repo's labels those of labels.tsv, and removes GitHub's default labels that
// are not in it and that no issue or pull request has. Other labels are the repo's own: kept.
func syncLabels(dir, repo string, check bool) ([]string, error) {
	want, err := labelRows()
	if err != nil {
		return nil, err
	}
	out, err := gh(dir, "label", "list", "-R", repo, "--limit", "500", "--json", "name,color,description")
	if err != nil {
		return nil, err
	}
	var have []struct{ Name, Color, Description string }
	if err := json.Unmarshal([]byte(out), &have); err != nil {
		return nil, err
	}
	current := map[string][2]string{}
	for _, label := range have {
		current[label.Name] = [2]string{strings.ToLower(label.Color), label.Description}
	}
	var changes []string
	listed := map[string]bool{}
	for _, label := range want {
		listed[label[0]] = true
		if now, ok := current[label[0]]; ok && now == [2]string{label[1], label[2]} {
			continue
		}
		changes = append(changes, "set "+label[0])
		if !check {
			if _, err := gh(dir, "label", "create", label[0], "-R", repo, "--color", label[1], "--description", label[2], "--force"); err != nil {
				return nil, err
			}
		}
	}
	for _, name := range githubDefaultLabels {
		if _, ok := current[name]; !ok || listed[name] {
			continue
		}
		used := false
		for _, kind := range []string{"issue", "pr"} {
			out, err := gh(dir, kind, "list", "-R", repo, "--label", name, "--state", "all", "--limit", "1", "--json", "number")
			if err != nil {
				return nil, err
			}
			used = used || (out != "[]" && out != "")
		}
		if used {
			continue
		}
		changes = append(changes, "remove "+name)
		if !check {
			if _, err := gh(dir, "label", "delete", name, "-R", repo, "--yes"); err != nil {
				return nil, err
			}
		}
	}
	return changes, nil
}

// syncTopics makes the repo's topics exactly those of charter.toml.
func syncTopics(dir string, repo githubRepo, want []string, check bool) ([]string, error) {
	var have []string
	for _, topic := range repo.RepositoryTopics {
		have = append(have, topic.Name)
	}
	var add, remove []string
	for _, topic := range want {
		if !slices.Contains(have, topic) {
			add = append(add, topic)
		}
	}
	for _, topic := range have {
		if !slices.Contains(want, topic) {
			remove = append(remove, topic)
		}
	}
	var changes []string
	args := []string{"repo", "edit", repo.NameWithOwner}
	if len(add) > 0 {
		changes = append(changes, "add "+strings.Join(add, ", "))
		args = append(args, "--add-topic", strings.Join(add, ","))
	}
	if len(remove) > 0 {
		changes = append(changes, "remove "+strings.Join(remove, ", "))
		args = append(args, "--remove-topic", strings.Join(remove, ","))
	}
	if len(changes) > 0 && !check {
		if _, err := gh(dir, args...); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

// syncPages turns GitHub Pages on for the docs folder on the default branch, built by GitHub itself
// (no workflow: GitHub renders the markdown).
func syncPages(dir string, repo githubRepo, docs string, check bool) ([]string, error) {
	path := "/" + strings.TrimPrefix(docs, ".")
	branch := repo.DefaultBranchRef.Name
	endpoint := "repos/" + repo.NameWithOwner + "/pages"
	source := []string{"-f", "source[branch]=" + branch, "-f", "source[path]=" + path}
	out, err := gh(dir, "api", endpoint)
	if err != nil {
		if !strings.Contains(err.Error(), "404") {
			return nil, err
		}
		if !check {
			if _, err := gh(dir, append([]string{"api", "-X", "POST", endpoint}, source...)...); err != nil {
				return nil, err
			}
		}
		return []string{"off, turn on for " + docs + "/ on " + branch}, nil
	}
	var pages struct {
		BuildType string `json:"build_type"`
		Source    struct{ Branch, Path string }
	}
	if err := json.Unmarshal([]byte(out), &pages); err != nil {
		return nil, err
	}
	if pages.BuildType == "legacy" && pages.Source.Branch == branch && pages.Source.Path == path {
		return nil, nil
	}
	if !check {
		if _, err := gh(dir, append([]string{"api", "-X", "PUT", endpoint, "-f", "build_type=legacy"}, source...)...); err != nil {
			return nil, err
		}
	}
	return []string{fmt.Sprintf("%s from %s %s, want built by GitHub from %s %s", pages.BuildType, pages.Source.Branch, pages.Source.Path, branch, path)}, nil
}
