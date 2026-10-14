package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// charter release-tool: GoReleaser on this tool, in its own repo: on a version tag, archives and checksums go to its GitHub Release (if they are not there yet); no tag: a snapshot into dist/
// Run by: mise run charter:release (mise.toml); mise run release:publish (mise.toml)
func init() {
	commands["release-tool"] = command{"[-tag vX.Y.Z]", "GoReleaser on this tool, in its own repo: on a version tag, archives and checksums go to its GitHub Release (if they are not there yet); no tag: a snapshot into dist/", releaseTool}
	anywhere["release-tool"] = true
}

// releaseTool runs GoReleaser (.goreleaser.yaml) on this tool. On a version tag it publishes the
// archives and checksums to the tag's GitHub Release, unless they are there already (a release cut
// from a machine has them: the workflow on the tag then only builds, as a check); anywhere else it is
// a snapshot: the same build into dist/, nothing published.
func releaseTool(args []string) error {
	tag, _, err := releaseTag("release-tool", args)
	if err != nil {
		return err
	}
	if tag != "" && releaseHasTheTool(tag) {
		fmt.Printf("release %s already has the tool (every archive checksums.txt names): building it to check, publishing nothing\n", tag)
		tag = ""
	}
	if tag == "" {
		return sh(".", "goreleaser", "release", "--snapshot", "--clean")
	}
	// Several tags sit on a release's commit (vX.Y.Z, and the nested modules' go/vX.Y.Z and others):
	// say which one this is. GoReleaser reads the token as GITHUB_TOKEN: the workflows set GH_TOKEN,
	// and on a developer's machine gh holds it.
	env := []string{"GORELEASER_CURRENT_TAG=" + tag}
	switch {
	case os.Getenv("GITHUB_TOKEN") != "":
	case os.Getenv("GH_TOKEN") != "":
		env = append(env, "GITHUB_TOKEN="+os.Getenv("GH_TOKEN"))
	default:
		token, err := output(".", "gh", "auth", "token")
		if err != nil {
			return errors.New("no GitHub token for GoReleaser: gh auth login")
		}
		env = append(env, "GITHUB_TOKEN="+token)
	}
	cmd := exec.Command("goreleaser", "release", "--clean")
	cmd.Env, cmd.Stdout, cmd.Stderr = append(os.Environ(), env...), os.Stdout, os.Stderr
	return cmd.Run()
}

// releaseHasTheTool says whether the tag's Release has every archive its checksums.txt names.
// checksums.txt alone is not enough: an upload that stopped part way leaves it there with archives
// missing, and publishing again is what puts them back.
func releaseHasTheTool(tag string) bool {
	assets := releaseAssets(tag)
	if !slices.ContainsFunc(assets, func(a asset) bool { return a.Name == "checksums.txt" }) {
		return false
	}
	sums, err := exec.Command("gh", "release", "download", tag, "--pattern", "checksums.txt", "--output", "-").Output()
	return err == nil && hasEveryArchive(string(sums), assets)
}

// hasEveryArchive says whether every file a checksums.txt names is among the assets.
func hasEveryArchive(sums string, assets []asset) bool {
	named := 0
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		named++
		if !slices.ContainsFunc(assets, func(a asset) bool { return a.Name == fields[1] }) {
			return false
		}
	}
	return named > 0
}
