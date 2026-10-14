package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// charter ci-scope: in the check workflow: which Windows and macOS jobs to run, all of them on a push, on a pull request those for what it changes
// Run by: mise run ci:scope (mise.toml)
func init() {
	commands["ci-scope"] = command{"",
		"in the check workflow: which Windows and macOS jobs to run, all of them on a push, on a pull request those for what it changes", ciScope}
	anywhere["ci-scope"] = true
}

func ciScope(args []string) error {
	flags("ci-scope", args, nil)
	var examples []string // the examples that run on Windows and macOS: each folder of examples/
	entries, err := os.ReadDir("examples")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() && exists(filepath.Join("examples", e.Name(), "mise.toml")) {
			examples = append(examples, "examples/"+e.Name())
		}
	}
	var changed []string
	if os.Getenv("GITHUB_EVENT_NAME") == "pull_request" {
		base := os.Getenv("GITHUB_BASE_REF")
		out, err := output(".", "git", "diff", "--name-only", "origin/"+base+"...HEAD")
		if err != nil {
			return fmt.Errorf("ci-scope: the files the pull request changes (git diff origin/%s...HEAD; the checkout needs fetch-depth: 0): %w", base, err)
		}
		changed = strings.Fields(out)
	}
	scope := scopeOf(changed, examples, os.Getenv("GITHUB_EVENT_NAME") != "pull_request")
	list, _ := json.Marshal(scope.examples)
	lines := fmt.Sprintf("tool=%t\nlibrary=%t\nlibrary-ts=%t\nexamples=%s\n", scope.tool, scope.library, scope.libraryTS, list)
	fmt.Print(lines)
	if file := os.Getenv("GITHUB_OUTPUT"); file != "" {
		f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(lines)
		return err
	}
	return nil
}

type ciJobs struct {
	tool, library, libraryTS bool
	examples                 []string
}

// scopeOf is the jobs for the changed files; all of them when all is set or a file every job
// depends on changed.
func scopeOf(changed, examples []string, all bool) ciJobs {
	for _, file := range changed {
		if strings.HasPrefix(file, ".github/") || strings.HasPrefix(file, "tasks/") || file == "mise.toml" {
			all = true
		}
	}
	if all {
		return ciJobs{true, true, true, examples}
	}
	var jobs ciJobs
	add := func(example string) {
		if !slices.Contains(jobs.examples, example) {
			jobs.examples = append(jobs.examples, example)
		}
	}
	for _, file := range changed {
		switch {
		case strings.HasPrefix(file, "cmd/"):
			jobs.tool = true
		case strings.HasPrefix(file, "go/"):
			jobs.library = true
			for _, example := range examples {
				if exists(filepath.Join(filepath.FromSlash(example), "go.mod")) {
					add(example)
				}
			}
		case strings.HasPrefix(file, "ts/"):
			jobs.libraryTS = true
			for _, example := range examples {
				if !exists(filepath.Join(filepath.FromSlash(example), "go.mod")) {
					add(example)
				}
			}
		}
		for _, example := range examples {
			if strings.HasPrefix(file, example+"/") {
				add(example)
			}
		}
	}
	slices.Sort(jobs.examples)
	if jobs.examples == nil {
		jobs.examples = []string{}
	}
	return jobs
}
