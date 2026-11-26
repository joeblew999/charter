package command

import (
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/command/seo"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/standard"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Test runs charter test: the app's Go tests, then the Worker itself, built and run on wrangler dev on a port and
// with state of its own: the standard test of an API with a contract, charter.toml's [test] run, and for a site
// ([seo]) the audit a search engine would pass or fail it on.
//
//	charter test                  all of that
//	charter test e2e              one of charter.toml's [test.suites], against the Worker on wrangler dev
//	charter test --env preview    the standard test and [test] run against that environment, deployed ([urls])
//	charter test --base <url>     the same against any address
//	charter test e2e --base <url> a suite against a deployed Worker
func Test(args []string) {
	suite := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		suite, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("charter test", flag.ExitOnError)
	base := fs.String("base", "", "the address of a Worker already running or deployed, instead of wrangler dev")
	env := fs.String("env", "", "an environment of "+project.Wrangler+", deployed: its address is charter.toml's [urls]")
	fs.Parse(args)
	p := load()
	if *env != "" {
		if *base != "" {
			check(errors.New("--env and --base both name the Worker to test: give one"))
		}
		if _, ok := p.Worker.Env[*env]; !ok {
			check(fmt.Errorf("%s has no environment %q", project.Wrangler, *env))
		}
		if *base = p.URLs[*env]; *base == "" {
			check(fmt.Errorf("charter.toml has no address for %s: under [urls], %s = \"https://…\" (go tool charter deploy %s prints it)", *env, *env, *env))
		}
	}
	*base = strings.TrimRight(*base, "/")
	if suite != "" {
		commands, ok := p.Test.Suites[suite]
		if !ok {
			check(fmt.Errorf("charter.toml's [test.suites] has no %s", suite))
		}
		only := &project.Project{Root: p.Root} // a suite is its own commands, without the standard test
		if *base != "" {
			say("testing %s", *base)
			check(try(only, commands, *base))
			return
		}
		check(serving(p, func(base string) error { return try(only, commands, base) }))
		return
	}
	if *base != "" {
		// A deployed Worker is asked what the local one is: the standard test and the app's own. Not go test (it
		// asks no Worker), and not the search-engine audit, which is of the site at url.
		commands := p.Test.Live
		if len(commands) == 0 {
			commands = p.Test.Run
		}
		t := newTimes("test of " + *base)
		t.part("worker", func() error { return try(p, commands, *base) })
		t.done()
		return
	}
	t := newTimes("test")
	t.part("go test", func() error {
		if err := generate(p); err != nil {
			return err
		}
		return run.Run(p.Root, nil, "go", "test", "./...")
	})
	// wrangler dev builds the Worker itself (wrangler.jsonc's build command).
	t.part("worker", func() error {
		return serving(p, func(base string) error {
			if err := try(p, p.Test.Run, base); err != nil {
				return err
			}
			if p.SEO != nil {
				seo.Main([]string{"audit", "--site", base})
			}
			return nil
		})
	})
	t.done()
}

// try runs the standard test against the Worker at base, for an API with a contract, then each command with BASE and
// {base}; it fails at the first that fails.
func try(p *project.Project, commands []string, base string) error {
	if p.Spec != "" {
		if err := standard.Run(base, func(line string) { say("%s", line) }); err != nil {
			return err
		}
	}
	for _, line := range commands {
		words := strings.Fields(strings.ReplaceAll(line, "{base}", base))
		if len(words) == 0 {
			return errors.New("an empty command in charter.toml's [test]")
		}
		if words[0] == "charter" {
			words[0] = self()
		}
		if err := run.Run(p.Root, map[string]string{"BASE": base, "TARGET": target(base)}, words[0], words[1:]...); err != nil {
			return err
		}
	}
	return nil
}

// target says what the Worker at base is, for a test that asks differently of the two: "local" (wrangler dev on this
// machine, where a Cron Trigger can be fired by hand and an email handed over) or "deployed". It is TARGET in a test
// command's environment, beside BASE.
func target(base string) string {
	if u, err := url.Parse(base); err == nil {
		if host := u.Hostname(); host == "localhost" || net.ParseIP(host).IsLoopback() {
			return "local"
		}
	}
	return "deployed"
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
		Command: []string{bin, "dev", "--port", strconv.Itoa(ports[0]), "--inspector-port", strconv.Itoa(ports[1]), "--persist-to", state, "--show-interactive-dev-session=false", "--test-scheduled"},
		Dir:     p.Root, Env: quiet, Ready: base + p.HealthPath(),
	}
	return run.With(server, func() error { return body(base) })
}
