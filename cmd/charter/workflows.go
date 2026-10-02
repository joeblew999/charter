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

// The GitHub workflows are templates in cmd/charter/workflows, compiled into this program, so a
// project gets them from the tool it already runs: check, deploy, sdk-check and release. Every step
// that does work is `mise run <task>`, so the same line runs locally (a test holds the templates to
// that).

//go:embed workflows/*.yml
var workflowFiles embed.FS

func init() {
	commands["workflows"] = command{"[-check] [-into <repo dir>]",
		"write the GitHub workflows (check, deploy, sdk-check, release) into <dir>/.github/workflows; -check fails if they differ", workflows}
	anywhere["workflows"] = true
}

// workflowTemplates are the embedded templates, by file name.
func workflowTemplates() (map[string][]byte, error) {
	names, err := fs.Glob(workflowFiles, "workflows/*.yml")
	if err != nil {
		return nil, err
	}
	templates := map[string][]byte{}
	for _, name := range names {
		if templates[filepath.Base(name)], err = workflowFiles.ReadFile(name); err != nil {
			return nil, err
		}
	}
	return templates, nil
}

// conditional keeps the lines a repo needs. A template says which with comment lines of its own:
//
//	# if-dir examples   the lines up to "# else" or "# end" are for a repo that has the folder examples/
//	# else              the lines up to "# end" are for a repo that doesn't
//	# end
//
// So one set of templates serves a repo that is one project, with its tasks at the root (what
// `charter new` makes), and a repo that holds several in examples/ (charter's own, which also has
// the tool and the Go library). The marker lines themselves are never written.
func conditional(template []byte, repo string) []byte {
	var out []string
	keep, inside := true, false
	for _, line := range strings.Split(string(template), "\n") {
		switch marker := strings.TrimSpace(line); {
		case strings.HasPrefix(marker, "# if-dir "):
			inside, keep = true, exists(filepath.Join(repo, strings.TrimPrefix(marker, "# if-dir ")))
		case inside && marker == "# else":
			keep = !keep
		case inside && marker == "# end":
			inside, keep = false, true
		case keep:
			out = append(out, line)
		}
	}
	return []byte(strings.Join(out, "\n"))
}

// workflowsFor are the workflows as the repo in dir needs them, by file name.
func workflowsFor(dir string) (map[string][]byte, error) {
	templates, err := workflowTemplates()
	if err != nil {
		return nil, err
	}
	for name, template := range templates {
		templates[name] = conditional(template, dir)
	}
	return templates, nil
}

// writeWorkflows writes every workflow into dir/.github/workflows.
func writeWorkflows(dir string) error {
	templates, err := workflowsFor(dir)
	if err != nil {
		return err
	}
	for name, content := range templates {
		if err := write(filepath.Join(dir, ".github", "workflows", name), string(content)); err != nil {
			return err
		}
	}
	return nil
}

func workflows(args []string) error {
	var check bool
	var into string
	flags("workflows", args, func(f *flag.FlagSet) {
		f.BoolVar(&check, "check", false, "write nothing: fail if a workflow in the repo differs from its template")
		f.StringVar(&into, "into", "", "the repo to write into (default: this folder)")
	})
	switch {
	case into == "":
		into = "." // the project, or the folder the command was started in
	case !filepath.IsAbs(into):
		into = filepath.Join(started, into)
	}
	if !exists(into) {
		return fmt.Errorf("%s does not exist", into)
	}
	templates, err := workflowsFor(into)
	if err != nil {
		return err
	}
	dir := filepath.Join(into, ".github", "workflows")
	if check && !exists(dir) {
		fmt.Println("no .github/workflows here: mise run workflows writes them (GitHub reads them at a repo's root)")
		return nil
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
			return fmt.Errorf("%s: %s differ from the tool's templates (cmd/charter/workflows in the charter repo): mise run workflows writes them again",
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
		fmt.Println("note: the workflows only call mise tasks, and there is no mise.toml here: they are for a project (charter new makes one)")
	}
	return nil
}
