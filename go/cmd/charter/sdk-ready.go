package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// charter sdk:ready: generate and build what the tests use, if missing: TypeScript and Go SDKs, the Fern CLI if the project has one (or only the groups named)
// Run by: mise run sdk:ready (tasks/repo/tasks.toml)
func init() {
	commands["sdk:ready"] = command{"[group...]", "generate and build what the tests use, if missing: TypeScript and Go SDKs, the Fern CLI if the project has one (or only the groups named)", sdkReady}
}

// sdkFresh reports whether sdk/out/<group> was generated from the Fern folder as it is now.
func sdkFresh(group string) bool {
	made, err := os.ReadFile(sdkStamp(group))
	source, err2 := sdkSourceHash()
	return err == nil && err2 == nil && strings.TrimSpace(string(made)) == source
}

func sdkReady(groups []string) error {
	if slices.Contains(groups, "cli") && !hasCLI() {
		return errNoCLI
	}
	// By default, the CLI too when the project has one.
	wanted := func(group string) bool {
		return (len(groups) == 0 && (group != "cli" || hasCLI())) || slices.Contains(groups, group)
	}
	needs := []struct{ file, group string }{
		{"typescript-dist/esm/index.mjs", "typescript-dist"},
		{"go/go.mod", "go"},
		{"cli/Cargo.toml", "cli"},
	}
	// Any other group named (a project's own, like a showcase's typescript-public) is there when its folder is.
	for _, group := range groups {
		if !slices.ContainsFunc(needs, func(need struct{ file, group string }) bool { return need.group == group }) {
			needs = append(needs, struct{ file, group string }{group, group})
		}
	}
	for _, need := range needs {
		// Missing, or generated from other specs or settings: generate it (again).
		if wanted(need.group) && (!exists(filepath.Join("sdk/out", need.file)) || !sdkFresh(need.group)) {
			if err := sdkGen([]string{need.group}); err != nil {
				return err
			}
		}
	}
	if !wanted("cli") {
		return nil
	}
	cli := sdkOut("cli")
	bin, err := cargoBin(cli)
	if err != nil {
		return err
	}
	// The binary is older than its sources when they were generated again.
	if built, err := os.Stat(filepath.Join(cli, "target/release", exe(bin))); err != nil || olderThan(built, sdkStamp("cli")) {
		return cliBuildDir(cli, false)
	}
	return nil
}

// olderThan reports whether info was modified before file (false if file is missing).
func olderThan(info os.FileInfo, file string) bool {
	other, err := os.Stat(file)
	return err == nil && info.ModTime().Before(other.ModTime())
}
