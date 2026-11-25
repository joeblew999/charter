package command

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	intlconfig "github.com/joeblew999/charter/intl/config"
)

// Dev runs charter dev: the loop a developer works in. With gsx (a gsx.toml) it is gsx's own, go tool gsx dev: a save
// regenerates, rebuilds and reloads the browser, through Vite. Without, wrangler dev.
//
// Everything else a developer runs meanwhile, in another terminal, reaches the open page by itself: gsx dev watches
// the Go and gsx files (charter add, gsxui add), Vite the browser's files, and charter the languages: a change to a
// catalog or to charter.toml (charter intl add-locale, charter intl translate, an edit by hand) compiles the
// messages again, which gsx dev then picks up.
func Dev(args []string) {
	p := load()
	if hasGSX(p) {
		if hasIntl(p) {
			check(generate(p)) // the first build needs the messages and Vite's manifest
			go watchLanguages(p)
		}
		check(passthrough(p, "go", append([]string{"tool", "gsx", "dev"}, args...)...))
		return
	}
	Preview(args)
}

// watchLanguages compiles the messages again whenever a catalog or charter.toml changes, for as long as dev runs. It
// looks twice a second: a handful of files, on every system, with no watcher to set up.
func watchLanguages(p *project.Project) {
	c, err := intlconfig.Find(p.Root)
	if err != nil {
		return
	}
	stamp := func() string {
		files, _ := filepath.Glob(filepath.Join(c.CatalogDir(), "*.toml"))
		files = append(files, filepath.Join(p.Root, project.File))
		var b strings.Builder
		for _, f := range files {
			if info, err := os.Stat(f); err == nil {
				fmt.Fprintf(&b, "%s %d %d\n", f, info.Size(), info.ModTime().UnixNano())
			}
		}
		return b.String()
	}
	last := stamp()
	for range time.Tick(500 * time.Millisecond) {
		now := stamp()
		if now == last {
			continue
		}
		last = now
		// As its own program: a catalog half-written by an editor or a translator fails this run, not dev.
		cmd := exec.Command(self(), "intl", "gen")
		cmd.Dir = p.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("charter dev: the languages did not compile (fix the catalog and save again):\n%s", out)
			continue
		}
		fmt.Println("charter dev: languages compiled")
	}
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
