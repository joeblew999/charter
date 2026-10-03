package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReleaseArgs(t *testing.T) {
	for _, args := range [][]string{
		{"v1.2.3", "-dry-run", "-notes-footer", "x"},
		{"-dry-run", "v1.2.3", "-notes-footer", "x"},
		{"-notes-footer", "x", "-dry-run", "v1.2.3"},
	} {
		tag, dryRun, prerelease, footer, err := releaseArgs(args)
		if err != nil || tag != "v1.2.3" || !dryRun || prerelease || footer != "x" {
			t.Errorf("releaseArgs(%q) = %q %t %t %q %v", args, tag, dryRun, prerelease, footer, err)
		}
	}
	for _, args := range [][]string{{}, {"1.2.3"}, {"v1.2.3", "v1.2.4"}, {"-dry-run"}} {
		if _, _, _, _, err := releaseArgs(args); err == nil {
			t.Errorf("releaseArgs(%q): no error", args)
		}
	}
}

func TestCLITargets(t *testing.T) {
	names := func(targets []cliTarget) (out []string) {
		for _, target := range targets {
			out = append(out, target.String())
		}
		return out
	}
	build, skipped, err := cliTargetsFor("darwin", "")
	if err != nil || len(build) != 6 || len(skipped) != 0 {
		t.Errorf("on a Mac: %v, skipped %v (%v), want all six", names(build), names(skipped), err)
	}
	build, skipped, err = cliTargetsFor("linux", "")
	if want := []string{"linux-amd64", "linux-arm64", "windows-amd64", "windows-arm64"}; err != nil || !slices.Equal(names(build), want) || !slices.Equal(names(skipped), []string{"darwin-amd64", "darwin-arm64"}) {
		t.Errorf("on Linux: %v, skipped %v (%v), want %v", names(build), names(skipped), err, want)
	}
	if build, _, err = cliTargetsFor("windows", "windows-arm64,linux-amd64"); err != nil || !slices.Equal(names(build), []string{"windows-arm64", "linux-amd64"}) {
		t.Errorf("-target windows-arm64,linux-amd64: %v (%v)", names(build), err)
	}
	if _, _, err = cliTargetsFor("linux", "darwin-arm64"); err == nil || !strings.Contains(err.Error(), "Mac") {
		t.Errorf("-target darwin-arm64 on Linux: %v, want an error that says it needs a Mac", err)
	}
	if _, _, err = cliTargetsFor("darwin", "freebsd-amd64"); err == nil {
		t.Error("-target freebsd-amd64: no error")
	}
	if got := cliFile("billing", cliTarget{os: "windows", arch: "arm64"}); got != "billing-cli-windows-arm64.exe" {
		t.Errorf("cliFile = %q", got)
	}
}

func TestChecksums(t *testing.T) {
	sums, err := checksumsOf([]asset{
		{"b-cli-linux-amd64", "sha256:bb"},
		{checksums, "sha256:ff"},
		{"a-specs.tar.gz", "sha256:aa"},
	})
	if want := "aa  a-specs.tar.gz\nbb  b-cli-linux-amd64\n"; err != nil || sums != want {
		t.Errorf("checksumsOf = %q (%v), want %q", sums, err, want)
	}
	if sums, err := checksumsOf([]asset{{checksums, "sha256:ff"}}); err != nil || sums != "" {
		t.Errorf("a release with only %s: %q (%v), want nothing", checksums, sums, err)
	}
	if _, err := checksumsOf([]asset{{"old", ""}}); err == nil {
		t.Error("an asset without a digest: no error")
	}
}

// gitIn runs git in dir, with an identity and no signing, whatever the machine's settings.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgSign=false", "-c", "tag.gpgSign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// A repo with its origin (a bare repo beside it) at the same commit, and the test in it.
func releaseRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin, work := filepath.Join(root, "origin.git"), filepath.Join(root, "work")
	gitIn(t, root, "init", "--quiet", "--bare", "-b", "main", origin)
	gitIn(t, root, "clone", "--quiet", origin, work)
	commit := func(subject string) {
		if err := os.WriteFile(filepath.Join(work, "file"), []byte(subject), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, work, "add", "file")
		gitIn(t, work, "commit", "--quiet", "-m", subject)
	}
	commit("first")
	gitIn(t, work, "tag", "-a", "v0.1.0", "-m", "v0.1.0")
	commit("second")
	commit("third")
	gitIn(t, work, "push", "--quiet", "origin", "main", "v0.1.0")
	t.Chdir(work)
	return work
}

func TestReleaseChecksAndNotes(t *testing.T) {
	work := releaseRepo(t)
	state := func(tag string) map[string]bool {
		t.Helper()
		checks, err := releaseChecks(tag)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, check := range checks {
			got[check.name] = check.ok
		}
		return got
	}
	if got := state("v0.2.0"); !got["clean"] || !got["main"] || !got["new here"] || !got["new on origin"] {
		t.Errorf("a clean checkout of main, a new tag: %v", got)
	}
	if got := state("v0.1.0"); got["new here"] || got["new on origin"] {
		t.Errorf("a tag that exists: %v", got)
	}
	if err := os.WriteFile(filepath.Join(work, "file"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := state("v0.2.0"); got["clean"] {
		t.Errorf("a changed file: %v", got)
	}
	gitIn(t, work, "commit", "--quiet", "-am", "fourth")
	if got := state("v0.2.0"); got["main"] {
		t.Errorf("a commit GitHub does not have: %v", got)
	}

	notes, err := releaseNotes("HEAD", "Tested: docs/findings.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := "## Changes since v0.1.0\n\n- fourth\n- third\n- second\n\nTested: docs/findings.md\n"; notes != want {
		t.Errorf("notes:\n%s\nwant:\n%s", notes, want)
	}
	footer := filepath.Join(t.TempDir(), "footer.md")
	if err := os.WriteFile(footer, []byte("From a file.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if notes, err := releaseNotes("v0.1.0", footer); err != nil || notes != "## Changes\n\n- first\n\nFrom a file.\n" {
		t.Errorf("the first release's notes, footer from a file: %q (%v)", notes, err)
	}

	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir("sub")
	if _, err := releaseChecks("v0.2.0"); err == nil || !strings.Contains(err.Error(), "root") {
		t.Errorf("from a subfolder: %v, want an error that says to run it at the root", err)
	}
}

// A dry run checks, builds and prints, and leaves no tag, here or on origin.
func TestReleaseDryRun(t *testing.T) {
	if exec.Command("gh", "auth", "status").Run() != nil {
		t.Skip("gh is not signed in: release checks that first")
	}
	work := releaseRepo(t)
	if err := release([]string{"v0.2.0", "-dry-run"}); err != nil {
		t.Fatal(err)
	}
	if tags := gitIn(t, work, "tag", "--list", "v0.2.0"); tags != "" {
		t.Errorf("a dry run made the tag here")
	}
	if remote := gitIn(t, work, "ls-remote", "--tags", "origin"); strings.Contains(remote, "v0.2.0") {
		t.Errorf("a dry run pushed the tag")
	}
	if err := release([]string{"v0.1.0", "-dry-run"}); err == nil || !strings.Contains(err.Error(), "a check failed") {
		t.Errorf("a dry run of a tag that exists: %v, want a failed check", err)
	}
}
