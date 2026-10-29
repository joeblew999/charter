package projectsteps

import (
	"flag"

	"github.com/joeblew999/charter/internal/run"
)

// Dev runs charter dev: the Worker built and on local workerd, on charter.toml's port (PORT overrides it), its D1
// migrated on first use, until Ctrl-C; with -native, the app run natively instead (go run .), with [local] vars.
func Dev(args []string) {
	fs := flag.NewFlagSet("charter dev", flag.ExitOnError)
	native := fs.Bool("native", false, "run the app natively (go run .), not the Worker on workerd")
	fs.Parse(args)
	p := load()
	if *native {
		env := localEnv(p, places{})
		env["PORT"] = p.LocalPort()
		check(run.Run(p.Root, env, "go", "run", "."))
		return
	}
	check(run.Run(p.Root, nil, self(), "build"))
	check(run.Run(p.Root, nil, self(), "serve"))
}
