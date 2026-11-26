package command

import (
	"errors"
	"flag"
	"github.com/joeblew999/charter/internal/fern"
)

// SDK runs charter pack: an SDK with Fern (Docker) into sdk/out, by default the Go one; -prove: every client,
// proven in its language; -publish: the Go SDK into sdk/go, which the project commits.
func SDK(args []string) {
	fs := flag.NewFlagSet("charter pack", flag.ExitOnError)
	prove := fs.Bool("prove", false, "generate every client (or those named) and prove each in its language")
	publish := fs.Bool("publish", false, "the Go SDK into sdk/go, proven ([pack] go = true)")
	ifStale := fs.Bool("if-stale", false, "only when fern/ is not what the SDK was made from")
	fs.Parse(flagsFirst(args))
	p := load()
	switch {
	case *publish:
		if !p.SDKs.Go {
			check(errors.New("sdk -publish needs [pack] go = true in charter.toml: a project that commits its Go SDK"))
		}
		check(fern.Publish(p.Root, false))
	case *prove:
		check(fern.Prove(p.Root, fs.Args(), p.SDKs.Go))
	default:
		group := "go"
		if fs.NArg() > 0 {
			group = fs.Arg(0)
		}
		_, err := fern.Generate(p.Root, group, *ifStale, false)
		check(err)
	}
}
