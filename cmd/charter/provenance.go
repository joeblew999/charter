package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Every file charter writes into a repo says so in its first lines, in the form its type allows (a
// comment; in JSON, a description field), and says whose it is from then on:
//
//	Written by `charter <command>` ... don't edit     charter's: the command writes it again
//	Started by `charter <command>` ... yours to edit  the repo's: written once, never again
//
// A test holds every template and everything the commands write to it. `charter files` lists the
// files of a repo that carry the mark, and whether charter's are what it would write now.

func init() {
	commands["files"] = command{"",
		"list every file in this repo that charter wrote, by the mark in its first lines: charter's (and whether it is what this release writes) or yours since charter started it", filesCommand}
	anywhere["files"] = true
}

const (
	// markLines is how far down a file its mark may be: under front matter and a title, in a page.
	markLines   = 12
	charterRepo = "github.com/joeblew999/charter"
)

var charterMark = regexp.MustCompile("(Written|Started) by `charter ([a-z-]+)`")

// markOf reads a file's mark: whether it is charter's to rewrite (or the repo's since charter
// started it), and the command that wrote it. No mark, or one that does not say whose the file is:
// false.
func markOf(content []byte) (charters bool, command string, ok bool) {
	lines := bytes.SplitN(content, []byte("\n"), markLines+1)
	if len(lines) > markLines {
		lines = lines[:markLines]
	}
	head := string(bytes.Join(lines, []byte("\n")))
	m := charterMark.FindStringSubmatch(head)
	switch {
	case m == nil:
		return false, "", false
	case m[1] == "Written" && strings.Contains(head, "don't edit"):
		return true, m[2], true
	case m[1] == "Started" && strings.Contains(head, "yours to edit"):
		return false, m[2], true
	}
	return false, "", false
}

// startedBy is a file that a command (new, adopt) writes once, with the mark that says so: a
// comment at its top, inside the front matter of a page that has some.
func startedBy(command, path, content string) string {
	mark := "Started by `charter " + command + "` (" + charterRepo + "): yours to edit."
	switch {
	case strings.HasSuffix(path, ".md") && strings.HasPrefix(content, "---\n"):
		return "---\n# " + mark + "\n" + strings.TrimPrefix(content, "---\n")
	case strings.HasSuffix(path, ".md"):
		return "<!-- " + mark + " -->\n\n" + content
	}
	return "# " + mark + "\n" + content
}

// writtenFiles are the files charter writes into the repo at root and rewrites when they differ,
// by their path in the repo, as this release writes them; and those it cannot say without more
// (the repo on GitHub, running a command of the repo), with what would.
func writtenFiles(root string) (files map[string][]byte, unknown map[string]string, err error) {
	config := charterToml{Docs: "docs", Renovate: true, Workflow: true}
	if text, readErr := os.ReadFile(filepath.Join(root, "charter.toml")); readErr == nil {
		if config, err = parseCharterToml(string(text)); err != nil {
			return nil, nil, err
		}
	}
	files, unknown = map[string][]byte{}, map[string]string{}
	projects := hasProjects(root, config)
	github, err := githubFiles(root, projects)
	if err != nil {
		return nil, nil, err
	}
	for name, content := range github {
		files[".github/"+filepath.ToSlash(name)] = content
	}
	if config.Renovate {
		files["renovate.json"] = renovateConfig(toolVersion())
	}
	// The docs site: its config has the repo's name and description on GitHub in it.
	var repo githubRepo
	out, ghErr := gh(root, "repo", "view", "--json", "nameWithOwner,name,description,defaultBranchRef")
	onGitHub := ghErr == nil && json.Unmarshal([]byte(out), &repo) == nil && repo.Name != ""
	repo.Description = config.Description
	for path, content := range docsSite(repo, config.Docs) {
		path = filepath.ToSlash(filepath.Clean(path))
		if strings.HasSuffix(path, "_config.yml") && !onGitHub {
			unknown[path] = "not compared: it holds the repo's name on GitHub, which gh could not read here"
			continue
		}
		files[path] = content
	}
	if !projects && config.Workflow {
		if path := ".github/workflows/" + repoWorkflow; onGitHub {
			files[path] = repoWorkflowFor(repo.DefaultBranchRef.Name)
		} else {
			unknown[path] = "not compared: it names the default branch on GitHub, which gh could not read here"
		}
	}
	generated, err := readGenerated(filepath.Join(root, filepath.FromSlash(config.Docs)))
	if err != nil {
		return nil, nil, err
	}
	for _, g := range generated {
		unknown[filepath.ToSlash(filepath.Join(config.Docs, filepath.FromSlash(g.Page)))] = "not compared: `" + g.Run + "` prints it, and mise run docs:check runs that"
	}
	return files, unknown, nil
}

// filesCommand lists the files of the repo that are charter's or that charter started. It reads the
// files git knows of or would add, and changes nothing; `charter repo -check` is the check.
func filesCommand(args []string) error {
	if len(args) > 0 {
		return errors.New("files takes no arguments")
	}
	top, err := output(started, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New("files runs in a git repo")
	}
	root := filepath.FromSlash(top)
	lines, err := filesReport(root)
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(lines, "\n"))
	return nil
}

// filesReport is what charter files prints for the repo at root, a line per file: charter's files
// first (matches, differs, missing, not compared), then those it started, then what carries the
// mark where charter writes nothing, and the issue forms that are the repo's own.
func filesReport(root string) ([]string, error) {
	want, unknown, err := writtenFiles(root)
	if err != nil {
		return nil, err
	}
	listed, err := output(root, "git", "ls-files", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var charters, yours, elsewhere []string
	seen := map[string]bool{}
	row := func(whose, state, path, note string) string {
		return strings.TrimRight(fmt.Sprintf("%-9s  %-8s  %-44s  %s", whose, state, path, note), " ")
	}
	for _, path := range strings.Split(listed, "\n") {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if path == "" || err != nil {
			continue
		}
		owned, command, ok := markOf(content)
		expected, written := want[path]
		why, later := unknown[path]
		seen[path] = true
		by := "charter " + command
		switch {
		case !ok && (written || later):
			charters = append(charters, row("charter's", "differs", path, "it carries no mark (an earlier release wrote it, or you did): mise run repo writes charter's"))
		case !ok:
		case !owned:
			yours = append(yours, row("yours", "started", path, by+" wrote it once: edit it"))
		case written && bytes.Equal(content, expected):
			charters = append(charters, row("charter's", "matches", path, by))
		case written:
			charters = append(charters, row("charter's", "differs", path, by+": this release writes it otherwise (mise run repo)"))
		case later:
			charters = append(charters, row("charter's", "-", path, by+"; "+why))
		default:
			elsewhere = append(elsewhere, row("charter's", "-", path, by+": this release writes no file here (a template, a copy, or one it has stopped writing)"))
		}
	}
	for path := range want {
		if !seen[path] {
			charters = append(charters, row("charter's", "missing", path, "mise run repo writes it"))
		}
	}
	for path := range unknown {
		if !seen[path] {
			charters = append(charters, row("charter's", "missing", path, "mise run repo writes it, with the repo on GitHub"))
		}
	}
	slices.Sort(charters)
	for _, form := range ownForms(root) {
		yours = append(yours, row("yours", "own form", ".github/"+form, "it does not start with charter's mark: charter leaves it alone"))
	}
	lines := append(append(charters, yours...), elsewhere...)
	if len(lines) == 0 {
		return []string{"no file here says charter wrote it: charter adopt starts a repo, mise run repo keeps it"}, nil
	}
	return lines, nil
}
