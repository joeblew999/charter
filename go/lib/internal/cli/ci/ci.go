// Package ci is go tool charter ci publish|release: the GitHub CI side of a Worker's previews and releases, so a
// commit is compiled once, on CI, and Cloudflare deploys that build (go tool charter builds).
//
//	go tool charter ci publish            this commit's build (build/, the Static Assets) and this binary to the
//	                                      repository's preview-builds pre-release; on a branch with a preview, its
//	                                      URL on the branch's pull request
//	go tool charter ci release <version>  a v* tag or a release/v* branch whose checks passed: on main, tag it (a
//	                                      branch), run -hook, move the production branch to it (Workers Builds
//	                                      deploys production), and delete the release branch
//
// Run it from a static linux/amd64 build (CGO_ENABLED=0 go build …/cmd/charter): publish hands Workers Builds that
// very binary. GitHub sets GITHUB_TOKEN (contents and pull-requests: write), GITHUB_REPOSITORY, GITHUB_SHA,
// GITHUB_REF_NAME and GITHUB_REF_TYPE.
package ci

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/joeblew999/charter/go/lib/internal/cli/self"
	"github.com/joeblew999/charter/go/lib/internal/previewbuild"
	"github.com/joeblew999/charter/go/lib/project"
)

// Main runs the step with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter ci", flag.ExitOnError)
	hook := fs.String("hook", "", "release: a shell command run after the commit is checked and tagged, before production moves (e.g. tagging the repo's own Go modules)")
	fs.Parse(args)
	log.SetFlags(0)
	p, err := project.Load(".")
	if err != nil {
		log.Fatal(err)
	}
	gh := github{repo: os.Getenv("GITHUB_REPOSITORY"), token: os.Getenv("GITHUB_TOKEN")}
	sha, ref, refType := os.Getenv("GITHUB_SHA"), os.Getenv("GITHUB_REF_NAME"), os.Getenv("GITHUB_REF_TYPE")
	if gh.repo == "" || gh.token == "" || sha == "" {
		log.Fatal("✗ charter ci needs GITHUB_REPOSITORY, GITHUB_TOKEN and GITHUB_SHA (GitHub Actions sets them)")
	}
	switch fs.Arg(0) {
	case "publish":
		os.Exit(publish(p, gh.repo, sha, ref, refType))
	case "release":
		if fs.NArg() != 2 {
			log.Fatal("usage: go tool charter ci release [-hook command] <version>")
		}
		if err := release(gh, fs.Arg(1), sha, ref, refType, *hook); err != nil {
			log.Fatal("✗ ", err)
		}
	default:
		log.Fatal("usage: go tool charter ci publish | release [-hook command] <version>")
	}
}

func publish(p *project.Project, repo, sha, ref, refType string) int {
	args := []string{"deploy", "-name", p.Name, "-repo", repo, "-publish-build", sha}
	if p.Assets != "" {
		args = append(args, "-assets", p.Assets)
	}
	if _, err := self.Output(args...); err != nil {
		return self.ExitCode(err)
	}
	if refType == "branch" && project.BranchEnvironment(ref) == "preview" && p.Subdomain != "" {
		note := ""
		if p.Preview != nil {
			note = p.Preview.Note
		}
		url := p.PreviewURL(previewbuild.Slug(ref, p.Name))
		if _, err := self.Output("deploy", "-repo", repo, "-comment-pr", ref, "-url", url, "-comment-note", note); err != nil {
			log.Printf("· the pull request comment failed (%v); the preview still deploys", err)
		}
	}
	return 0
}

// release checks the commit is on main, tags it (for a release branch), runs hook, moves production to it and
// deletes the release branch.
func release(gh github, version, sha, ref, refType, hook string) error {
	if !strings.HasPrefix(version, "v") {
		return fmt.Errorf("%s: a release is v<something>", version)
	}
	var compare struct {
		Status string `json:"status"`
	}
	if err := gh.do(http.MethodGet, "/compare/"+sha+"..."+project.StagingBranch, nil, &compare); err != nil {
		return err
	}
	if compare.Status != "identical" && compare.Status != "ahead" {
		return fmt.Errorf("%s (%.7s) isn't on %s (%s): not released", version, sha, project.StagingBranch, compare.Status)
	}
	if refType == "branch" {
		if err := gh.do(http.MethodPost, "/git/refs", map[string]string{"ref": "refs/tags/" + version, "sha": sha}, nil); err != nil {
			return fmt.Errorf("tag %s (it may exist already): %w", version, err)
		}
		log.Printf("✓ tag %s → %.7s", version, sha)
	}
	if hook != "" {
		cmd := exec.Command("sh", "-c", hook)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("-hook: %w", err)
		}
	}
	if err := pushProduction(gh, sha); err != nil {
		return err
	}
	log.Printf("✓ %s: %s → %.7s; Workers Builds deploys it", version, project.ProductionBranch, sha)
	if refType == "branch" {
		if err := gh.do(http.MethodDelete, "/git/refs/heads/"+ref, nil, nil); err != nil {
			log.Printf("· couldn't delete %s: %v", ref, err)
		}
	}
	return nil
}

// pushProduction force-pushes sha to the production branch: the one write a release makes that the API can't (moving
// a branch backwards for a roll-back is a force push).
var pushProduction = func(gh github, sha string) error {
	cmd := exec.Command("git", "push", "--force", "https://x-access-token:"+gh.token+"@github.com/"+gh.repo+".git",
		sha+":refs/heads/"+project.ProductionBranch)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("moving %s to %.7s: %w", project.ProductionBranch, sha, err)
	}
	return nil
}

// github is the repository's REST API.
type github struct{ repo, token, api string }

func (g github) do(method, path string, body, out any) error {
	api := g.api
	if api == "" {
		api = "https://api.github.com"
	}
	var b []byte
	if body != nil {
		var err error
		if b, err = json.Marshal(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, api+"/repos/"+g.repo+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := (&http.Client{Timeout: time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("GitHub %s %s: HTTP %d", method, path, res.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}
