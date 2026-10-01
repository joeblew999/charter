package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// A release is one version tag (vX.Y.Z) on a commit. The dist-* commands build what ships into
// dist/, `release` attaches dist/ to the tag's GitHub Release, and `release-tags` adds the tags the
// Go modules in subdirectories need. Without a tag, `release` and `release-tags` only say what they
// would do, so the whole path can run on a branch or a pull request.

func init() {
	commands["release-dev"] = command{"[-tag vX.Y.Z]", "GoReleaser on this tool: on a version tag, archives and checksums go to its GitHub Release; no tag: a snapshot into dist/", releaseDev}
	commands["dist-sdk"] = command{"<api>...", "generate, check and archive the Go and TypeScript SDKs and the specs of each API into dist/ (Docker)", distSDK}
	commands["dist-cli"] = command{"[-linux] <api>...", "HEAVY: generate and build the Fern CLI of each API into dist/, for this machine or for Linux in Docker", distCLI}
	commands["release"] = command{"[-tag vX.Y.Z]", "attach dist/* to the tag's GitHub Release, creating it if needed (no tag: a dry run)", release}
	commands["release-tags"] = command{"[-tag vX.Y.Z] <module dir>...", "tag each Go module in a subdirectory <dir>/vX.Y.Z at the tag's commit (no tag: a dry run)", releaseTags}
	commands["need-env"] = command{"<NAME>...", "fail, naming them, unless these environment variables are set (the secrets a workflow needs)", needEnv}
}

const dist = "dist"

// distDir makes dist/, which ignores itself: no repo that uses this tool has to list it in .gitignore.
func distDir() error {
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dist, ".gitignore"), []byte("*\n"), 0o644)
}

// Go module versions are semantic versions; anything after a hyphen is a pre-release.
var version = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)

// built is the release a downloaded binary was made from: GoReleaser sets it (.goreleaser.yaml).
var built string

// releaseDev runs GoReleaser (.goreleaser.yaml) on this tool. On a version tag it publishes the
// archives and checksums to the tag's GitHub Release; anywhere else it is a snapshot: the same
// build into dist/, nothing published.
func releaseDev(args []string) error {
	tag, _, err := releaseTag("release-dev", args)
	if err != nil {
		return err
	}
	if tag == "" {
		return sh(".", "goreleaser", "release", "--snapshot", "--clean")
	}
	// Several tags sit on a release's commit (vX.Y.Z, and the modules' api-go/vX.Y.Z, dev/vX.Y.Z):
	// say which one this is. GoReleaser reads the token as GITHUB_TOKEN; the workflows set GH_TOKEN.
	env := []string{"GORELEASER_CURRENT_TAG=" + tag}
	if os.Getenv("GITHUB_TOKEN") == "" && os.Getenv("GH_TOKEN") != "" {
		env = append(env, "GITHUB_TOKEN="+os.Getenv("GH_TOKEN"))
	}
	cmd := exec.Command("goreleaser", "release", "--clean")
	cmd.Env, cmd.Stdout, cmd.Stderr = append(os.Environ(), env...), os.Stdout, os.Stderr
	return cmd.Run()
}

func distSDK(apis []string) error {
	if len(apis) == 0 {
		return errors.New("dist-sdk needs <api>...: folders in sdk/fern/apis")
	}
	for _, api := range apis {
		for _, group := range []string{"go", "typescript"} {
			dir := filepath.Join("sdk/out", api, group)
			// Fresh, so that what ships is what the committed specs give, and proven before it ships.
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			if err := sdkGen([]string{api, group}); err != nil {
				return err
			}
			if err := sdkCheck([]string{dir}); err != nil {
				return err
			}
			if err := archive(api+"-sdk-"+group, dir, nil); err != nil {
				return err
			}
		}
		if err := archive(api+"-specs", filepath.Join("sdk/fern/apis", api), []string{"openapi.json", "asyncapi.json"}); err != nil {
			return err
		}
	}
	return nil
}

func distCLI(args []string) error {
	var linux bool
	apis := flags("dist-cli", args, func(f *flag.FlagSet) {
		f.BoolVar(&linux, "linux", false, "build for Linux inside Docker (the container's arch) instead of for this machine")
	})
	if len(apis) == 0 {
		return errors.New("dist-cli needs <api>...: folders in sdk/fern/apis with a cli group")
	}
	if err := distDir(); err != nil {
		return err
	}
	for _, api := range apis {
		dir := filepath.Join("sdk/out", api, "cli")
		if err := sdkGen([]string{api, "cli"}); err != nil {
			return err
		}
		bin, err := cargoBin(dir)
		if err != nil {
			return err
		}
		build, target, goos := []string{dir}, "target", runtime.GOOS
		if linux {
			build, target, goos = []string{"-linux", dir}, "target-linux", "linux"
		}
		if err := cliBuild(build); err != nil {
			return err
		}
		// Both APIs' CLIs have the same binary name, so the file is named after its API.
		out := filepath.Join(dist, api+"-cli-"+goos+"-"+runtime.GOARCH)
		built, err := os.ReadFile(filepath.Join(dir, target, "release", bin))
		if err != nil {
			return err
		}
		if err := os.WriteFile(out, built, 0o755); err != nil {
			return err
		}
		fmt.Printf("built: %s (run it as %s)\n", out, bin)
	}
	return nil
}

// archive writes dist/<name>.tar.gz with dir's files under <name>/: the files named, or all of
// them except what a build or a check left there.
func archive(name, dir string, files []string) error {
	if len(files) == 0 {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			switch base := entry.Name(); {
			case entry.IsDir() && (base == "node_modules" || base == "target" || base == "target-linux" || base == ".git"):
				return filepath.SkipDir
			case entry.Type().IsRegular() && base != "tsconfig.check.json": // sdk-check writes that one
				rel, err := filepath.Rel(dir, path)
				files = append(files, rel)
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if err := distDir(); err != nil {
		return err
	}
	out, err := os.Create(filepath.Join(dist, name+".tar.gz"))
	if err != nil {
		return err
	}
	defer out.Close()
	zip := gzip.NewWriter(out)
	tape := tar.NewWriter(zip)
	for _, file := range files {
		in, err := os.Open(filepath.Join(dir, file))
		if err != nil {
			return err
		}
		info, err := in.Stat()
		if err != nil {
			return err
		}
		if err := tape.WriteHeader(&tar.Header{Name: name + "/" + filepath.ToSlash(file), Mode: int64(info.Mode().Perm()), Size: info.Size(), ModTime: info.ModTime()}); err != nil {
			return err
		}
		_, err = io.Copy(tape, in)
		in.Close()
		if err != nil {
			return err
		}
	}
	if err := tape.Close(); err != nil {
		return err
	}
	if err := zip.Close(); err != nil {
		return err
	}
	fmt.Printf("archived: %s (%d files)\n", out.Name(), len(files))
	return out.Close()
}

// releaseTag is the version to release: -tag, or the tag a GitHub workflow was started by. None
// means a dry run.
func releaseTag(name string, args []string) (tag string, rest []string, err error) {
	rest = flags(name, args, func(f *flag.FlagSet) {
		f.StringVar(&tag, "tag", "", "the version tag (default: the tag the workflow runs on; none: a dry run)")
	})
	if tag == "" && os.Getenv("GITHUB_REF_TYPE") == "tag" {
		tag = os.Getenv("GITHUB_REF_NAME")
	}
	if tag != "" && !version.MatchString(tag) {
		return "", nil, fmt.Errorf("%q is not a version tag: it must look like v1.2.3 or v1.2.3-rc.1", tag)
	}
	return tag, rest, nil
}

func release(args []string) error {
	tag, _, err := releaseTag("release", args)
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dist, "[^.]*")) // not the ignore file distDir writes
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("nothing in dist/: run a dist task first (mise run sdk:dist, sdk:dist:cli)")
	}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		fmt.Printf("  %-36s %9d B\n", filepath.Base(file), info.Size())
	}
	if tag == "" {
		fmt.Printf("dry run (not on a version tag): %d files built, nothing published\n", len(files))
		return nil
	}
	// Several jobs attach to the same release, and any of them may be the first.
	if exec.Command("gh", "release", "view", tag).Run() != nil {
		create := []string{"release", "create", tag, "--verify-tag", "--title", tag, "--generate-notes"}
		if strings.Contains(tag, "-") {
			create = append(create, "--prerelease")
		}
		if out, err := exec.Command("gh", create...).CombinedOutput(); err != nil && exec.Command("gh", "release", "view", tag).Run() != nil {
			return fmt.Errorf("gh release create %s: %s", tag, strings.TrimSpace(string(out)))
		}
	}
	if err := sh(".", "gh", append([]string{"release", "upload", tag, "--clobber"}, files...)...); err != nil {
		return err
	}
	fmt.Printf("release %s: %d files attached\n", tag, len(files))
	return nil
}

// releaseTags gives each Go module in a subdirectory the tag Go looks for: <dir>/vX.Y.Z, on the
// commit of vX.Y.Z. Through the GitHub API, so it needs no git credentials, only gh's token.
func releaseTags(args []string) error {
	tag, dirs, err := releaseTag("release-tags", args)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return errors.New("release-tags needs <module dir>..., e.g. api-go dev sdk/go")
	}
	var modules []string
	for _, dir := range dirs {
		switch {
		case !exists(dir): // a project's sdk/go before its first mise run sdk:publish
			fmt.Printf("skipped: %s (no such folder)\n", dir)
		case !exists(filepath.Join(dir, "go.mod")):
			return fmt.Errorf("%s has no go.mod", dir)
		default:
			modules = append(modules, dir)
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
		name := filepath.ToSlash(dir) + "/" + tag
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

func needEnv(names []string) error {
	var missing []string
	for _, name := range names {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("not set: %s. On GitHub, add each as a repository secret (gh secret set <NAME>, or Settings > Secrets and variables > Actions); locally, export them",
			strings.Join(missing, ", "))
	}
	return nil
}
