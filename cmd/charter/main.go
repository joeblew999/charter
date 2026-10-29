// Command charter is a Worker's every step on Cloudflare, and how the repo that holds it is kept, one command for all
// of them (#100, #121). charter help lists every command; docs/charter/README.md says when each is run.
//
// A project's steps (a folder with a charter.toml naming a Worker):
//
//	charter build    the Worker into build/: a Go one with TinyGo (go doc .../internal/cli/build), a TypeScript one
//	                 bundled with esbuild ([build] entry)
//	charter dev      the Worker on local workerd (-native: the app run natively), until Ctrl-C
//	charter check    every check that needs no Cloudflare account (go doc .../internal/cli/projectsteps)
//	charter spec     the specs from its contract, Fern's settings, its database's code
//	charter sdk      its SDKs with Fern: one, every one proven (-prove), the Go SDK it commits (-publish)
//	charter deploy   to Cloudflare over its REST API: production, Worker Previews, the preview pipeline
//	charter live-test  the standard test and its live tests against the deployed Worker
//	charter local    write .charter/ from charter.toml: what runs the Worker on plain workerd
//	charter serve    run it locally, in the developer's time zone
//	charter with     run a command against it on a free port (BASE its URL), then stop it
//	charter tail     the deployed Worker's live logs
//	charter bench    the CPU a local Worker spends per request
//	charter token    whether CLOUDFLARE_API_TOKEN is active
//	charter builds   Workers Builds' side: fetch a commit's build, deploy what its branch deploys
//
// The repo's (go doc .../internal/cli/repo):
//
//	charter init     charter's files in the repo, its mise tasks and skill; an app: charter.toml first
//	charter issues   the open issues; with a kind, a new one's body and the gh command that files it
//	charter docs     what a program can see wrong in docs/; -review: Claude reads the rest
//	charter ci       a job of charter.toml's [ci.jobs], here or (-remote) on GitHub; an app's own [ci] (ci run,
//	                 publish, release)
//	charter upstream the upstream issues the code works around, and their state
//	charter repo     the repo on GitHub as charter.toml's [github] says
//	charter new      a new project, a copy of a tested example, in a repo of its own
//	charter twins    two projects of the same API (a TypeScript one and its Go twin) answer the same
//	charter release  a release of charter itself, from its own repo
//	charter tasks, skill, workflow   the mise tasks, the agent skill, an app's CI workflow, again
//	charter version  the version of charter this is
//
// A repo takes it from mise, as a binary of charter's GitHub release: mise use github:joeblew999/charter.
// `charter <step> -h` lists a step's flags.
package main

import (
	"fmt"
	"os"

	"github.com/joeblew999/charter/internal/cli/bench"
	"github.com/joeblew999/charter/internal/cli/build"
	"github.com/joeblew999/charter/internal/cli/builds"
	"github.com/joeblew999/charter/internal/cli/ci"
	"github.com/joeblew999/charter/internal/cli/deploy"
	"github.com/joeblew999/charter/internal/cli/local"
	"github.com/joeblew999/charter/internal/cli/projectsteps"
	"github.com/joeblew999/charter/internal/cli/repo"
	"github.com/joeblew999/charter/internal/cli/scaffold"
	"github.com/joeblew999/charter/internal/cli/tail"
	"github.com/joeblew999/charter/internal/cli/token"
	"github.com/joeblew999/charter/internal/twins"
	"github.com/joeblew999/charter/internal/version"
)

var steps = map[string]func(args []string){
	"build":     build.Main,
	"deploy":    deploy.Main,
	"tail":      tail.Main,
	"bench":     bench.Main,
	"local":     local.Local,
	"serve":     local.Serve,
	"with":      local.With,
	"token":     token.Main,
	"ci":        ci.Main,
	"builds":    builds.Main,
	"init":      scaffold.Init,
	"tasks":     scaffold.Tasks,
	"skill":     scaffold.Skill,
	"workflow":  scaffold.Workflow,
	"check":     projectsteps.Check,
	"spec":      projectsteps.Spec,
	"sdk":       projectsteps.SDK,
	"dev":       projectsteps.Dev,
	"live-test": projectsteps.LiveTest,
	"twins":     twins.Main,
	"issues":    repo.Issues,
	"upstream":  repo.Upstream,
	"repo":      repo.Repo,
	"docs":      repo.Docs,
	"new":       repo.New,
	"release":   repo.Release,
	"test":      projectsteps.Test, // what check runs under charter with: no step to run alone
	"version":   func([]string) { fmt.Println("charter", version.String()) },
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" {
		help()
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
	}
	if steps[os.Args[1]] == nil {
		fmt.Fprintf(os.Stderr, "charter has no command %s: charter help lists them\n", os.Args[1])
		os.Exit(2)
	}
	steps[os.Args[1]](os.Args[2:])
}

// help lists charter's commands, each with what it takes and what it does.
func help() {
	fmt.Printf("charter %s: contract-first APIs on Cloudflare Workers, in TypeScript or Go, and the repos that hold them.\n\n", version.String())
	for _, c := range repo.Commands {
		fmt.Printf("  charter %s\n      %s\n", c.Usage, c.About)
	}
	fmt.Printf("\nAnd: charter %s. charter <command> -h: its flags.\n", joinExtra())
}

func joinExtra() string {
	s := ""
	for i, name := range repo.Extra {
		if i > 0 {
			s += ", "
		}
		s += name
	}
	return s
}
