package repo

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/project"
)

// charterTopic is the topic every repo charter keeps has, so they can be found.
const charterTopic = "charter"

// githubDefaultLabels are the labels GitHub gives a new repo: those no issue uses and charter does not list go.
var githubDefaultLabels = []string{"bug", "documentation", "duplicate", "enhancement", "good first issue", "help wanted", "invalid", "question", "wontfix"}

type githubRepo struct {
	NameWithOwner       string                  `json:"nameWithOwner"`
	Name                string                  `json:"name"`
	Description         string                  `json:"description"`
	HomepageURL         string                  `json:"homepageUrl"`
	Visibility          string                  `json:"visibility"`
	RepositoryTopics    []struct{ Name string } `json:"repositoryTopics"`
	DefaultBranchRef    struct{ Name string }   `json:"defaultBranchRef"`
	DeleteBranchOnMerge bool                    `json:"deleteBranchOnMerge"`
}

// Topics are the repo's topics on GitHub: [github] topics, charter's own among them.
func Topics(g project.GitHub) ([]string, error) {
	topics := slices.Clone(g.Topics)
	for _, t := range topics {
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`).MatchString(t) {
			return nil, fmt.Errorf("charter.toml [github] topics: %q: a GitHub topic is lower-case letters, digits and hyphens, up to 50", t)
		}
	}
	if !slices.Contains(topics, charterTopic) {
		topics = append(topics, charterTopic)
	}
	if len(topics) > 20 {
		return nil, errors.New("charter.toml [github] topics: GitHub takes up to 20, charter's own among them")
	}
	return topics, nil
}

// Repo runs charter repo [-check]: the repo on GitHub in shape from charter.toml's [github]: the labels (charter's
// .github/charter-labels.tsv), its description, homepage and topics, GitHub Pages serving its docs (a public repo's: a
// private repo's docs are never published, #118), and a pull request's branch deleted once it is merged. It writes no
// file: charter init writes charter's files. -check changes nothing and fails on drift.
func Repo(args []string) {
	fs := flag.NewFlagSet("charter repo", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "change nothing; fail on drift")
	fs.Parse(args)
	r := root()
	s, err := settings(r)
	check(err)
	g := s.GitHub
	if g.Description == "" {
		check(errors.New("charter.toml has no [github] description: GitHub shows it as the repo's"))
	}
	topics, err := Topics(g)
	check(err)
	run := func(a ...string) string {
		out, err := gh(r, a...)
		check(err)
		return out
	}
	var repo githubRepo
	check(json.Unmarshal([]byte(run("repo", "view", "--json", "nameWithOwner,name,description,homepageUrl,visibility,repositoryTopics,defaultBranchRef,deleteBranchOnMerge")), &repo))
	public := strings.EqualFold(repo.Visibility, "public")
	homepage := g.Homepage
	if homepage == "" && public {
		owner, _, _ := strings.Cut(repo.NameWithOwner, "/")
		homepage = "https://" + strings.ToLower(owner) + ".github.io/" + repo.Name + "/"
	}
	edit := func(flag, have, want string) func() []string {
		return func() []string {
			if have == want {
				return nil
			}
			if !*checkOnly {
				run("repo", "edit", repo.NameWithOwner, flag, want)
			}
			return []string{fmt.Sprintf("%q, want %q", have, want)}
		}
	}
	items := []struct {
		name string
		sync func() []string
	}{
		{"labels", func() []string { return labels(r, repo.NameWithOwner, *checkOnly) }},
		{"description", edit("--description", repo.Description, g.Description)},
		{"homepage", edit("--homepage", repo.HomepageURL, homepage)},
		{"topics", func() []string { return syncTopics(r, repo, topics, *checkOnly) }},
		{"GitHub Pages", func() []string { return pages(r, repo, g.DocsFolder(), public, *checkOnly) }},
		// A merged pull request's branch goes with it: squash merges leave branches no merge reaches.
		{"delete branches once merged", func() []string {
			if repo.DeleteBranchOnMerge {
				return nil
			}
			if !*checkOnly {
				run("repo", "edit", repo.NameWithOwner, "--delete-branch-on-merge")
			}
			return []string{"off, want on"}
		}},
	}
	drift := 0
	for _, item := range items {
		changes := item.sync()
		switch {
		case len(changes) == 0:
			fmt.Printf("ok       %s\n", item.name)
		case *checkOnly:
			drift++
			fmt.Printf("differs  %s: %s\n", item.name, strings.Join(changes, "; "))
		default:
			fmt.Printf("changed  %s: %s\n", item.name, strings.Join(changes, "; "))
		}
	}
	if drift > 0 {
		check(fmt.Errorf("%d items differ from charter.toml's [github] and charter's labels: charter repo", drift))
	}
}

// labels makes the repo's labels charter's, and removes GitHub's default ones that charter does not list and nothing
// uses. Other labels are the repo's own: kept.
func labels(root, repo string, checkOnly bool) []string {
	b, err := files.ReadFile("files/.github/charter-labels.tsv")
	check(err)
	var want [][]string
	for i, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") || i == 0 {
			continue
		}
		if f := strings.Split(line, "\t"); len(f) == 3 && f[0] != "name" {
			want = append(want, f)
		}
	}
	out, err := gh(root, "label", "list", "-R", repo, "--limit", "500", "--json", "name,color,description")
	check(err)
	var list []struct{ Name, Color, Description string }
	check(json.Unmarshal([]byte(out), &list))
	have := map[string]string{}
	for _, l := range list {
		have[l.Name] = strings.ToLower(l.Color) + "\t" + l.Description
	}
	var changes []string
	listed := map[string]bool{}
	for _, f := range want {
		listed[f[0]] = true
		if have[f[0]] == f[1]+"\t"+f[2] {
			continue
		}
		changes = append(changes, "set "+f[0])
		if !checkOnly {
			_, err := gh(root, "label", "create", f[0], "-R", repo, "--color", f[1], "--description", f[2], "--force")
			check(err)
		}
	}
	for _, name := range githubDefaultLabels {
		if _, ok := have[name]; !ok || listed[name] {
			continue
		}
		used := false
		for _, kind := range []string{"issue", "pr"} {
			out, err := gh(root, kind, "list", "-R", repo, "--label", name, "--state", "all", "--limit", "1", "--json", "number")
			check(err)
			if s := strings.TrimSpace(out); s != "" && s != "[]" {
				used = true
			}
		}
		if used {
			continue
		}
		changes = append(changes, "remove "+name)
		if !checkOnly {
			_, err := gh(root, "label", "delete", name, "-R", repo, "--yes")
			check(err)
		}
	}
	return changes
}

// syncTopics makes the repo's topics exactly [github] topics, charter's own among them.
func syncTopics(root string, repo githubRepo, want []string, checkOnly bool) []string {
	var have []string
	for _, t := range repo.RepositoryTopics {
		have = append(have, t.Name)
	}
	var add, remove []string
	for _, t := range want {
		if !slices.Contains(have, t) {
			add = append(add, t)
		}
	}
	for _, t := range have {
		if !slices.Contains(want, t) {
			remove = append(remove, t)
		}
	}
	var changes []string
	if len(add) > 0 {
		changes = append(changes, "add "+strings.Join(add, ", "))
	}
	if len(remove) > 0 {
		changes = append(changes, "remove "+strings.Join(remove, ", "))
	}
	if len(changes) > 0 && !checkOnly {
		a := []string{"repo", "edit", repo.NameWithOwner}
		if len(add) > 0 {
			a = append(a, "--add-topic", strings.Join(add, ","))
		}
		if len(remove) > 0 {
			a = append(a, "--remove-topic", strings.Join(remove, ","))
		}
		_, err := gh(root, a...)
		check(err)
	}
	return changes
}

// pages turns GitHub Pages on for the docs folder on the default branch, built by GitHub itself (no workflow: GitHub
// renders the Markdown), for a public repo. A private repo's docs are never published (on a personal account a private
// repo's site is public): its Pages stays as it is.
func pages(root string, repo githubRepo, docs string, public, checkOnly bool) []string {
	if !public {
		return nil
	}
	path := "/" + docs
	if docs == "." {
		path = "/"
	}
	branch := repo.DefaultBranchRef.Name
	endpoint := "repos/" + repo.NameWithOwner + "/pages"
	source := []string{"-f", "source[branch]=" + branch, "-f", "source[path]=" + path}
	out, err := gh(root, "api", endpoint)
	if err != nil {
		if !strings.Contains(err.Error(), "404") {
			check(err)
		}
		if !checkOnly {
			_, err := gh(root, append([]string{"api", "-X", "POST", endpoint}, source...)...)
			check(err)
		}
		return []string{"off, turn on for " + docs + "/ on " + branch}
	}
	var now struct {
		BuildType string `json:"build_type"`
		Source    struct{ Branch, Path string }
	}
	check(json.Unmarshal([]byte(out), &now))
	if now.BuildType == "legacy" && now.Source.Branch == branch && now.Source.Path == path {
		return nil
	}
	if !checkOnly {
		_, err := gh(root, append([]string{"api", "-X", "PUT", endpoint, "-f", "build_type=legacy"}, source...)...)
		check(err)
	}
	return []string{fmt.Sprintf("%s from %s %s, want built by GitHub from %s %s", now.BuildType, now.Source.Branch, now.Source.Path, branch, path)}
}
