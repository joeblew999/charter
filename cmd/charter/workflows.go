package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The GitHub workflows are templates in cmd/charter/workflows, compiled into this program, so a
// project gets them from the tool it already runs: check, deploy, sdk-check and release. Every step
// that does work is `mise run <task>`, so the same line runs locally (a test holds the templates to
// that).

//go:embed workflows/*.yml
var workflowFiles embed.FS

// Beside the workflows go the issue forms (a bug, a feature, a bug in a project this one is built
// on, a plan) and blank issues turned off: the kit's .github/ISSUE_TEMPLATE (formFiles), the same in
// every repo. The labels the forms use are the tool's own (labels.tsv), which charter repo and charter
// labels give the repo on GitHub.
//
//go:embed labels.tsv
var labelsFile []byte

// repoWorkflow is the workflow of a repo that is not a project: the checks any repo takes from
// charter, as a call to charter's own reusable workflow (.github/workflows/repo-check.yml in the
// charter repo) at the release the repo pins. A project's workflows run its own check, which holds
// its docs to the same, so a repo gets this one or those, never both.
const repoWorkflow = "charter.yml"

// repoWorkflowFor is that workflow for the repo at root, whose default branch is branch, calling
// charter's at release (a checkout: main). The mise it installs is the one the repo's mise.toml asks
// for (min_version): one place says which mise, and a runner's own may be older (seen: a repo that
// needs 2026.10.4 on a runner with 2026.10.3).
func repoWorkflowFor(root, branch, release string) []byte {
	if branch == "" {
		branch = "main"
	}
	if release == "" {
		release = "main"
	}
	with := ""
	if config, err := os.ReadFile(filepath.Join(root, "mise.toml")); err == nil {
		if m := regexp.MustCompile(`(?m)^min_version\s*=\s*"([0-9][0-9A-Za-z.-]*)"`).FindStringSubmatch(string(config)); m != nil {
			with = "\n    with:\n      mise-version: " + m[1] + " # the repo's mise.toml: min_version"
		}
	}
	return []byte(strings.NewReplacer("__BRANCH__", branch, "__REF__", release, "__WITH__", with).Replace(kitFile(".github/workflows/" + repoWorkflow)))
}

// hasProjects reports whether the repo at root is a project or lists some: it gets the projects'
// workflows.
func hasProjects(root string, config charterToml) bool {
	return len(config.Projects) > 0 || isProject(root)
}

func init() {
	commands["workflows"] = command{"[-check] [-into <repo dir>]",
		"write a project's .github: the workflows (check, deploy, sdk-check, release) and charter's issue forms (charter-*.yml), each where the file there is missing or charter's; -check fails if one differs", workflows}
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
//	# if-cli            the lines up to "# else" or "# end" are for a repo that ships a CLI (shipsCLI)
//	# else              the lines up to "# end" are for a repo that doesn't
//	# end
//
// So one set of templates serves a repo that is one project, with its tasks at the root (what
// `charter new` makes), and a repo that holds several in examples/ (charter's own, which also has
// the tool and the Go library); and a project with a CLI and one without. A block can hold another.
// The marker lines themselves are never written.
func conditional(template []byte, repo string) []byte {
	var out []string
	var open []bool // whether each open block keeps its lines
	keep := func() bool { return !slices.Contains(open, false) }
	for _, line := range strings.Split(string(template), "\n") {
		switch marker := strings.TrimSpace(line); {
		case strings.HasPrefix(marker, "# if-dir "):
			open = append(open, exists(filepath.Join(repo, strings.TrimPrefix(marker, "# if-dir "))))
		case marker == "# if-cli":
			open = append(open, shipsCLI(repo))
		case len(open) > 0 && marker == "# else":
			open[len(open)-1] = !open[len(open)-1]
		case len(open) > 0 && marker == "# end":
			open = open[:len(open)-1]
		case keep():
			out = append(out, line)
		}
	}
	return []byte(strings.Join(out, "\n"))
}

// shipsCLI reports whether the repo in dir releases a CLI: its project (at the root) or one of the
// projects in its examples/ has the cli group in fern/generators.yml.
func shipsCLI(dir string) bool {
	files, _ := filepath.Glob(filepath.Join(dir, "examples", "*", generators))
	for _, file := range append([]string{filepath.Join(dir, generators)}, files...) {
		if groups, _ := sdkGroups(file); slices.Contains(groups, "cli") {
			return true
		}
	}
	return false
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

// githubFiles are the files charter writes into the .github folder of the repo in dir, by their path
// below it: the issue forms, and with workflows a project's workflows as the repo needs them.
func githubFiles(dir string, workflows bool) (map[string][]byte, error) {
	files := formFiles()
	if !workflows {
		return files, nil
	}
	found, err := workflowsFor(dir)
	if err != nil {
		return nil, err
	}
	for name, content := range found {
		files[filepath.Join("workflows", name)] = content
	}
	return files, nil
}

// writeWorkflows writes the workflows and the issue forms into dir/.github, where charter may.
func writeWorkflows(dir string) error {
	files, err := githubFiles(dir, true)
	if err != nil {
		return err
	}
	_, _, err = keepCharters(filepath.Join(dir, ".github"), files, false)
	return err
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
	templates, err := githubFiles(into, true) // by path below .github
	if err != nil {
		return err
	}
	dir := filepath.Join(into, ".github")
	if check && !exists(filepath.Join(dir, "workflows")) {
		fmt.Println("no .github/workflows here: mise run workflows writes them (GitHub reads them at a repo's root)")
		return nil
	}
	stale, kept, err := keepCharters(dir, templates, check)
	if err != nil {
		return err
	}
	moved, err := sweep(dir, "ISSUE_TEMPLATE", func(name string) bool { return strings.HasPrefix(name, "charter-") }, templates, check)
	if err != nil {
		return err
	}
	for _, name := range kept {
		fmt.Printf("kept %s: the repo's own (it does not carry charter's mark)\n", filepath.Join(dir, name))
	}
	if check {
		if len(stale)+len(moved) > 0 {
			return fmt.Errorf("%s: %s differ from the tool's templates (cmd/charter/workflows and cmd/charter/kit/.github in the charter repo): mise run workflows writes them again",
				dir, strings.Join(append(stale, moved...), ", "))
		}
		fmt.Printf("%s: %d workflows and issue forms match their templates\n", dir, len(templates)-len(kept))
		return nil
	}
	for _, name := range stale {
		fmt.Println("wrote", filepath.Join(dir, name))
	}
	for _, change := range moved {
		fmt.Println(change)
	}
	fmt.Printf("%s: %d workflows and issue forms, %d written\n", dir, len(templates), len(stale))
	if len(stale) > 0 && !exists(filepath.Join(into, "mise.toml")) {
		fmt.Println("note: the workflows only call mise tasks, and there is no mise.toml here: they are for a project (charter new makes one)")
	}
	return nil
}
