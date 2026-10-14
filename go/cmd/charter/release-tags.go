package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// charter release:tags: tag each Go module in a subdirectory <path in the repo>/vX.Y.Z at the tag's commit (no tag: a dry run)
// Run by: mise run release:tags (mise.toml); mise run release:tags (tasks/repo/tasks.toml)
func init() {
	commands["release:tags"] = command{"[-tag vX.Y.Z] <module dir>...", "tag each Go module in a subdirectory <path in the repo>/vX.Y.Z at the tag's commit (no tag: a dry run)", releaseTags}
	anywhere["release:tags"] = true
}

// releaseTags gives each Go module in a subdirectory the tag Go looks for: <its path in the
// repo>/vX.Y.Z, on the commit of vX.Y.Z. Through the GitHub API, so it needs no git credentials,
// only gh's token.
func releaseTags(args []string) error {
	tag, dirs, err := releaseTag("release-tags", args)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return errors.New("release:tags needs <module dir>..., e.g. sdk/go")
	}
	// Where this folder is in the repo: a module's tag is its path from the repo's root.
	prefix, err := output(".", "git", "rev-parse", "--show-prefix")
	if err != nil {
		return errors.New("release:tags needs a git repo here")
	}
	var modules []string
	for _, dir := range dirs {
		switch {
		case !exists(dir): // a project's sdk/go before its first mise run sdk:publish
			fmt.Printf("skipped: %s (no such folder)\n", dir)
		case !exists(filepath.Join(dir, "go.mod")):
			return fmt.Errorf("%s has no go.mod", dir)
		default:
			modules = append(modules, prefix+filepath.ToSlash(filepath.Clean(dir)))
		}
	}
	if dirs = modules; len(dirs) == 0 {
		return nil
	}
	if tag == "" {
		fmt.Printf("dry run (not on a version tag): on vX.Y.Z this tags %s/vX.Y.Z\n", strings.Join(dirs, "/vX.Y.Z, "))
		return nil
	}
	commit, err := output(".", "git", "rev-parse", tag+"^{commit}")
	if err != nil {
		return fmt.Errorf("no tag %s in this checkout", tag)
	}
	for _, dir := range dirs {
		name := dir + "/" + tag
		if have, err := exec.Command("gh", "api", "repos/{owner}/{repo}/git/ref/tags/"+name, "--jq", ".object.sha").Output(); err == nil {
			if strings.TrimSpace(string(have)) != commit {
				return fmt.Errorf("the tag %s exists on another commit (%s)", name, strings.TrimSpace(string(have)))
			}
			fmt.Println("tag exists:", name)
			continue
		}
		if err := quiet(".", nil, "gh", "api", "repos/{owner}/{repo}/git/refs", "-f", "ref=refs/tags/"+name, "-f", "sha="+commit); err != nil {
			return err
		}
		fmt.Printf("tagged: %s (%.12s)\n", name, commit)
	}
	return nil
}
