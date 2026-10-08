package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// `release` cuts a release from a developer's machine, in minutes: the checks the repo's CI runs,
// the build, the tag, the GitHub Release with every file. GitHub's workflows then run on the tag, on
// every OS, as a check after the fact. It works in any repo whose tasks follow charter's names, and
// in one that ships no files: each step is a mise task the repo may or may not have.
//
//	setup            installs what the checks need, as CI does first (none: skipped)
//	check            the repo's own checks (none: skipped)
//	spec:diff        fails on a breaking change to the specs unless the tag is a major release (none: skipped)
//	dist             builds what ships into dist/ (none: the repo ships no files)
//	release:publish  attaches dist/ to the Release (none: publish here)
//	release:tags     tags Go modules in subdirectories (none: skipped)

func init() {
	commands["release"] = command{"vX.Y.Z [-dry-run] [-prerelease] [-notes-footer <text or file>]",
		"cut a release from this machine: check the repo is clean, is the default branch on GitHub and has no failed workflow there, run its setup, check, spec:diff and dist tasks, tag, push the tag, make the GitHub Release (notes: the commits since the last tag) and run its release:publish and release:tags tasks; -dry-run changes nothing", release}
	anywhere["release"] = true
}

// releaseArgs takes the tag and the flags in either order: mise run release -- vX.Y.Z -dry-run.
func releaseArgs(args []string) (tag string, dryRun, prerelease bool, footer string, err error) {
	set := flag.NewFlagSet("release", flag.ContinueOnError)
	set.BoolVar(&dryRun, "dry-run", false, "run every check and the build, say what would be done, change nothing")
	set.BoolVar(&prerelease, "prerelease", false, "mark the Release a pre-release (a tag with a hyphen always is)")
	set.StringVar(&footer, "notes-footer", "", "a line for the end of the notes, or a file that holds it")
	var rest []string
	for {
		if err := set.Parse(args); err != nil {
			return "", false, false, "", err
		}
		if set.NArg() == 0 {
			break
		}
		rest, args = append(rest, set.Arg(0)), set.Args()[1:]
	}
	if len(rest) != 1 {
		return "", false, false, "", errors.New("release needs one version tag: mise run release -- vX.Y.Z [-dry-run]")
	}
	if !version.MatchString(rest[0]) {
		return "", false, false, "", fmt.Errorf("%q is not a version tag: it must look like v1.2.3 or v1.2.3-rc.1", rest[0])
	}
	return rest[0], dryRun, prerelease, footer, nil
}

// releaseCheck is one thing that must hold before a tag is cut.
type releaseCheck struct {
	name, what string
	ok         bool
}

// releaseChecks say whether this checkout may be released as tag: at the repo's root, nothing
// uncommitted, HEAD what GitHub's default branch is, the tag new here and on GitHub.
func releaseChecks(tag string) ([]releaseCheck, error) {
	top, err := output(".", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, errors.New("release needs a git repo")
	}
	here, _ := filepath.Abs(".")
	here, _ = filepath.EvalSymlinks(here)
	if there, _ := filepath.EvalSymlinks(top); here != there {
		return nil, fmt.Errorf("a release is the whole repo's: run it at its root, %s", top)
	}
	// The tags GitHub has, so the notes start at the last release, and the check below is current.
	_ = quiet(".", nil, "git", "fetch", "--quiet", "--tags", "origin")
	head, err := output(".", "git", "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	remote, err := output(".", "git", "ls-remote", "origin", "HEAD")
	if err != nil {
		return nil, errors.New("git ls-remote origin: release needs the repo's remote, origin")
	}
	main, _, _ := strings.Cut(remote, "\t")
	status, err := output(".", "git", "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	local := exec.Command("git", "rev-parse", "--quiet", "--verify", "refs/tags/"+tag).Run() != nil
	onGitHub, err := output(".", "git", "ls-remote", "--tags", "origin", "refs/tags/"+tag)
	if err != nil {
		return nil, err
	}
	signedIn := exec.Command("gh", "auth", "status").Run() == nil
	ciOK, ci := ciPassed(head)
	return []releaseCheck{
		{"clean", "the working tree is clean", status == ""},
		{"main", fmt.Sprintf("HEAD (%.7s) is the default branch on origin (%.7s)", head, main), head == main},
		{"new here", tag + " is not a tag here", local},
		{"new on origin", tag + " is not a tag on origin", onGitHub == ""},
		{"gh", "gh is signed in", signedIn},
		{"ci", ci, ciOK},
	}, nil
}

// ciPassed says whether every workflow GitHub ran on a push of commit passed, on every OS it runs:
// a release is cut only from a commit that is green on the default branch. A repo with no workflows
// has nothing to wait for.
func ciPassed(commit string) (bool, string) {
	if files, _ := filepath.Glob(filepath.Join(".github", "workflows", "*.y*ml")); len(files) == 0 {
		return true, "no workflows here: nothing on GitHub to wait for"
	}
	out, err := output(".", "gh", "run", "list", "--commit", commit, "--event", "push", "--limit", "50", "--json", "workflowName,status,conclusion")
	if err != nil {
		return false, "GitHub's runs on HEAD could not be read (gh run list)"
	}
	var runs []struct{ WorkflowName, Status, Conclusion string }
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		return false, "GitHub's runs on HEAD could not be read: " + err.Error()
	}
	if len(runs) == 0 {
		return true, fmt.Sprintf("GitHub has run no workflow on HEAD (%.7s) yet: not waited for, the checks run here", commit)
	}
	var waiting, failed []string
	for _, run := range runs {
		switch {
		case run.Status != "completed":
			waiting = append(waiting, run.WorkflowName)
		case run.Conclusion != "success" && run.Conclusion != "skipped":
			failed = append(failed, run.WorkflowName+" "+run.Conclusion)
		}
	}
	switch {
	case len(failed) > 0:
		return false, "CI failed on HEAD: " + strings.Join(failed, ", ")
	case len(waiting) > 0:
		// Every step of a workflow is a mise task, and release runs check here: a runner that has
		// not got to it yet (GitHub may have none free for an hour) tells nothing a failure would.
		return true, "CI has not finished on HEAD (" + strings.Join(waiting, ", ") + "): not waited for, the checks run here"
	}
	return true, fmt.Sprintf("CI passed on HEAD: %d workflow runs", len(runs))
}

func release(args []string) error {
	tag, dryRun, prerelease, footer, err := releaseArgs(args)
	if err != nil {
		return err
	}
	start := time.Now()
	checks, err := releaseChecks(tag)
	if err != nil {
		return err
	}
	failed := false
	for _, check := range checks {
		mark := "ok  "
		if !check.ok {
			mark, failed = "FAIL", true
		}
		fmt.Printf("  %s  %s\n", mark, check.what)
	}
	if failed {
		return errors.New("not released: a check failed")
	}
	tasks := miseTasks(".")
	// The tag is not cut yet: the tasks are told it, as a workflow on the tag is told by GitHub.
	run := func(task string) error {
		if !tasks[task] {
			return nil
		}
		fmt.Printf("== mise run %s\n", task)
		cmd := exec.Command("mise", "run", task)
		cmd.Stdout, cmd.Stderr, cmd.Env = os.Stdout, os.Stderr, append(os.Environ(), releaseTagEnv+"="+tag)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("not released: mise run %s failed", task)
		}
		return nil
	}
	if !tasks["check"] {
		fmt.Println("  no check task: nothing to run before the tag")
	}
	// As CI runs it: setup (the packages a fresh clone lacks), then check.
	if err := run("setup"); err != nil {
		return err
	}
	if err := run("check"); err != nil {
		return err
	}
	// A breaking change to the API's specs since the last release needs a major version.
	if err := run("spec:diff"); err != nil {
		return err
	}
	if err := os.RemoveAll(dist); err != nil {
		return err
	}
	if !tasks["dist"] {
		fmt.Println("  no dist task: this repo ships no files")
	}
	if err := run("dist"); err != nil {
		return err
	}
	files, err := distFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		fmt.Printf("  built: %s\n", file)
	}
	if dryRun {
		notes, err := releaseNotes("HEAD", footer)
		if err != nil {
			return err
		}
		fmt.Printf("\n%s\n", notes)
		fmt.Printf("  would: git tag -a %s, git push origin %s\n", tag, tag)
		fmt.Printf("  would: make the GitHub Release %s (pre-release: %t) with these notes\n", tag, prerelease || strings.Contains(tag, "-"))
		for _, task := range []string{"release:publish", "release:tags"} {
			if tasks[task] {
				fmt.Printf("  would: mise run %s\n", task)
			}
		}
		fmt.Printf("release: dry run of %s finished in %s; nothing was changed\n", tag, time.Since(start).Round(time.Second))
		return nil
	}
	if err := quiet(".", nil, "git", "tag", "-a", tag, "-m", tag); err != nil {
		return err
	}
	if err := quiet(".", nil, "git", "push", "--quiet", "origin", "refs/tags/"+tag); err != nil {
		_ = quiet(".", nil, "git", "tag", "-d", tag) // so that the same command can run again
		return fmt.Errorf("git push origin %s failed: the tag is removed here again", tag)
	}
	fmt.Printf("  tagged and pushed: %s\n", tag)
	if err := ensureRelease(tag, prerelease, footer, true); err != nil {
		return err
	}
	if tasks["release:publish"] {
		err = run("release:publish")
	} else {
		err = publish([]string{"-tag", tag})
	}
	if err != nil {
		return err
	}
	if err := run("release:tags"); err != nil {
		return err
	}
	url, _ := output(".", "gh", "release", "view", tag, "--json", "url", "--jq", ".url")
	fmt.Printf("release: %s is out in %s: %s\nGitHub's workflows now check the tag on every OS.\n", tag, time.Since(start).Round(time.Second), url)
	return nil
}
