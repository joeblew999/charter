package command

import (
	"errors"
	"flag"
	"github.com/joeblew999/charter/internal/fern"
	"github.com/joeblew999/charter/internal/project"
	"os"
	"path/filepath"
	"strings"
)

// SDK runs charter pack: an SDK with Fern (Docker) into sdk/out, by default the Go one; -prove: every client,
// proven in its language; -publish: the Go SDK into sdk/go, which the project commits.
func SDK(args []string) {
	fs := flag.NewFlagSet("charter pack", flag.ExitOnError)
	prove := fs.Bool("prove", false, "generate every client (or those named) and prove each in its language")
	publish := fs.Bool("publish", false, "the Go SDK into sdk/go, proven ([pack] go = true)")
	ifStale := fs.Bool("if-stale", false, "only when fern/ is not what the SDK was made from")
	live := fs.Bool("live", false, "only run the clients already generated (sdk/out) against a Worker started here: the programs of test/sdk")
	fs.Parse(flagsFirst(args))
	p := load()
	switch {
	case *publish:
		if !p.SDKs.Go {
			check(errors.New("sdk -publish needs [pack] go = true in charter.toml: a project that commits its Go SDK"))
		}
		check(fern.Publish(p.Root, false))
	case *prove, *live:
		if *prove {
			check(fern.Prove(p.Root, fs.Args(), p.SDKs.Go))
		}
		// Then each client asks the Worker itself, running here, what the contract says it may and may not: the
		// programs of test/sdk, one a language.
		if exists(filepath.Join(p.Root, "test", "sdk")) {
			localVars(p)
			check(servingOn(p, "0.0.0.0", func(_ string, port int) error {
				return fern.Live(p.Root, port, devVars(p, "READ_TOKEN", "WRITE_TOKEN"))
			}))
		}
	default:
		group := "go"
		if fs.NArg() > 0 {
			group = fs.Arg(0)
		}
		_, err := fern.Generate(p.Root, group, *ifStale, false)
		check(err)
	}
}

// devVars are the values of these names in the app's .dev.vars: the local Worker's own secrets, which a proof of a
// client sends it.
func devVars(p *project.Project, names ...string) map[string]string {
	out := map[string]string{}
	b, _ := os.ReadFile(filepath.Join(p.Root, project.DevVars))
	for _, line := range strings.Split(string(b), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		for _, want := range names {
			if ok && strings.TrimSpace(name) == want {
				out[want] = strings.Trim(strings.TrimSpace(value), `"'`)
			}
		}
	}
	return out
}
