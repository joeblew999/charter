package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// charter repo: keep the repo on GitHub in shape from charter.toml at its root: labels, description, homepage, topics, Pages (it writes no file: adopt writes the kit); -check changes nothing and fails on drift
// Run by: mise run repo (tasks/repo/tasks.toml); mise run repo:check (tasks/repo/tasks.toml)
func init() {
	commands["repo"] = command{"[-check]",
		"keep the repo on GitHub in shape from charter.toml at its root: labels, description, homepage, topics, Pages (it writes no file: adopt writes the kit); -check changes nothing and fails on drift", repoCommand}
	anywhere["repo"] = true
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
	type step struct {
		name string
		run  func() ([]string, error)
	}
	steps := []step{}
	steps = append(steps,
		step{"labels", func() ([]string, error) { return syncLabels(root, repo.NameWithOwner, check) }},
		step{"description", edit("--description", repo.Description, config.Description)},
		step{"homepage", edit("--homepage", repo.HomepageURL, config.Homepage)},
		step{"topics", func() ([]string, error) { return syncTopics(root, repo, config.Topics, check) }},
		step{"GitHub Pages", func() ([]string, error) { return syncPages(root, repo, config.Docs, check) }},
	)
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

// labelRows are the labels in the kit's .github/charter-labels.tsv: name, color, description.
func labelRows() ([][3]string, error) {
	rows := kitFile(".github/charter-labels.tsv")
	var labels [][3]string
	// A line that starts with # says where the file comes from; the first of the others names the columns.
	header := true
	for _, row := range strings.Split(strings.TrimSpace(string(rows)), "\n") {
		if strings.HasPrefix(row, "#") {
			continue
		}
		if field := strings.Split(row, "\t"); len(field) == 3 && !header {
			labels = append(labels, [3]string{field[0], field[1], field[2]})
		}
		header = false
	}
	return labels, nil
}

// The labels GitHub gives a new repo.
var githubDefaultLabels = []string{"bug", "documentation", "duplicate", "enhancement", "good first issue", "help wanted", "invalid", "question", "wontfix"}

// syncLabels makes the repo's labels those of charter-labels.tsv, and removes GitHub's default labels that
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
