package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// charter publish: create the tag's GitHub Release if it has none, attach the files in dist/ it lacks, and write SHA256SUMS of everything on it (no tag: a dry run that lists dist/ and prints the notes)
// Run by: mise run release:publish (mise.toml); mise run release:publish (tasks/repo/tasks.toml); mise run ts:release (mise.toml)
func init() {
	commands["publish"] = command{"[-tag vX.Y.Z]", "create the tag's GitHub Release if it has none, attach the files in dist/ it lacks, and write SHA256SUMS of everything on it (no tag: a dry run that lists dist/ and prints the notes)", publish}
	anywhere["publish"] = true
}

// The checksums of everything on a release, in the format sha256sum -c reads.
const checksums = "SHA256SUMS"

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
