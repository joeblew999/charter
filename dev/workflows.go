package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The GitHub workflows are templates in dev/workflows, compiled into this program, so another repo
// gets them without a copy of this one:
//
//	go run github.com/joeblew999/orpc-api/dev@latest workflows -into .
//
// A file's prefix says what it is for: api-*, sdk-*, or dev-* (this tool itself). Every step that
// does work is `mise run <task>`, so the same line runs locally (a test holds the templates to that).

//go:embed workflows/*.yml
var workflowFiles embed.FS

func init() {
	commands["workflows"] = command{"[-check] [-into <repo dir>] [-only api,sdk]",
		"write the GitHub workflows (api-*, sdk-*, dev-*) into <dir>/.github/workflows; -check fails if they differ", workflows}
	anywhere["workflows"] = true
}

// workflowTemplates are the embedded templates whose prefix is in only (all of them when it is empty).
func workflowTemplates(only []string) (map[string][]byte, error) {
	names, err := fs.Glob(workflowFiles, "workflows/*.yml")
	if err != nil {
		return nil, err
	}
	templates := map[string][]byte{}
	for _, name := range names {
		prefix, _, _ := strings.Cut(filepath.Base(name), "-")
		keep := len(only) == 0
		for _, want := range only {
			keep = keep || want == prefix
		}
		if !keep {
			continue
		}
		if templates[filepath.Base(name)], err = workflowFiles.ReadFile(name); err != nil {
			return nil, err
		}
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("no workflows with the prefix %s: there are api, sdk and dev", strings.Join(only, ","))
	}
	return templates, nil
}

func workflows(args []string) error {
	var check bool
	var into, only string
	flags("workflows", args, func(f *flag.FlagSet) {
		f.BoolVar(&check, "check", false, "write nothing: fail if a workflow in the repo differs from its template")
		f.StringVar(&into, "into", "", "the repo to write into (default: this one)")
		f.StringVar(&only, "only", "", "prefixes to write, comma-separated (default: api,sdk, and dev where the repo has dev/)")
	})
	switch {
	case into == "":
		into = "." // main has moved to the repo's root, when there is one
	case !filepath.IsAbs(into):
		into = filepath.Join(started, into)
	}
	if !exists(into) {
		return fmt.Errorf("%s does not exist", into)
	}
	prefixes := []string{"api", "sdk"}
	if only != "" {
		prefixes = strings.Split(only, ",")
	} else if exists(filepath.Join(into, "dev", "go.mod")) {
		prefixes = nil // this repo, or a fork of it: all of them
	}
	templates, err := workflowTemplates(prefixes)
	if err != nil {
		return err
	}
	dir := filepath.Join(into, ".github", "workflows")
	if rel, err := filepath.Rel(started, dir); err == nil && filepath.IsLocal(rel) {
		dir = rel // shorter to read; main may have moved to the repo's root, so only when that is where we started
	}
	var stale []string
	for name, template := range templates {
		if have, err := os.ReadFile(filepath.Join(dir, name)); err != nil || !bytes.Equal(have, template) {
			stale = append(stale, name)
		}
	}
	slices.Sort(stale)
	if check {
		if len(stale) > 0 {
			return fmt.Errorf("%s: %s differ from the templates in dev/workflows. Edit the template, then: mise run dev:workflows",
				dir, strings.Join(stale, ", "))
		}
		fmt.Printf("%s: %d workflows match their templates\n", dir, len(templates))
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range stale {
		if err := os.WriteFile(filepath.Join(dir, name), templates[name], 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", filepath.Join(dir, name))
	}
	fmt.Printf("%s: %d workflows, %d written\n", dir, len(templates), len(stale))
	if len(stale) > 0 && !exists(filepath.Join(into, "mise.toml")) {
		fmt.Println("note: the workflows only call mise tasks, and there is no mise.toml here: copy the tasks they name from orpc-api's mise.toml")
	}
	return nil
}
