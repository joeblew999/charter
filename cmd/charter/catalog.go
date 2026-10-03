package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// `charter catalog` finds which of an owner's repos use each other, from what is already in them.
// The charter repos are those with the GitHub topic charter (charter repo sets it), charter itself,
// and every repo that pins one of them. For each it lists what it publishes (its latest release and
// its files, specs, Go modules, npm packages, task folders, deployed URL) and its consumers: the
// repos whose mise.toml, go.mod or package.json pins it, at which version. Nothing is declared for
// the catalog: it reads the pins the repos build with. Read-only.

func init() {
	commands["catalog"] = command{"[-owner <login>] [-json] [-page]",
		"REMOTE, read-only: the owner's charter repos, what each publishes, and the repos that pin it (their mise.toml, go.mod, package.json), as Markdown or -json; -page adds a docs page's front matter", catalogCommand}
	anywhere["catalog"] = true
}

type catalog struct {
	Owner string     `json:"owner"`
	Read  string     `json:"read"` // when, RFC 3339
	Repos []producer `json:"repos"`
}

type producer struct {
	Repo        string   `json:"repo"` // owner/name
	Description string   `json:"description,omitempty"`
	Topic       bool     `json:"topic"`             // has the topic charter
	Release     string   `json:"release,omitempty"` // the latest release's tag
	Binaries    []string `json:"binaries,omitempty"`
	Files       []string `json:"files,omitempty"` // the latest release's other files
	Specs       []string `json:"specs,omitempty"`
	GoModules   []string `json:"go_modules,omitempty"`
	Packages    []string `json:"packages,omitempty"` // npm
	Tasks       []string `json:"tasks,omitempty"`    // folders a mise.toml can include
	Deployed    []string `json:"deployed,omitempty"`
	Consumers   []pin    `json:"consumers"`
}

// pin is one place a repo says which version of another it uses.
type pin struct {
	Repo    string `json:"repo"`    // the consumer
	File    string `json:"file"`    // where, in the consumer
	Kind    string `json:"kind"`    // tool (mise [tools]), tasks (mise includes), go, npm
	Name    string `json:"name"`    // what it pins
	Version string `json:"version"` // as written
	Target  string `json:"-"`       // the producer, owner/name
}

func catalogCommand(args []string) error {
	var owner string
	var asJSON, page bool
	flags("catalog", args, func(f *flag.FlagSet) {
		f.StringVar(&owner, "owner", "", "whose repos (default: the GitHub login of the token)")
		f.BoolVar(&asJSON, "json", false, "print JSON instead of Markdown")
		f.BoolVar(&page, "page", false, "Markdown with a docs page's front matter")
	})
	client, err := newGitHubClient()
	if err != nil {
		return err
	}
	if owner == "" {
		var me struct{ Login string }
		if err := client.json("/user", &me); err != nil {
			return err
		}
		owner = me.Login
	}
	found, err := readCatalog(client, owner, time.Now())
	if err != nil {
		return err
	}
	if asJSON {
		out, err := json.MarshalIndent(found, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	fmt.Print(catalogMarkdown(found, page))
	return nil
}

// githubClient reads the GitHub REST API with a token: GITHUB_TOKEN, GH_TOKEN, or gh's.
type githubClient struct {
	base, token string
}

var githubAPI = "https://api.github.com"

func newGitHubClient() (githubClient, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		token, _ = output(".", "gh", "auth", "token")
	}
	if token == "" {
		return githubClient{}, errors.New("no GitHub token: set GITHUB_TOKEN, or gh auth login")
	}
	return githubClient{githubAPI, token}, nil
}

var errNotFound = errors.New("not found")

func (c githubClient) get(path, accept string) ([]byte, error) {
	request, err := http.NewRequest("GET", c.base+path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", accept)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	switch {
	case err != nil:
		return nil, err
	case response.StatusCode == 404 || response.StatusCode == 409: // 409: an empty repo
		return nil, errNotFound
	case response.StatusCode != 200:
		return nil, fmt.Errorf("GitHub %s: %s: %s", path, response.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c githubClient) json(path string, v any) error {
	body, err := c.get(path, "application/vnd.github+json")
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// githubListing is a repo as the API lists it.
type githubListing struct {
	FullName      string   `json:"full_name"`
	Description   string   `json:"description"`
	Topics        []string `json:"topics"`
	DefaultBranch string   `json:"default_branch"`
	Fork          bool     `json:"fork"`
	Archived      bool     `json:"archived"`
}

// scanned is what catalog reads of one repo: the paths in it and the pin files' contents.
type scanned struct {
	listing githubListing
	paths   []string
	files   map[string]string // mise.toml, go.mod, package.json, by path
	err     error
}

// The files that hold pins, and the folders whose files are someone else's.
var (
	pinFile    = regexp.MustCompile(`(^|/)(\.?mise\.toml|go\.mod|package\.json)$`)
	notOurs    = regexp.MustCompile(`(^|/)(node_modules|vendor|testdata|third_party|dist|build)/`)
	specFile   = regexp.MustCompile(`(^|/)fern/(openapi|asyncapi)\.(json|ya?ml)$`)
	taskFolder = regexp.MustCompile(`^(tasks/[^/]+)/[^/]+\.toml$`)
)

const maxPinFiles = 60 // per repo: a large monorepo is read in part, and says so

func readCatalog(client githubClient, owner string, now time.Time) (catalog, error) {
	var listings []githubListing
	for page := 1; ; page++ {
		var batch []githubListing
		if err := client.json(fmt.Sprintf("/users/%s/repos?type=owner&per_page=100&page=%d", url.PathEscape(owner), page), &batch); err != nil {
			return catalog{}, err
		}
		for _, repo := range batch {
			if !repo.Fork && !repo.Archived {
				listings = append(listings, repo)
			}
		}
		if len(batch) < 100 {
			break
		}
	}
	repos := make([]scanned, len(listings))
	var wait sync.WaitGroup
	work := make(chan int)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := range work {
				repos[i] = scanRepo(client, listings[i])
			}
		}()
	}
	for i := range listings {
		work <- i
	}
	close(work)
	wait.Wait()

	var pins []pin
	for _, repo := range repos {
		if repo.err != nil {
			fmt.Fprintf(os.Stderr, "catalog: %s: %v\n", repo.listing.FullName, repo.err)
		}
		for _, file := range sortedKeys(anyMap(repo.files)) {
			pins = append(pins, pinsIn(repo.listing.FullName, file, repo.files[file])...)
		}
	}
	// The charter repos: the topic, charter itself, and the repos that pin one of those.
	charterRepos := map[string]bool{strings.ToLower(strings.TrimPrefix(repoURL, "https://github.com/")): true}
	for _, repo := range repos {
		if slices.Contains(repo.listing.Topics, "charter") {
			charterRepos[strings.ToLower(repo.listing.FullName)] = true
		}
	}
	for _, p := range pins {
		if charterRepos[strings.ToLower(p.Target)] {
			charterRepos[strings.ToLower(p.Repo)] = true
		}
	}
	found := catalog{Owner: owner, Read: now.UTC().Format(time.RFC3339)}
	for _, repo := range repos {
		if !charterRepos[strings.ToLower(repo.listing.FullName)] {
			continue
		}
		p := publishes(repo)
		if err := latestRelease(client, &p); err != nil {
			return catalog{}, err
		}
		for _, q := range pins {
			if strings.EqualFold(q.Target, p.Repo) && !strings.EqualFold(q.Repo, p.Repo) {
				p.Consumers = append(p.Consumers, q)
			}
		}
		if p.Consumers == nil {
			p.Consumers = []pin{}
		}
		found.Repos = append(found.Repos, p)
	}
	slices.SortFunc(found.Repos, func(a, b producer) int { return strings.Compare(a.Repo, b.Repo) })
	return found, nil
}

func scanRepo(client githubClient, listing githubListing) scanned {
	repo := scanned{listing: listing, files: map[string]string{}}
	var tree struct {
		Tree []struct{ Path, Type string }
	}
	escaped := "/repos/" + listing.FullName
	if repo.err = client.json(escaped+"/git/trees/"+url.PathEscape(listing.DefaultBranch)+"?recursive=1", &tree); repo.err != nil {
		if errors.Is(repo.err, errNotFound) {
			repo.err = nil // empty
		}
		return repo
	}
	for _, entry := range tree.Tree {
		if entry.Type != "blob" || notOurs.MatchString(entry.Path) {
			continue
		}
		repo.paths = append(repo.paths, entry.Path)
		if !pinFile.MatchString(entry.Path) || len(repo.files) >= maxPinFiles {
			continue
		}
		body, err := client.get(escaped+"/contents/"+escapePath(entry.Path)+"?ref="+url.QueryEscape(listing.DefaultBranch), "application/vnd.github.raw")
		if err != nil {
			repo.err = err
			return repo
		}
		repo.files[entry.Path] = string(body)
	}
	return repo
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

var (
	// github.com/owner/name in a URL, a module path or a git remote.
	githubRepoRef = regexp.MustCompile(`github\.com[/:]([A-Za-z0-9_.-]+)/([A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*?)(?:\.git)?(?:[/#?"' ]|$)`)
	miseSection   = regexp.MustCompile(`^\[([^\]]+)\]\s*$`)
	miseTool      = regexp.MustCompile(`^"?((?:github|ubi|go):[^"=\s]+)"?\s*=\s*(.+)$`)
	miseInclude   = regexp.MustCompile(`git::https://github\.com/([^/]+)/([^/]+?)(?:\.git)?//([^?"]+)\?ref=([^"]+)`)
	quotedVersion = regexp.MustCompile(`^"([^"]*)"|version\s*=\s*"([^"]*)"`)
	goRequire     = regexp.MustCompile(`^(?:require\s+)?(github\.com/\S+)\s+(v\S+)(\s*//\s*indirect)?`)
	npmGitHub     = regexp.MustCompile(`^(?:github:|git\+https://github\.com/|https://github\.com/)([^/]+)/([^/#.]+)`)
	npmVersion    = regexp.MustCompile(`releases/download/([^/]+)/|#(.+)$`)
	apiURL        = regexp.MustCompile(`(?m)^API_URL\s*=.*?(https://[^'"\s}]+)`)
)

// pinsIn are the pins in one file of a repo: mise.toml's tools and task includes, go.mod's direct
// requirements, package.json's dependencies on GitHub.
func pinsIn(repo, file, content string) []pin {
	var found []pin
	add := func(kind, name, version, target string) {
		found = append(found, pin{Repo: repo, File: file, Kind: kind, Name: name, Version: version, Target: target})
	}
	switch path.Base(file) {
	case "mise.toml", ".mise.toml":
		section := ""
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if m := miseSection.FindStringSubmatch(line); m != nil {
				section = m[1]
				continue
			}
			if section == "tools" {
				if m := miseTool.FindStringSubmatch(line); m != nil {
					tool := m[1]
					target := strings.TrimPrefix(strings.SplitN(tool, ":", 2)[1], "github.com/")
					if parts := strings.Split(target, "/"); len(parts) >= 2 {
						v := quotedVersion.FindStringSubmatch(m[2])
						if v != nil {
							add("tool", tool, v[1]+v[2], parts[0]+"/"+parts[1])
						}
					}
				}
			}
		}
		for _, m := range miseInclude.FindAllStringSubmatch(content, -1) {
			add("tasks", m[3], m[4], m[1]+"/"+m[2])
		}
	case "go.mod":
		block := false
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "require ("):
				block = true
				continue
			case line == ")":
				block = false
				continue
			}
			if !block && !strings.HasPrefix(line, "require ") {
				continue
			}
			if m := goRequire.FindStringSubmatch(line); m != nil && m[3] == "" {
				if parts := strings.Split(m[1], "/"); len(parts) >= 3 {
					add("go", m[1], m[2], parts[1]+"/"+parts[2])
				}
			}
		}
	case "package.json":
		var manifest map[string]any
		if json.Unmarshal([]byte(content), &manifest) != nil {
			return nil
		}
		for _, field := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
			deps := object(manifest[field])
			for _, name := range sortedKeys(deps) {
				spec, _ := deps[name].(string)
				m := npmGitHub.FindStringSubmatch(spec)
				if m == nil {
					continue
				}
				version := spec
				if v := npmVersion.FindStringSubmatch(spec); v != nil {
					version = v[1] + v[2]
				}
				add("npm", name, version, m[1]+"/"+m[2])
			}
		}
	}
	return found
}

// publishes is what a repo offers others, from its paths and pin files.
func publishes(repo scanned) producer {
	p := producer{Repo: repo.listing.FullName, Description: repo.listing.Description, Topic: slices.Contains(repo.listing.Topics, "charter")}
	tasks := map[string]bool{}
	for _, file := range repo.paths {
		if specFile.MatchString(file) {
			p.Specs = append(p.Specs, file)
		}
		if m := taskFolder.FindStringSubmatch(file); m != nil && !tasks[m[1]] {
			tasks[m[1]] = true
			p.Tasks = append(p.Tasks, m[1])
		}
	}
	for _, file := range sortedKeys(anyMap(repo.files)) {
		content := repo.files[file]
		switch path.Base(file) {
		case "go.mod":
			for _, line := range strings.Split(content, "\n") {
				// Only a module named for the repo can be fetched by another.
				module, ok := strings.CutPrefix(strings.TrimSpace(line), "module ")
				if module = strings.TrimSpace(module); ok && (strings.EqualFold(module, "github.com/"+p.Repo) || strings.HasPrefix(strings.ToLower(module), strings.ToLower("github.com/"+p.Repo+"/"))) {
					p.GoModules = append(p.GoModules, module)
				}
			}
		case "package.json":
			var manifest struct {
				Name    string
				Private bool
				Files   []string
			}
			if json.Unmarshal([]byte(content), &manifest) == nil && manifest.Name != "" && (!manifest.Private || manifest.Files != nil) {
				p.Packages = append(p.Packages, manifest.Name)
			}
		case "mise.toml", ".mise.toml":
			for _, m := range apiURL.FindAllStringSubmatch(content, -1) {
				if !slices.Contains(p.Deployed, m[1]) {
					p.Deployed = append(p.Deployed, m[1])
				}
			}
		}
	}
	if len(repo.files) >= maxPinFiles {
		p.Description += fmt.Sprintf(" (read in part: the first %d files with pins)", maxPinFiles)
	}
	return p
}

// A release file built for an operating system is a program (or an archive of one).
var forAnOS = regexp.MustCompile(`(?i)(linux|darwin|macos|windows)`)

func latestRelease(client githubClient, p *producer) error {
	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct{ Name string }
	}
	err := client.json("/repos/"+p.Repo+"/releases/latest", &release)
	if errors.Is(err, errNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	p.Release = release.TagName
	for _, asset := range release.Assets {
		if forAnOS.MatchString(asset.Name) {
			p.Binaries = append(p.Binaries, asset.Name)
		} else {
			p.Files = append(p.Files, asset.Name)
		}
	}
	return nil
}

// pseudoVersion is a Go version of a commit, not of a release: v0.0.0-20261003054342-50e83ee58abf.
var pseudoVersion = regexp.MustCompile(`-(0\.)?\d{14}-[0-9a-f]{12}$`)

// standing says how a pin stands against the producer's latest release.
func standing(p pin, release string) string {
	switch {
	case release == "":
		return ""
	case pseudoVersion.MatchString(p.Version):
		return "a commit, not a release"
	case strings.TrimPrefix(p.Version, "v") == strings.TrimPrefix(release, "v"):
		return "latest"
	case version.MatchString("v" + strings.TrimPrefix(p.Version, "v")):
		return "behind " + release
	}
	return ""
}

func catalogMarkdown(found catalog, page bool) string {
	var b strings.Builder
	if page {
		b.WriteString("---\ntitle: Catalog\nnav_order: 50\n---\n\n")
	}
	fmt.Fprintf(&b, "# The charter repos of %s\n\n", found.Owner)
	fmt.Fprintf(&b, "Written by `charter catalog` from GitHub at %s: each repo with the topic `charter`, charter itself, or a pin of one of them; what it publishes, and the repos that pin it.\n", found.Read)
	cell := func(list []string) string {
		if len(list) == 0 {
			return ""
		}
		return "`" + strings.Join(list, "`, `") + "`"
	}
	for _, p := range found.Repos {
		fmt.Fprintf(&b, "\n## %s\n\n", p.Repo)
		if p.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(p.Description))
		}
		rows := [][2]string{
			{"Release", p.Release}, {"Programs", cell(p.Binaries)}, {"Other release files", cell(p.Files)}, {"Specs", cell(p.Specs)},
			{"Go modules", cell(p.GoModules)}, {"npm packages", cell(p.Packages)}, {"Task folders", cell(p.Tasks)}, {"Deployed", strings.Join(p.Deployed, ", ")},
		}
		if p.Release != "" {
			rows[0][1] = "`" + p.Release + "`"
		}
		table := "| Publishes | |\n|---|---|\n"
		for _, row := range rows {
			if row[1] != "" {
				table += fmt.Sprintf("| %s | %s |\n", row[0], row[1])
			}
		}
		if strings.Count(table, "\n") == 2 {
			table = "It publishes nothing another repo can pin: no release, spec, module, package or task folder.\n"
		}
		b.WriteString(table)
		if len(p.Consumers) == 0 {
			b.WriteString("\nNo repo pins it.\n")
			continue
		}
		b.WriteString("\n| Consumer | File | Pins | Version | |\n|---|---|---|---|---|\n")
		for _, c := range p.Consumers {
			fmt.Fprintf(&b, "| %s | `%s` | %s `%s` | `%s` | %s |\n", c.Repo, c.File, c.Kind, c.Name, c.Version, standing(c, p.Release))
		}
	}
	return b.String()
}
