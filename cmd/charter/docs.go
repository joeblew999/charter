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
	"strconv"
	"strings"
)

func init() {
	commands["docs"] = command{"[-into <repo dir>] [-check]",
		"write the docs site's config, the writing rules and the pages docs/_generated.toml lists into a repo's docs/ folder (GitHub Pages renders it; any repo, same look); -check fails if one is stale", docs}
	commands["docs-review"] = command{"[-print]",
		"have Claude bring docs/ up to date and into line with docs/writing.md (-print: only show the prompt)", docsReviewRun}
	anywhere["docs"], anywhere["docs-review"] = true, true
}

//go:embed docs/_config.yml
var docsConfig string

//go:embed docs/custom.scss
var docsStyle string

//go:embed docs/writing.md
var docsWriting string

//go:embed docs/review.md
var docsReview string

// llms.txt: the docs' index for agents, each page linked as Markdown. A template the site's
// renderer fills in. (An all-in-one llms-full.txt can't be made this way: the renderer hands a
// template the pages as HTML.)
//
//go:embed docs/llms.txt
var docsLLMs string

// docs writes the files that make a repo's docs/ folder a site on GitHub Pages. They are the same
// for every repo except for its name, description and URLs, which GitHub is asked for. Then the
// pages its docs/_generated.toml lists, each from its command's output. With -check
// nothing is written, and it fails if a file differs.
func docs(args []string) error {
	into, check := ".", false
	flags("docs", args, func(f *flag.FlagSet) {
		f.StringVar(&into, "into", into, "the repo to write into (its docs/ folder)")
		f.BoolVar(&check, "check", false, "write nothing; fail if a file differs")
	})
	if check && !exists(filepath.Join(into, "docs", "_config.yml")) {
		fmt.Println("no docs/_config.yml here yet: mise run docs:setup writes it")
		return nil
	}
	out, err := output(into, "gh", "repo", "view", "--json", "nameWithOwner,name,description,defaultBranchRef")
	if err != nil {
		return errors.New("docs needs a GitHub repo here (gh repo view failed)")
	}
	var repo githubRepo
	if err := json.Unmarshal([]byte(out), &repo); err != nil {
		return err
	}
	files := docsSite(repo, "docs")
	generated, err := readGenerated(filepath.Join(into, "docs"))
	if err != nil {
		return err
	}
	if err := addGenerated(files, into, "docs", generated); err != nil {
		return err
	}
	writer := map[string]string{} // a generated page: its command
	for _, g := range generated {
		writer[filepath.Join(into, "docs", filepath.FromSlash(g.Page))] = g.Run
	}
	stale := 0
	for path, content := range files {
		path = filepath.Join(into, path)
		if current, _ := os.ReadFile(path); bytes.Equal(current, content) {
			continue
		}
		stale++
		if check {
			if run, ok := writer[path]; ok {
				fmt.Fprintf(os.Stderr, "%s is stale: `%s` now prints something else (mise run docs:setup)\n", path, run)
			} else {
				fmt.Fprintf(os.Stderr, "%s differs from what `charter docs` writes: mise run docs:setup\n", path)
			}
			continue
		}
		if err := write(path, string(content)); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}
	if check && stale > 0 {
		return errors.New("the docs site's config or a generated page is stale")
	}
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")
	fmt.Printf("docs site: https://%s.github.io/%s/ (first time: mise run docs:pages)\n", owner, name)
	return nil
}

// githubRepo is what gh repo view --json says about a repo.
type githubRepo struct {
	NameWithOwner, Name, Description, HomepageURL string
	DefaultBranchRef                              struct{ Name string }
	RepositoryTopics                              []struct{ Name string }
}

// docsSite are the files that make a repo's docs folder a site, by their path in the repo: the same
// for every repo but for its name, description, URLs and the folder.
func docsSite(repo githubRepo, folder string) map[string][]byte {
	description, _ := json.Marshal(repo.Description) // a YAML string, safely quoted
	config := strings.NewReplacer("__NAME__", repo.Name, "__DESCRIPTION__", string(description),
		"__REPO__", repo.NameWithOwner, "__BRANCH__", repo.DefaultBranchRef.Name,
		"gh_edit_source: docs", "gh_edit_source: "+folder).Replace(docsConfig)
	files := map[string][]byte{}
	for path, content := range map[string]string{"_config.yml": config, "_sass/custom/custom.scss": docsStyle, "writing.md": docsWriting, "llms.txt": docsLLMs} {
		files[filepath.Join(folder, path)] = []byte(content)
	}
	return files
}

// addGenerated adds the generated pages to files (by their path in the repo at root, below the
// docs folder), each written from what its command prints, run at root.
func addGenerated(files map[string][]byte, root, docs string, generated []generatedPage) error {
	for i, g := range generated {
		args := splitCommand(g.Run)
		cmd := exec.Command(args[0], args[1:]...)
		var stderr bytes.Buffer
		cmd.Dir, cmd.Stderr = root, &stderr
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("generated page %s: %s: %v\n%s", g.Page, g.Run, err, strings.TrimSpace(stderr.String()))
		}
		page, err := generatedPageText(root, docs, g, i, string(out))
		if err != nil {
			return fmt.Errorf("generated page %s: %s: %w", g.Page, g.Run, err)
		}
		files[filepath.Join(docs, filepath.FromSlash(g.Page))] = []byte(page)
	}
	return nil
}

// generatedPageText is a generated page as it is committed: front matter, then what the command
// printed, with a line under its title that says where it comes from. The title is the output's
// first "# " heading; the parent is the title of its section's page (reference/x.md: reference.md);
// nav_order puts the generated pages after the written ones, in the listed order.
func generatedPageText(root, docs string, g generatedPage, index int, out string) (string, error) {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	if strings.HasPrefix(out, "---\n") {
		return "", errors.New("it printed front matter: charter writes that, the command prints the page from its # title on")
	}
	heading := regexp.MustCompile(`(?m)^# +(.+?) *$`).FindStringSubmatchIndex(out)
	if heading == nil {
		return "", errors.New("it printed no # title")
	}
	title := out[heading[2]:heading[3]]
	front := "---\ntitle: " + yamlString(title) + "\nnav_order: " + strconv.Itoa(100+index) + "\n"
	if dir := filepath.Dir(filepath.FromSlash(g.Page)); dir != "." {
		section := filepath.Join(root, docs, dir+".md")
		parent := ""
		if raw, err := os.ReadFile(section); err == nil {
			if m := regexp.MustCompile(`(?m)^title: *(.+?) *$`).FindStringSubmatch(strings.SplitN(string(raw), "\n---", 2)[0]); m != nil {
				parent = strings.Trim(m[1], `"'`)
			}
		}
		if parent == "" {
			return "", fmt.Errorf("its parent is the section page %s, which has no title", filepath.ToSlash(filepath.Join(docs, dir+".md")))
		}
		front += "parent: " + yamlString(parent) + "\n"
	}
	note := "\n\nWritten by `" + g.Run + "` (docs/_generated.toml), never by hand: change the code it reads, then `mise run docs:setup`."
	body := out[:heading[1]] + note + out[heading[1]:]
	return front + "---\n\n" + strings.TrimRight(body, "\n") + "\n", nil
}

// yamlString is s as a YAML string: JSON's quoting, which YAML reads.
func yamlString(s string) string {
	quoted, _ := json.Marshal(s)
	return string(quoted)
}

// splitCommand splits a command line into its program and arguments at spaces; "double quotes"
// hold one with spaces. No shell runs it, so it is the same on Windows.
func splitCommand(line string) []string {
	var args []string
	var arg strings.Builder
	quoted, any := false, false
	for _, r := range line {
		switch {
		case r == '"':
			quoted, any = !quoted, true
		case (r == ' ' || r == '\t') && !quoted:
			if any {
				args, any = append(args, arg.String()), false
				arg.Reset()
			}
		default:
			arg.WriteRune(r)
			any = true
		}
	}
	if any {
		args = append(args, arg.String())
	}
	return args
}

// docsReviewRun hands Claude (the claude command, Claude Code) the review prompt with what
// docs-lint found, in this repo, allowed to edit files. The prompt is the same for every repo; the
// standard it applies is docs/writing.md.
func docsReviewRun(args []string) error {
	show := false
	flags("docs-review", args, func(f *flag.FlagSet) { f.BoolVar(&show, "print", false, "print the prompt, don't run Claude") })
	lint := exec.Command(os.Args[0], "docs-lint")
	found, _ := lint.CombinedOutput() // a failing lint is the point: its output goes into the prompt
	prompt := strings.Replace(docsReview, "__LINT__", strings.TrimSpace(string(found)), 1)
	if show {
		fmt.Println(prompt)
		return nil
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return errNoClaude
	}
	// It may edit files and run the checks, nothing else.
	cmd := exec.Command("claude", "-p", prompt, "--permission-mode", "acceptEdits", "--allowedTools",
		"Read,Edit,Write,Glob,Grep,Bash(mise run docs:lint),Bash(mise run charter:check),Bash(mise tasks),Bash(git status:*),Bash(git diff:*),Bash(ls:*)")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr // no stdin: the prompt is the whole input
	fmt.Println("Claude is reviewing docs/ (it prints its report when it is done; watch `git status` for its edits)")
	return cmd.Run()
}
