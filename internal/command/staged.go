package command

import (
	"errors"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
	"strings"
)

// Staged runs charter staged: the files staged for a commit written as charter fmt has them and staged again, then
// charter check, or charter.toml's [staged] run when it names commands. charter staged install makes it the repository's pre-commit hook.
func Staged(args []string) {
	p := load()
	if len(args) == 1 && args[0] == "install" {
		top, err := run.Output(p.Root, nil, "git", "rev-parse", "--git-path", "hooks")
		check(err)
		hook := filepath.Join(strings.TrimSpace(top), "pre-commit") // git's path: absolute, or from the app's folder
		if !filepath.IsAbs(hook) {
			hook = filepath.Join(p.Root, hook)
		}
		check(os.MkdirAll(filepath.Dir(hook), 0o755))
		check(os.WriteFile(hook, []byte("#!/bin/sh\n# Written by charter staged install.\nexec go tool charter staged\n"), 0o755))
		say("✓ %s runs charter staged before every commit", hook)
		return
	}
	out, err := run.Output(p.Root, nil, "git", "diff", "--cached", "--name-only", "--relative", "--diff-filter=ACMR") // --relative: the app may be a folder of its repository
	check(err)
	var files []string
	for _, f := range strings.Fields(out) {
		if strings.HasSuffix(f, ".go") || strings.HasSuffix(f, ".gsx") {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return
	}
	check(format(p, true))
	check(run.Run(p.Root, nil, "git", append([]string{"add", "--"}, files...)...))
	if len(p.Staged.Run) == 0 {
		Check(nil)
		return
	}
	for _, line := range p.Staged.Run {
		words := strings.Fields(line)
		if len(words) == 0 {
			check(errors.New("an empty command in charter.toml's [staged]"))
		}
		if words[0] == "charter" {
			words[0] = self()
		}
		check(run.Run(p.Root, nil, words[0], words[1:]...))
	}
}
