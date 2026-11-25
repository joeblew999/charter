package ci

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/deps"
	"github.com/joeblew999/charter/project"
)

// UpdateBranch is the branch the weekly job keeps its pull request on: one branch, moved each week, so an update
// nobody merged is replaced by the next and never piles up.
const UpdateBranch = "charter/update"

// update is charter ci update: the repo's update command, then, on GitHub, a pull request with what it changed. It
// returns the exit code.
//
// The command moves the pins and proves them (charter update ends with charter check; an app's own, mise run update,
// with its checks): a failure stops here with nothing pushed, and the run is red. GitHub starts no workflow for a
// branch its own token pushed, so the pull request has no checks of its own: the proof is this run, which it links.
func update(p *project.Project, gh github) int {
	var t timings
	defer func() { t.report() }()
	// Every tool: an update runs the repo's whole check, not only what a push's needs.
	if _, err := os.Stat(filepath.Join(p.Root, "mise.toml")); err == nil && !t.phase("tools: mise install", sh("mise install", nil)) {
		return 1
	}
	said, command := p.CI.Update, p.CI.Update
	if command == "" {
		self, err := os.Executable()
		if err != nil {
			log.Print("✗ ", err)
			return 1
		}
		said, command = "charter update", self+" update"
	}
	if !t.phase("update: "+said, sh(command, nil)) {
		return 1
	}
	dirty, err := deps.Dirty(p.Root)
	if err != nil {
		log.Print("✗ ", err)
		return 1
	}
	if !dirty {
		fmt.Println("✓ nothing moved: every pin is at its latest, or held")
		return 0
	}
	if os.Getenv("GITHUB_ACTIONS") == "" || gh.repo == "" || gh.token == "" {
		fmt.Println("✓ the pins moved and their checks passed. Not on GitHub, so no pull request: commit them from here.")
		return 0
	}
	if !t.phase("pull request: "+UpdateBranch, func() error { return propose(p.Root, gh, said) }) {
		return 1
	}
	return 0
}

// propose commits the working tree to the update branch, pushes it, and opens its pull request unless one is open.
func propose(root string, gh github, command string) error {
	git := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", root,
			"-c", "user.name=github-actions[bot]", "-c", "user.email=41898282+github-actions[bot]@users.noreply.github.com"}, args...)...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return cmd.Run()
	}
	stat, _ := exec.Command("git", "-C", root, "status", "--short").Output()
	for _, step := range [][]string{{"checkout", "-q", "-B", UpdateBranch}, {"add", "-A"}, {"commit", "-q", "-m", "Update pins"}} {
		if err := git(step...); err != nil {
			return fmt.Errorf("git %s: %w", step[0], err)
		}
	}
	if err := pushBranch(gh, root, UpdateBranch); err != nil {
		return err
	}
	owner, _, _ := strings.Cut(gh.repo, "/")
	var open []struct {
		URL string `json:"html_url"`
	}
	if err := gh.do(http.MethodGet, "/pulls?state=open&head="+url.QueryEscape(owner+":"+UpdateBranch), nil, &open); err != nil {
		return err
	}
	if len(open) > 0 {
		log.Printf("✓ %s moved to this update: its pull request is %s", UpdateBranch, open[0].URL)
		return nil
	}
	var made struct {
		URL string `json:"html_url"`
	}
	if err := gh.do(http.MethodPost, "/pulls", map[string]string{
		"title": "Update pins", "head": UpdateBranch, "base": project.StagingBranch, "body": body(command, string(stat)),
	}, &made); err != nil {
		return fmt.Errorf("opening the pull request: %w", err)
	}
	log.Printf("✓ pull request: %s", made.URL)
	return nil
}

// body is the pull request's text: what ran, what it changed, and where its proof is.
func body(command, stat string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The weekly update: `%s` moved the repo's pins to their latest, except what `deps.toml` holds, and its checks passed.\n\n", command)
	if run := os.Getenv("GITHUB_RUN_ID"); run != "" {
		fmt.Fprintf(&b, "**The checks are that run's:** %s/%s/actions/runs/%s. GitHub starts no workflow for a branch its own token pushed, so this pull request shows none of its own.\n\n",
			strings.TrimSuffix(os.Getenv("GITHUB_SERVER_URL"), "/"), os.Getenv("GITHUB_REPOSITORY"), run)
	}
	b.WriteString("Changed:\n\n```\n" + strings.TrimRight(stat, "\n") + "\n```\n\n")
	b.WriteString("Something to keep where it is: a line in `deps.toml`'s `[hold]`, with why. The branch is moved by the next update, so merge it or leave it.\n")
	return b.String()
}

// pushBranch force-pushes root's branch to the repository with the job's token.
var pushBranch = func(gh github, root, branch string) error {
	cmd := exec.Command("git", "-C", root, "push", "--force", "https://x-access-token:"+gh.token+"@github.com/"+gh.repo+".git",
		branch+":refs/heads/"+branch)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pushing %s: %w", branch, err)
	}
	return nil
}
