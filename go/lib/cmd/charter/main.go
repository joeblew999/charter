// Command charter is a Go Worker's every step on Cloudflare without Node, one command for all of them (#100):
//
//	go tool charter build    the Worker into build/, with TinyGo (go doc .../internal/cli/build)
//	go tool charter deploy   to Cloudflare over its REST API: production, Worker Previews, the preview pipeline
//	go tool charter tail     the deployed Worker's live logs
//	go tool charter bench    the CPU a local Worker spends per request
//	go tool charter local    write .charter/ from charter.toml: what runs the Worker on plain workerd
//	go tool charter serve    run it locally, in the developer's time zone
//	go tool charter with     run a command against it on a free port (BASE its URL), then stop it
//	go tool charter token    whether CLOUDFLARE_API_TOKEN is active
//	go tool charter ci       CI's side of previews and releases: publish a commit's build, release a version
//	go tool charter builds   Workers Builds' side: fetch a commit's build, deploy what its branch deploys
//
// go.mod: tool github.com/joeblew999/charter/go/lib/cmd/charter. `go tool charter <step> -h` lists a step's flags.
// go tool workerbuild, workerdeploy, workertail and workerbench are the same steps by their old names.
package main

import (
	"fmt"
	"os"

	"github.com/joeblew999/charter/go/lib/internal/cli/bench"
	"github.com/joeblew999/charter/go/lib/internal/cli/build"
	"github.com/joeblew999/charter/go/lib/internal/cli/builds"
	"github.com/joeblew999/charter/go/lib/internal/cli/ci"
	"github.com/joeblew999/charter/go/lib/internal/cli/deploy"
	"github.com/joeblew999/charter/go/lib/internal/cli/local"
	"github.com/joeblew999/charter/go/lib/internal/cli/tail"
	"github.com/joeblew999/charter/go/lib/internal/cli/token"
)

var steps = map[string]func(args []string){
	"build":  build.Main,
	"deploy": deploy.Main,
	"tail":   tail.Main,
	"bench":  bench.Main,
	"local":  local.Local,
	"serve":  local.Serve,
	"with":   local.With,
	"token":  token.Main,
	"ci":     ci.Main,
	"builds": builds.Main,
}

func main() {
	if len(os.Args) < 2 || steps[os.Args[1]] == nil {
		fmt.Fprintln(os.Stderr, "usage: go tool charter build | deploy | tail | bench | local | serve | with | token | ci | builds [flags]   (go tool charter <step> -h: its flags)")
		os.Exit(2)
	}
	steps[os.Args[1]](os.Args[2:])
}
