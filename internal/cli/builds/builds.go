// Package builds is go tool charter builds build|deploy|configure: what Workers Builds (Cloudflare's GitHub app) runs
// for each pushed branch, from the charter binary CI published for that commit (go tool charter ci publish), so
// nothing is compiled at Cloudflare. The launcher .mise/tasks/charter-builds (go tool charter tasks) fetches that
// binary and runs it.
//
//	go tool charter builds build      fetch this commit's build (build/, the Static Assets), verified against its
//	                                  manifest; exit 3 when CI never published one
//	go tool charter builds build -compile   build it here instead: charter.toml's [ci] tools and [build] command
//	go tool charter builds deploy     what the branch deploys (production, staging, or its preview, charter.toml's),
//	                                  then delete the previews of branches that are gone
//	go tool charter builds configure  set Workers Builds up for the Worker: the repository, the launcher as its build
//	                                  and deploy commands, every branch built (needs Cloudflare credentials)
//
// Workers Builds sets WORKERS_CI_BRANCH and WORKERS_CI_COMMIT_SHA, and its build token's CLOUDFLARE_API_TOKEN;
// CLOUDFLARE_ACCOUNT_ID is a build variable (go tool charter deploy -configure-builds).
package builds

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/joeblew999/charter/internal/cli/self"
	"github.com/joeblew999/charter/internal/previewbuild"
	"github.com/joeblew999/charter/project"
)

// Main runs the step with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter builds", flag.ExitOnError)
	wait := fs.String("wait", "2m", "build: how long to wait for CI's build of this commit")
	compile := fs.Bool("compile", false, "build: build here, with charter.toml's [ci] tools and [build] command")
	token := fs.String("build-token", "", `configure: the Workers Builds token's name, made once in the dashboard (default "<name> build token")`)
	fallback := fs.String("fallback-build-token", os.Getenv("FALLBACK_BUILD_TOKEN"), "configure: a token name to use until -build-token exists")
	fs.Parse(args)
	log.SetFlags(0)
	p, err := project.Load(".")
	if err != nil {
		log.Fatal(err)
	}
	branch, sha := os.Getenv("WORKERS_CI_BRANCH"), os.Getenv("WORKERS_CI_COMMIT_SHA")
	switch fs.Arg(0) {
	case "build":
		if *compile {
			os.Exit(compileHere(p))
		}
		if sha == "" || p.Repo == "" {
			log.Fatal("✗ builds build needs WORKERS_CI_COMMIT_SHA and charter.toml's repo")
		}
		_, err := self.Output("deploy", "-repo", p.Repo, "-fetch-build", sha, "-wait", *wait)
		os.Exit(self.ExitCode(err))
	case "configure":
		if p.Repo == "" {
			log.Fatal("✗ builds configure needs charter.toml's repo")
		}
		if *token == "" {
			*token = p.Name + " build token"
		}
		_, err := self.Output("deploy", "-name", p.Name, "-repo", p.Repo, "-configure-builds",
			"-build-command", "sh .mise/tasks/charter-builds build", "-deploy-command", "sh .mise/tasks/charter-builds deploy",
			"-build-token", *token, "-fallback-build-token", *fallback,
			"-build-var", "CLOUDFLARE_ACCOUNT_ID="+os.Getenv("CLOUDFLARE_ACCOUNT_ID"), "-path-exclude", "*.md")
		os.Exit(self.ExitCode(err))
	case "deploy":
		if branch == "" {
			log.Fatal("✗ builds deploy needs WORKERS_CI_BRANCH (Workers Builds sets it)")
		}
		os.Exit(deploy(p, branch, sha))
	default:
		log.Fatal("usage: go tool charter builds build [-compile] | deploy | configure")
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

// compileHere builds the Worker where it runs (Workers Builds, when CI published nothing): the tools CI installs, then
// the app's build command.
func compileHere(p *project.Project) int {
	install := "mise install"
	if len(p.CI.Tools) > 0 {
		install += " " + strings.Join(p.CI.Tools, " ")
	}
	for _, script := range []string{install, p.BuildCommand()} {
		cmd := exec.Command("sh", "-c", script)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("✗ %s: %v", script, err)
			return 1
		}
	}
	return 0
}
