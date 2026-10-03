package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
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
	"slices"
	"strings"
	"time"
)

// A release is one version tag (vX.Y.Z) on a commit, cut from a developer's machine with `release`
// (release.go's neighbour, cut.go): the project's checks, then the dist-* commands build what ships
// into dist/, the tag is pushed, `publish` attaches dist/ to the tag's GitHub Release, and
// `release-tags` adds the tags the Go modules in subdirectories need. The release workflow runs the
// same commands on the tag afterwards, on GitHub's machines. Without a tag, `publish` and
// `release-tags` only say what they would do, so the whole path can run on a branch or a pull request.

func init() {
	commands["release-tool"] = command{"[-tag vX.Y.Z]", "GoReleaser on this tool, in its own repo: on a version tag, archives and checksums go to its GitHub Release (if they are not there yet); no tag: a snapshot into dist/", releaseTool}
	commands["dist"] = command{"[-target <os>-<arch>,...]", "build everything the project ships into an emptied dist/: its SDKs and specs (dist-sdk), then its CLI if it has one (dist-cli)", distAll}
	commands["dist-sdk"] = command{"", "generate, check and archive the project's SDKs and its specs into dist/ (Docker)", distSDK}
	commands["dist-cli"] = command{"[-target <os>-<arch>,...]", "HEAVY: generate the project's Fern CLI and build it into dist/ for darwin, linux and windows on amd64 and arm64: every target this machine builds (a Mac builds all six)", distCLI}
	commands["cli-smoke"] = command{"", "run the CLI that dist/ holds for this machine with --version: proves a binary built elsewhere starts here", cliSmoke}
	commands["dist-ts"] = command{"[-tag vX.Y.Z]", "build the TypeScript library (ts/) and pack it, versioned as the tag, into dist/charter-ts-X.Y.Z.tgz: the package a project installs from the GitHub Release's URL (no tag: version 0.0.0-dev)", distTS}
	anywhere["dist-ts"], anywhere["publish"] = true, true
	commands["publish"] = command{"[-tag vX.Y.Z]", "create the tag's GitHub Release if it has none, attach the files in dist/ it lacks, and write SHA256SUMS of everything on it (no tag: a dry run that lists dist/ and prints the notes)", publish}
	commands["release-tags"] = command{"[-tag vX.Y.Z] <module dir>...", "tag each Go module in a subdirectory <path in the repo>/vX.Y.Z at the tag's commit (no tag: a dry run)", releaseTags}
	commands["need-env"] = command{"<NAME>...", "fail, naming them, unless these environment variables are set (the secrets a workflow needs)", needEnv}
	anywhere["release-tool"], anywhere["release-tags"], anywhere["need-env"] = true, true, true
}

// distName starts the name of everything a project ships: the project's folder name, so that the
// files of several projects can sit on one GitHub Release.
func distName() (string, error) {
	dir, err := os.Getwd()
	return filepath.Base(dir), err
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

// releaseTool runs GoReleaser (.goreleaser.yaml) on this tool. On a version tag it publishes the
// archives and checksums to the tag's GitHub Release, unless they are there already (a release cut
// from a machine has them: the workflow on the tag then only builds, as a check); anywhere else it is
// a snapshot: the same build into dist/, nothing published.
func releaseTool(args []string) error {
	tag, _, err := releaseTag("release-tool", args)
	if err != nil {
		return err
	}
	if tag != "" && slices.ContainsFunc(releaseAssets(tag), func(a asset) bool { return a.Name == "checksums.txt" }) {
		fmt.Printf("release %s already has the tool (checksums.txt): building it to check, publishing nothing\n", tag)
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

func distSDK(args []string) error {
	if len(args) != 0 {
		return errors.New("dist-sdk takes no arguments: it ships the project's SDKs")
	}
	name, err := distName()
	if err != nil {
		return err
	}
	// Every SDK the project defines ships: a language added to generators.yml is released with no
	// other change. Not the CLI (dist-cli builds it) and not a build made only for the tests.
	groups, err := sdkGroups(generators)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group == "cli" || strings.HasSuffix(group, "-dist") {
			continue
		}
		dir := sdkOut(group)
		// Fresh, so that what ships is what the committed specs give, and proven before it ships.
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		if err := sdkGen([]string{group}); err != nil {
			return err
		}
		if err := sdkCheckDir(dir); err != nil {
			return err
		}
		if err := archive(name+"-sdk-"+group, dir, nil); err != nil {
			return err
		}
	}
	return archive(name+"-specs", fernDir, []string{"openapi.json", "asyncapi.json"})
}

// distAll builds what the project ships into an emptied dist/, so nothing left from an older build
// goes onto a release: the SDKs and the specs, then the CLI if the project has one.
func distAll(args []string) error {
	var only string
	rest := flags("dist", args, func(f *flag.FlagSet) {
		f.StringVar(&only, "target", "", "only the CLI for these targets, as dist-cli takes them")
	})
	if len(rest) != 0 {
		return errors.New("dist takes no arguments: it builds what the project ships")
	}
	if only != "" && !hasCLI() {
		return fmt.Errorf("dist -target picks the CLI's targets: %w", errNoCLI)
	}
	if err := os.RemoveAll(dist); err != nil {
		return err
	}
	if err := distSDK(nil); err != nil {
		return err
	}
	if !hasCLI() {
		fmt.Println("no CLI: the project has no cli group in fern/generators.yml, so dist/ holds the SDKs and the specs")
		return nil
	}
	if only == "" {
		return distCLI(nil)
	}
	return distCLI([]string{"-target", only})
}

// A target of the CLI: the name a file on a release carries (<os>-<arch>, as Go and mise name them)
// and the Rust target it is built for. Linux is musl, a static binary that runs on any distribution
// (and the generated Cargo.toml then uses rustls, not the system's OpenSSL). Windows is the MinGW ABI,
// which zig links for both arches from any machine.
type cliTarget struct{ os, arch, rust string }

func (t cliTarget) String() string { return t.os + "-" + t.arch }

var cliTargets = []cliTarget{
	{"darwin", "amd64", "x86_64-apple-darwin"},
	{"darwin", "arm64", "aarch64-apple-darwin"},
	{"linux", "amd64", "x86_64-unknown-linux-musl"},
	{"linux", "arm64", "aarch64-unknown-linux-musl"},
	{"windows", "amd64", "x86_64-pc-windows-gnu"},
	{"windows", "arm64", "aarch64-pc-windows-gnullvm"},
}

// cliTargetsFor are the targets to build on a machine running host: those named (-target), or
// every one it can build. macOS's are built with Apple's linker, so only on a Mac; the others with
// zig (cargo-zigbuild), on any machine. skipped are the ones it can't.
func cliTargetsFor(host, only string) (build, skipped []cliTarget, err error) {
	can := func(t cliTarget) bool { return t.os != "darwin" || host == "darwin" }
	if only == "" {
		for _, t := range cliTargets {
			if can(t) {
				build = append(build, t)
			} else {
				skipped = append(skipped, t)
			}
		}
		return build, skipped, nil
	}
	for _, name := range strings.Split(only, ",") {
		at := slices.IndexFunc(cliTargets, func(t cliTarget) bool { return t.String() == strings.TrimSpace(name) })
		switch {
		case at < 0:
			return nil, nil, fmt.Errorf("no CLI target %q: the targets are %v", name, cliTargets)
		case !can(cliTargets[at]):
			return nil, nil, fmt.Errorf("%s builds only on a Mac (Apple's linker)", name)
		}
		build = append(build, cliTargets[at])
	}
	return build, nil, nil
}

// cliFile is the CLI's name on a release: after the project, not the binary, as two projects' CLIs
// can have the same binary name.
func cliFile(name string, t cliTarget) string {
	file := name + "-cli-" + t.String()
	if t.os == "windows" {
		file += ".exe"
	}
	return file
}

func distCLI(args []string) error {
	var only string
	rest := flags("dist-cli", args, func(f *flag.FlagSet) {
		f.StringVar(&only, "target", "", "only these, comma-separated: darwin-amd64, darwin-arm64, linux-amd64, linux-arm64, windows-amd64, windows-arm64 (default: every one this machine builds)")
	})
	if len(rest) != 0 {
		return errors.New("dist-cli takes no arguments: it builds the project's cli group")
	}
	if !hasCLI() {
		return errNoCLI
	}
	targets, skipped, err := cliTargetsFor(runtime.GOOS, only)
	if err != nil {
		return err
	}
	name, err := distName()
	if err != nil {
		return err
	}
	if err := distDir(); err != nil {
		return err
	}
	dir := sdkOut("cli")
	if err := sdkGen([]string{"cli"}); err != nil {
		return err
	}
	bin, err := cargoBin(dir)
	if err != nil {
		return err
	}
	for _, t := range targets {
		start := time.Now()
		if err := quiet(".", nil, "rustup", "target", "add", t.rust); err != nil {
			return fmt.Errorf("rustup target add %s: the project's mise.toml pins Rust (mise install)", t.rust)
		}
		program, build := "cargo", []string{"build", "--release", "--bin", bin, "--no-default-features", "--features", "rustls", "--target", t.rust}
		if t.os != "darwin" {
			if _, look := exec.LookPath("cargo-zigbuild"); look != nil {
				return errors.New("no cargo-zigbuild: the project's mise.toml pins it and zig (mise install)")
			}
			program, build[0] = "cargo-zigbuild", "zigbuild"
		}
		cargo := exec.Command(program, build...)
		cargo.Dir, cargo.Stdout, cargo.Stderr = dir, os.Stdout, os.Stderr
		if err := cargo.Run(); err != nil {
			return fmt.Errorf("the CLI for %s (%s): %w", t, t.rust, err)
		}
		built := bin
		if t.os == "windows" {
			built += ".exe"
		}
		content, err := os.ReadFile(filepath.Join(dir, "target", t.rust, "release", built))
		if err != nil {
			return err
		}
		out := filepath.Join(dist, cliFile(name, t))
		if err := os.WriteFile(out, content, 0o755); err != nil {
			return err
		}
		// About 450 MB a target, and no use to the next build: the crate Fern writes again compiles
		// every dependency again.
		if err := os.RemoveAll(filepath.Join(dir, "target", t.rust)); err != nil {
			return err
		}
		fmt.Printf("built: %s (%s, %s)\n", out, t.rust, time.Since(start).Round(time.Second))
	}
	for _, t := range skipped {
		fmt.Printf("skipped: %s (it builds only on a Mac)\n", t)
	}
	// The one this machine runs, run: a build that does not start fails here, not on a user's machine.
	if exists(filepath.Join(dist, cliFile(name, cliTarget{os: runtime.GOOS, arch: runtime.GOARCH}))) {
		return cliSmoke(nil)
	}
	return nil
}

// cliSmoke runs the CLI in dist/ built for this machine. The release workflow runs it on Windows,
// which no other machine can start a Windows binary on.
func cliSmoke(args []string) error {
	if len(args) != 0 {
		return errors.New("cli-smoke takes no arguments: it runs the CLI in dist/ for this machine")
	}
	if !hasCLI() {
		return errNoCLI
	}
	name, err := distName()
	if err != nil {
		return err
	}
	file := filepath.Join(dist, cliFile(name, cliTarget{os: runtime.GOOS, arch: runtime.GOARCH}))
	if !exists(file) {
		return fmt.Errorf("no %s: mise run sdk:dist:cli builds it (or the release workflow's cli job)", file)
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	if err := os.Chmod(abs, 0o755); err != nil { // a downloaded artifact loses the mode
		return err
	}
	said, err := output(".", abs, "--version")
	if err != nil {
		return fmt.Errorf("%s --version: %w", file, err)
	}
	fmt.Printf("ran: %s --version: %s\n", file, said)
	return nil
}

// The TypeScript library and the version its package.json carries in the repo, which a release's
// replaces: the package is versioned only as it is packed.
const (
	tsLibrary   = "ts"
	tsDevelop   = `"version": "0.0.0",`
	tsDryRunVer = "0.0.0-dev"
)

// distTS packs the TypeScript library as npm would publish it, without a registry: a tarball for
// the GitHub Release, which a project installs by its URL. It is compiled first (a package in
// node_modules must be JavaScript: Node strips types only outside it). The pack runs on a copy, so
// the version is set without touching the repo's package.json.
func distTS(args []string) error {
	tag, rest, err := releaseTag("dist-ts", args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return errors.New("dist-ts takes no arguments: it packs " + tsLibrary + "/")
	}
	ver := strings.TrimPrefix(tag, "v")
	if ver == "" {
		ver = tsDryRunVer
	}
	manifest, err := os.ReadFile(filepath.Join(tsLibrary, "package.json"))
	if err != nil {
		return fmt.Errorf("dist-ts runs at the root of the charter repo: %w", err)
	}
	if !strings.Contains(string(manifest), tsDevelop) {
		return fmt.Errorf("%s/package.json has no %s line to set the version in", tsLibrary, tsDevelop)
	}
	if err := sh(tsLibrary, "npm", "run", "build"); err != nil {
		return err
	}
	stage, err := os.MkdirTemp("", "charter-ts-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.CopyFS(filepath.Join(stage, "dist"), os.DirFS(filepath.Join(tsLibrary, "dist"))); err != nil {
		return err
	}
	versioned := strings.Replace(string(manifest), tsDevelop, `"version": "`+ver+`",`, 1)
	if err := os.WriteFile(filepath.Join(stage, "package.json"), []byte(versioned), 0o644); err != nil {
		return err
	}
	if license, err := os.ReadFile("LICENSE"); err == nil {
		if err := os.WriteFile(filepath.Join(stage, "LICENSE"), license, 0o644); err != nil {
			return err
		}
	}
	if err := distDir(); err != nil {
		return err
	}
	out, err := filepath.Abs(dist)
	if err != nil {
		return err
	}
	if err := quiet(stage, nil, "npm", "pack", "--pack-destination", out); err != nil {
		return err
	}
	fmt.Printf("packed: %s\n", filepath.Join(dist, "charter-ts-"+ver+".tgz"))
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

// releaseTagEnv carries the tag from `release` to the tasks it runs (dist, release:publish,
// release:tags), as GitHub's variables carry it in a workflow on a tag.
const releaseTagEnv = "CHARTER_RELEASE_TAG"

// releaseTag is the version to release: -tag, or the tag `release` or a GitHub workflow was started
// for. None means a dry run.
func releaseTag(name string, args []string) (tag string, rest []string, err error) {
	rest = flags(name, args, func(f *flag.FlagSet) {
		f.StringVar(&tag, "tag", "", "the version tag (default: the tag being released, or the tag the workflow runs on; none: a dry run)")
	})
	if tag == "" {
		tag = os.Getenv(releaseTagEnv)
	}
	if tag == "" && os.Getenv("GITHUB_REF_TYPE") == "tag" {
		tag = os.Getenv("GITHUB_REF_NAME")
	}
	if tag != "" && !version.MatchString(tag) {
		return "", nil, fmt.Errorf("%q is not a version tag: it must look like v1.2.3 or v1.2.3-rc.1", tag)
	}
	return tag, rest, nil
}

// The checksums of everything on a release, in the format sha256sum -c reads.
const checksums = "SHA256SUMS"

// distFiles are the files in dist/ a release takes: not the ignore file distDir writes, not folders.
func distFiles() ([]string, error) {
	found, err := filepath.Glob(filepath.Join(dist, "[^.]*"))
	if err != nil {
		return nil, err
	}
	var files []string
	for _, file := range found {
		if info, err := os.Stat(file); err == nil && info.Mode().IsRegular() {
			files = append(files, file)
		}
	}
	return files, nil
}

// publish brings the tag's GitHub Release up to what this checkout built. Several jobs, and a
// release cut from a machine before them, publish to the same Release: any of them may be the first,
// a file already on it is kept (the first build of it is the one released), and SHA256SUMS is
// written again from what is on it. A repo that ships no files gets the Release and its notes.
func publish(args []string) error {
	tag, rest, err := releaseTag("publish", args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return errors.New("publish takes no arguments: it attaches dist/")
	}
	files, err := distFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		fmt.Printf("  %-40s %9d B\n", filepath.Base(file), info.Size())
	}
	if len(files) == 0 {
		fmt.Println("  dist/ is empty: the release carries its notes only")
	}
	if tag == "" {
		notes, err := releaseNotes("HEAD", "")
		if err != nil {
			return err
		}
		fmt.Printf("%s\ndry run (not on a version tag): %d files built, nothing published\n", notes, len(files))
		return nil
	}
	if err := ensureRelease(tag, false, "", false); err != nil {
		return err
	}
	have := map[string]bool{}
	for _, a := range releaseAssets(tag) {
		have[a.Name] = true
	}
	var attach []string
	for _, file := range files {
		if have[filepath.Base(file)] {
			fmt.Printf("kept: %s (already on the release)\n", filepath.Base(file))
		} else {
			attach = append(attach, file)
		}
	}
	if len(attach) > 0 {
		if err := sh(".", "gh", append([]string{"release", "upload", tag}, attach...)...); err != nil {
			return err
		}
	}
	if err := writeChecksums(tag); err != nil {
		return err
	}
	fmt.Printf("release %s: %d files attached, %d already there\n", tag, len(attach), len(files)-len(attach))
	return nil
}

// ensureRelease makes the tag's GitHub Release if there is none, with the notes of releaseNotes;
// with update, it also sets the notes and pre-release of one that is there (a workflow on the tag
// may have made it first). A tag with a hyphen (v1.2.0-rc.1) is always a pre-release.
func ensureRelease(tag string, prerelease bool, footer string, update bool) error {
	prerelease = prerelease || strings.Contains(tag, "-")
	exists := exec.Command("gh", "release", "view", tag).Run() == nil
	if exists && !update {
		return nil
	}
	notes, err := releaseNotes(tag, footer)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp("", "release-notes-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(notes); err != nil {
		return err
	}
	file.Close()
	if exists {
		return quiet(".", nil, "gh", "release", "edit", tag, "--title", tag, "--notes-file", file.Name(), fmt.Sprintf("--prerelease=%t", prerelease))
	}
	create := []string{"release", "create", tag, "--verify-tag", "--title", tag, "--notes-file", file.Name()}
	if prerelease {
		create = append(create, "--prerelease")
	}
	if out, err := exec.Command("gh", create...).CombinedOutput(); err != nil && exec.Command("gh", "release", "view", tag).Run() != nil {
		return fmt.Errorf("gh release create %s: %s", tag, strings.TrimSpace(string(out)))
	}
	return nil
}

// releaseNotes are the subjects of the commits since the version tag before target (all of them
// for the first), and the footer: text, or a file's.
func releaseNotes(target, footer string) (string, error) {
	logRange, heading := target, "## Changes"
	if before, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*", target+"^").Output(); err == nil {
		previous := strings.TrimSpace(string(before))
		logRange, heading = previous+".."+target, "## Changes since "+previous
	}
	subjects, err := exec.Command("git", "log", "--no-merges", "--format=- %s", logRange).Output()
	if err != nil {
		return "", fmt.Errorf("git log %s: %w", logRange, err)
	}
	notes := heading + "\n\n" + strings.TrimSpace(string(subjects)) + "\n"
	if footer != "" {
		if content, err := os.ReadFile(footer); err == nil {
			footer = string(content)
		}
		notes += "\n" + strings.TrimSpace(footer) + "\n"
	}
	return notes, nil
}

// An asset of a GitHub Release, with the SHA-256 GitHub computed when it was uploaded.
type asset struct{ Name, Digest string }

// releaseAssets are what the tag's Release carries; none if there is no Release.
func releaseAssets(tag string) []asset {
	out, err := exec.Command("gh", "api", "repos/{owner}/{repo}/releases/tags/"+tag).Output()
	if err != nil {
		return nil
	}
	var release struct{ Assets []asset }
	if json.Unmarshal(out, &release) != nil {
		return nil
	}
	return release.Assets
}

// checksumsOf is SHA256SUMS for these assets (not itself), sorted by name; empty if there are none.
func checksumsOf(assets []asset) (string, error) {
	var lines []string
	for _, a := range assets {
		if a.Name == checksums {
			continue
		}
		sum, ok := strings.CutPrefix(a.Digest, "sha256:")
		if !ok {
			return "", fmt.Errorf("GitHub has no SHA-256 of %s (%q): upload it again", a.Name, a.Digest)
		}
		lines = append(lines, sum+"  "+a.Name)
	}
	slices.SortFunc(lines, func(a, b string) int { return strings.Compare(a[strings.Index(a, "  "):], b[strings.Index(b, "  "):]) })
	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// writeChecksums writes SHA256SUMS from the digests GitHub holds, so it covers every file on the
// Release, whoever attached it. Jobs publish at once: after writing, it reads the Release again, and
// writes again if a file came in meanwhile, so the last writer leaves the full list.
func writeChecksums(tag string) error {
	dir, err := os.MkdirTemp("", "checksums-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for range 5 {
		sums, err := checksumsOf(releaseAssets(tag))
		if err != nil || sums == "" {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, checksums), []byte(sums), 0o644); err != nil {
			return err
		}
		if err := quiet(".", nil, "gh", "release", "upload", tag, "--clobber", filepath.Join(dir, checksums)); err != nil {
			return err
		}
		if again, err := checksumsOf(releaseAssets(tag)); err == nil && again == sums {
			fmt.Printf("%s: %d files\n", checksums, strings.Count(sums, "\n"))
			return nil
		}
	}
	return fmt.Errorf("%s: the release kept changing while it was written", checksums)
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
		return errors.New("release-tags needs <module dir>..., e.g. sdk/go")
	}
	// Where this folder is in the repo: a module's tag is its path from the repo's root.
	prefix, err := output(".", "git", "rev-parse", "--show-prefix")
	if err != nil {
		return errors.New("release-tags needs a git repo here")
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
