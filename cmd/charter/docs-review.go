package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// charter docs-review: have Claude bring docs/ up to date and into line with docs/charter/writing.md (-print: only show the prompt)
// Run by: mise run docs:review (tasks/repo/tasks.toml)
func init() {
	commands["docs-review"] = command{"[-print]",
		"have Claude bring docs/ up to date and into line with docs/charter/writing.md (-print: only show the prompt)", docsReviewRun}
	anywhere["docs-review"] = true
}

// The prompt docs-review hands Claude: the same for every repo.
//
//go:embed review.md
var docsReview string

// docsReviewRun hands Claude (the claude command, Claude Code) the review prompt with what
// docs-lint found, in this repo, allowed to edit files. The prompt is the same for every repo; the
// standard it applies is docs/charter/writing.md.
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

var errNoClaude = errors.New("docs-review needs the claude command (Claude Code) on PATH")
