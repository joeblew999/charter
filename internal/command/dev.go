package command

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
)

// Dev runs charter dev: the loop a developer works in. With gsx (a gsx.toml) it is gsx's own, go tool gsx dev: a save
// regenerates, rebuilds and reloads the browser, through Vite. Without, wrangler dev.
func Dev(args []string) {
	p := load()
	if hasGSX(p) {
		check(passthrough(p, "go", append([]string{"tool", "gsx", "dev"}, args...)...))
		return
	}
	Preview(args)
}

// Preview runs charter preview: the Worker as Cloudflare runs it, on wrangler dev, with the local D1's migrations
// applied first. More arguments are wrangler dev's.
func Preview(args []string) {
	p := load()
	check(migrate(p, nil, "--local"))
	bin, err := npm(p, "wrangler")
	check(err)
	check(passthrough(p, bin, append([]string{"dev"}, args...)...))
}

// Deploy runs charter deploy [environment]: the D1 migrations, then wrangler deploy. An environment is one of
// wrangler.jsonc's env (production, staging). More arguments are wrangler deploy's. Credentials are wrangler's:
// CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID in the environment, or wrangler login.
func Deploy(args []string) {
	p := load()
	var env []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		if _, ok := p.Worker.Env[args[0]]; !ok {
			check(fmt.Errorf("%s has no environment %q", project.Wrangler, args[0]))
		}
		env, args = []string{"--env", args[0]}, args[1:]
	}
	check(migrate(p, nil, append([]string{"--remote"}, env...)...))
	check(wrangler(p, nil, append(append([]string{"deploy"}, env...), args...)...))
}

// migrate applies every D1 database's migrations with wrangler (its own record of which ran).
func migrate(p *project.Project, env map[string]string, where ...string) error {
	for _, db := range p.Worker.D1 {
		if err := wrangler(p, with(env, "CI", "1"), append([]string{"d1", "migrations", "apply", db.Binding}, where...)...); err != nil {
			return err
		}
	}
	return nil
}

func with(env map[string]string, key, value string) map[string]string {
	out := map[string]string{key: value}
	for k, v := range env {
		out[k] = v
	}
	return out
}

// passthrough runs a program with the terminal, until it ends.
func passthrough(p *project.Project, program string, args ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = p.Root, os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// With runs charter with <command>: the Worker on wrangler dev, on a free port with local state of its own, for as
// long as the command runs. BASE in the command's environment is the Worker's URL.
func With(args []string) {
	fs := flag.NewFlagSet("charter with", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatal("usage: charter with <command> [args…]")
	}
	p := load()
	check(serving(p, func(base string) error {
		return run.Run(p.Root, map[string]string{"BASE": base}, fs.Arg(0), fs.Args()[1:]...)
	}))
}

// serving runs body with the Worker on wrangler dev at base: a free port, and state in a folder of its own that goes
// when it ends, so a run starts from nothing and two runs never share a database.
func serving(p *project.Project, body func(base string) error) error {
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	ports, err := run.FreePorts(2)
	if err != nil {
		return err
	}
	state, err := os.MkdirTemp("", "charter-state-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(state)
	quiet := map[string]string{"WRANGLER_SEND_METRICS": "false", "CI": "1"}
	if err := migrate(p, quiet, "--local", "--persist-to", state); err != nil {
		return err
	}
	base := "http://127.0.0.1:" + strconv.Itoa(ports[0])
	server := run.Server{
		Command: []string{bin, "dev", "--port", strconv.Itoa(ports[0]), "--inspector-port", strconv.Itoa(ports[1]), "--persist-to", state, "--show-interactive-dev-session=false"},
		Dir:     p.Root, Env: quiet, Ready: base + p.HealthPath(),
	}
	return run.With(server, func() error { return body(base) })
}
