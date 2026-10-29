package repo

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joeblew999/charter/internal/run"
)

// Release runs charter release X.Y.Z [-dry-run]: a release of charter, cut from charter's own repo, onto GitHub.
//  1. The version: charter-lib's (ts/package.json) and every example's dependency on it, and every install command the
//     docs show (charter-lib's URL, mise use github:joeblew999/charter@X.Y.Z), committed.
//  2. charter check: the jobs charter.toml's [ci] check lists (the rest are the CI's: charter ci).
//  3. charter-lib packed (npm pack, into dist/), and the charter binary for every machine, built as a snapshot
//     (goreleaser, dist/charter), which proves it builds. -dry-run stops here and changes no file.
//  4. The tag vX.Y.Z (the Go library's: the repo's root module), pushed with main.
//  5. The GitHub Release vX.Y.Z, marked latest, with the binaries and charter-lib (.goreleaser.yaml): mise use
//     github:joeblew999/charter takes the binary, a TypeScript project's package.json charter-lib.
//
// Safe to run again: a step already done is skipped.
func Release(args []string) {
	flags := flag.NewFlagSet("charter release", flag.ExitOnError)
	dryRun := flags.Bool("dry-run", false, "only the check, the package and the build; change no file, tag and publish nothing")
	flags.Parse(flagsFirst(args))
	r := root()
	if !IsCharter(r) {
		check(errors.New("charter release cuts a release of charter itself, from its own repo"))
	}
	version := flags.Arg(0)
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
		check(errors.New("usage: charter release X.Y.Z [-dry-run]"))
	}
	// The Go library is the repo's root module, tagged vX.Y.Z. Before charter-ts the root held charter's Go tool,
	// tagged up to v0.19.1, and go get takes the highest tag, so a release is above that.
	n := 0
	for _, part := range strings.Split(version, ".") {
		i, _ := strconv.Atoi(part)
		n = n*1000 + i
	}
	if n <= 19001 {
		check(fmt.Errorf("%s is not above v0.19.1, the Go tool's last tag at the root: go get would take that instead", version))
	}
	git := func(args ...string) string {
		out, err := run.Output(r, nil, "git", args...)
		check(err)
		return strings.TrimSpace(out)
	}
	if git("branch", "--show-current") != "main" {
		check(errors.New("a release is cut from main"))
	}
	if git("status", "--porcelain") != "" {
		check(errors.New("the repo has changes that are not committed: commit them first"))
	}

	// 1. The version.
	lib := filepath.Join(r, "ts", "package.json")
	b, err := os.ReadFile(lib)
	check(err)
	have := regexp.MustCompile(`"version": "([^"]+)"`).FindStringSubmatch(string(b))
	if have == nil {
		check(errors.New("ts/package.json has no version"))
	}
	if have[1] != version && !*dryRun {
		fmt.Printf("charter-lib %s -> %s\n", have[1], version)
		changed := []string{"ts/package.json"}
		check(os.WriteFile(lib, []byte(strings.Replace(string(b), have[0], `"version": "`+version+`"`, 1)), 0o644))
		manifests, _ := filepath.Glob(filepath.Join(r, "examples", "*", "package.json"))
		for _, m := range manifests {
			b, err := os.ReadFile(m)
			check(err)
			next := regexp.MustCompile(`"charter-lib": "[^"]*"`).ReplaceAllString(string(b), `"charter-lib": "^`+version+`"`)
			if next != string(b) {
				check(os.WriteFile(m, []byte(next), 0o644))
				rel, _ := filepath.Rel(r, m)
				changed = append(changed, filepath.ToSlash(rel))
			}
		}
		// The commands the docs show, at this version.
		pages := []string{filepath.Join(r, "README.md")}
		filepath.WalkDir(filepath.Join(r, "docs"), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") && !strings.Contains(p, "/docs/"+folder+"/") {
				pages = append(pages, p)
			}
			return nil
		})
		libURL := regexp.MustCompile(`releases/download/v\d+\.\d+\.\d+/charter-lib-\d+\.\d+\.\d+\.tgz`)
		miseUse := regexp.MustCompile(`github:joeblew999/charter@\d+\.\d+\.\d+`)
		for _, page := range pages {
			b, err := os.ReadFile(page)
			if err != nil {
				continue
			}
			next := libURL.ReplaceAllString(string(b), "releases/download/v"+version+"/charter-lib-"+version+".tgz")
			next = miseUse.ReplaceAllString(next, "github:joeblew999/charter@"+version)
			if next != string(b) {
				check(os.WriteFile(page, []byte(next), 0o644))
				rel, _ := filepath.Rel(r, page)
				changed = append(changed, filepath.ToSlash(rel))
			}
		}
		check(run.Run(r, nil, "npm", "install", "--no-fund", "--no-audit"))
		check(run.Run(r, nil, "git", append([]string{"add", "--"}, append(changed, "package-lock.json")...)...))
		check(run.Run(r, nil, "git", "commit", "-q", "-m", "charter "+version))
	}

	// 2. The check: the jobs charter.toml's [ci] check lists.
	check(run.Run(r, nil, self(), "check"))

	// 3. charter-lib (a dry run packs the version there is), and the binaries, every machine's, built here first, so a
	// dry run proves they build.
	packed := version
	if *dryRun {
		packed = have[1]
	}
	check(os.MkdirAll(filepath.Join(r, "dist"), 0o755))
	check(run.Run(r, nil, "npm", "pack", "-w", "ts", "--pack-destination", filepath.Join(r, "dist")))
	tarball := filepath.Join(r, "dist", "charter-lib-"+packed+".tgz")
	info, err := os.Stat(tarball)
	check(err)
	fmt.Printf("made dist/charter-lib-%s.tgz (charter-lib, which the release carries): %.2f MB\n", packed, float64(info.Size())/1e6)
	check(run.Run(r, nil, "goreleaser", "build", "--snapshot", "--clean"))
	if *dryRun {
		fmt.Println("dry run: charter-lib and the binaries are what the release would carry; nothing tagged, pushed or published")
		return
	}

	// 4. The tag: the Go library is the root module, so go get finds its version by the tag vX.Y.Z.
	tag := "v" + version
	if git("tag", "-l", tag) == "" {
		check(run.Run(r, nil, "git", "tag", tag))
	}
	check(run.Run(r, nil, "git", "push", "-q", "origin", "main", tag))

	// 5. The release, marked latest: the binaries and charter-lib (.goreleaser.yaml). The repo has tags of several kinds
	// (ts-v, go/lib/v, examples/…), so goreleaser is told which is this release's and the one before.
	token, err := run.Output(r, nil, "gh", "auth", "token")
	check(err)
	env := map[string]string{"GITHUB_TOKEN": strings.TrimSpace(token), "GORELEASER_CURRENT_TAG": tag}
	for _, t := range strings.Split(git("tag", "-l", "v*", "--sort=-v:refname"), "\n") {
		if regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(t) && t != tag {
			env["GORELEASER_PREVIOUS_TAG"] = t
			break
		}
	}
	check(run.Run(r, env, "goreleaser", "release", "--clean"))
	fmt.Printf("released: charter %s: mise use github:joeblew999/charter@%s; charter-lib, %s\n", version, version, LibURL(version))
}
