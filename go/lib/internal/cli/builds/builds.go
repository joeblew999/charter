// Package builds is go tool charter builds build|deploy: what Workers Builds (Cloudflare's GitHub app) runs for each
// pushed branch, from the charter binary CI published for that commit (go tool charter ci publish), so nothing is
// compiled at Cloudflare.
//
//	go tool charter builds build    fetch this commit's build (build/, the Static Assets), verified against its
//	                                manifest; exit 3 when CI never published one (then build here: go tool charter build)
//	go tool charter builds deploy   what the branch deploys (production, staging, or its preview, charter.toml's),
//	                                then delete the previews of branches that are gone
//
// Workers Builds sets WORKERS_CI_BRANCH and WORKERS_CI_COMMIT_SHA, and its build token's CLOUDFLARE_API_TOKEN;
// CLOUDFLARE_ACCOUNT_ID is a build variable (go tool charter deploy -configure-builds).
package builds

import (
	"flag"
	"log"
	"os"

	"github.com/joeblew999/charter/go/lib/internal/cli/self"
	"github.com/joeblew999/charter/go/lib/internal/previewbuild"
	"github.com/joeblew999/charter/go/lib/project"
)

// Main runs the step with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter builds", flag.ExitOnError)
	wait := fs.String("wait", "2m", "build: how long to wait for CI's build of this commit")
	fs.Parse(args)
	log.SetFlags(0)
	p, err := project.Load(".")
	if err != nil {
		log.Fatal(err)
	}
	branch, sha := os.Getenv("WORKERS_CI_BRANCH"), os.Getenv("WORKERS_CI_COMMIT_SHA")
	switch fs.Arg(0) {
	case "build":
		if sha == "" || p.Repo == "" {
			log.Fatal("✗ builds build needs WORKERS_CI_COMMIT_SHA and charter.toml's repo")
		}
		_, err := self.Output("deploy", "-repo", p.Repo, "-fetch-build", sha, "-wait", *wait)
		os.Exit(self.ExitCode(err))
	case "deploy":
		if branch == "" {
			log.Fatal("✗ builds deploy needs WORKERS_CI_BRANCH (Workers Builds sets it)")
		}
		os.Exit(deploy(p, branch, sha))
	default:
		log.Fatal("usage: go tool charter builds build|deploy")
	}
}

func deploy(p *project.Project, branch, sha string) int {
	env := project.BranchEnvironment(branch)
	if env == "" {
		log.Printf("· %s is a release request: CI releases it (go tool charter ci release)", branch)
		return 0
	}
	args := []string{"deploy", env}
	if env == "preview" {
		args = append(args, previewbuild.Slug(branch, p.Name))
		if sha != "" {
			args = append(args, "-annotation", "workers/commit_sha="+sha)
		}
		if p.Repo != "" {
			args = append(args, "-annotation", "workers/repository_url=https://github.com/"+p.Repo)
		}
	}
	url, err := self.Output(args...)
	if err != nil {
		log.Printf("✗ %s: %v", env, err)
		return self.ExitCode(err)
	}
	log.Printf("✓ %s for branch %s: %s", env, branch, url)
	if env != "production" && p.Repo != "" {
		if _, err := self.Output("deploy", "-name", p.Name, "-repo", p.Repo, "-prune-previews"); err != nil {
			log.Printf("· pruning previews failed (%v); the %s is live", err, env)
		}
	}
	return 0
}
