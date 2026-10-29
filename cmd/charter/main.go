// Command charter is a Worker's every step on Cloudflare, one command for all of them (#100, #121):
//
//	charter build    the Worker into build/: a Go one with TinyGo (go doc .../internal/cli/build), a TypeScript one
//	                 bundled with esbuild ([build] entry)
//	charter deploy   to Cloudflare over its REST API: production, Worker Previews, the preview pipeline
//	charter tail     the deployed Worker's live logs
//	charter bench    the CPU a local Worker spends per request
//	charter local    write .charter/ from charter.toml: what runs the Worker on plain workerd
//	charter serve    run it locally, in the developer's time zone
//	charter with     run a command against it on a free port (BASE its URL), then stop it
//	charter token    whether CLOUDFLARE_API_TOKEN is active
//	charter ci       CI's side of previews and releases: publish a commit's build, release a version
//	charter builds   Workers Builds' side: fetch a commit's build, deploy what its branch deploys
//	charter init     set an app up: charter.toml, .gitignore, the mise tasks, the skill, the CI workflow
//	charter tasks    the mise tasks and the Workers Builds launcher again (after upgrading charter)
//	charter skill    the agent skill again
//	charter workflow the CI workflow again
//	charter check    every problem with charter.toml and charter's files in the app; in a project, every check that
//	                 needs no Cloudflare account (go doc .../internal/cli/projectsteps)
//	charter spec     a project's specs from its contract, Fern's settings, its database's code
//	charter sdk      a project's SDKs with Fern: one, every one proven (-prove), the Go SDK it commits (-publish)
//	charter dev      a project's Worker on local workerd (-native: the app run natively), until Ctrl-C
//	charter live-test  the standard test and a project's live tests against the deployed Worker
//	charter twins    two projects of the same API (a TypeScript one and its Go twin) answer the same
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
	"test":      projectsteps.Test, // what check runs under charter with: no step to run alone
	"version":   func([]string) { fmt.Println("charter", version.String()) },
}

func main() {
	if len(os.Args) < 2 || steps[os.Args[1]] == nil {
		fmt.Fprintln(os.Stderr, "usage: charter build | deploy | tail | bench | local | serve | with | token | ci | builds | init | tasks | skill | workflow | check | spec | sdk | dev | live-test | twins | version [flags]   (charter <step> -h: its flags)")
		os.Exit(2)
	}
	steps[os.Args[1]](os.Args[2:])
}
