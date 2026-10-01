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
	"strings"
)

func init() {
	commands["docs"] = command{"[-into <repo dir>] [-check]",
		"write the docs site's config and the writing rules into a repo's docs/ folder (GitHub Pages renders it; any repo, same look)", docs}
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

// docs writes the two files that make a repo's docs/ folder a site on GitHub Pages. They are the
// same for every repo except for its name, description and URLs, which GitHub is asked for. With
// -check nothing is written, and it fails if a file differs.
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
	var repo struct {
		NameWithOwner, Name, Description string
		DefaultBranchRef                 struct{ Name string }
	}
	if err := json.Unmarshal([]byte(out), &repo); err != nil {
		return err
	}
	description, _ := json.Marshal(repo.Description) // a YAML string, safely quoted
	config := strings.NewReplacer("__NAME__", repo.Name, "__DESCRIPTION__", string(description),
		"__REPO__", repo.NameWithOwner, "__BRANCH__", repo.DefaultBranchRef.Name).Replace(docsConfig)
	stale := 0
	for path, content := range map[string]string{"docs/_config.yml": config, "docs/_sass/custom/custom.scss": docsStyle, "docs/writing.md": docsWriting, "docs/llms.txt": docsLLMs} {
		path = filepath.Join(into, path)
		if current, _ := os.ReadFile(path); bytes.Equal(current, []byte(content)) {
			continue
		}
		stale++
		if check {
			fmt.Fprintf(os.Stderr, "%s differs from what `dev docs` writes: mise run docs:setup\n", path)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}
	if check && stale > 0 {
		return errors.New("the docs site's config is stale")
	}
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")
	fmt.Printf("docs site: https://%s.github.io/%s/ (first time: mise run docs:pages)\n", owner, name)
	return nil
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
		"Read,Edit,Write,Glob,Grep,Bash(mise run docs:lint),Bash(mise run dev:check),Bash(mise tasks),Bash(git status:*),Bash(git diff:*),Bash(ls:*)")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr // no stdin: the prompt is the whole input
	fmt.Println("Claude is reviewing docs/ (it prints its report when it is done; watch `git status` for its edits)")
	return cmd.Run()
}
