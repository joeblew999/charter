package main

import (
	"path/filepath"
	"slices"
)

// The workflows are in the kit (cmd/charter/kit/.github/workflows), copied as they are: a project's
// four (charter-check, charter-deploy, charter-sdk-check, charter-release, and charter-release-cli
// where it ships a CLI), or in a repo that is not a project charter.yml, a call to charter's own
// reusable workflow. Every step that does work is `mise run <task>`, so the same line runs locally.

const repoWorkflow = "charter.yml"

// hasProjects reports whether the repo at root is a project or lists some.
func hasProjects(root string, config charterToml) bool {
	return len(config.Projects) > 0 || isProject(root)
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
