package projectsteps

import (
	"errors"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/standard"
	"github.com/joeblew999/charter/project"
)

// portsVar is how check hands the free ports it chose for what the tests serve ({port1}…{port3}) to charter test,
// which charter with runs.
const portsVar = "CHARTER_TEST_PORTS"

// Test runs charter test run: the standard test, then [test] run, against the Worker at $BASE (as charter with runs
// it). charter check runs it; it is no step a developer runs alone.
func Test(args []string) {
	fs := flag.NewFlagSet("charter test", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 1 || fs.Arg(0) != "run" {
		check(errors.New("usage: charter test run   (charter check runs it, under charter with, BASE the Worker's URL)"))
	}
	p := load()
	base := os.Getenv("BASE")
	if base == "" {
		check(errors.New("charter test: no BASE, the Worker's URL (charter with sets it)"))
	}
	var ports []int
	for _, s := range strings.Split(os.Getenv(portsVar), ",") {
		if n, err := strconv.Atoi(s); err == nil {
			ports = append(ports, n)
		}
	}
	pl := places{base, ports}
	check(try(p, p.Test.Run, pl, localEnv(p, pl)))
}

// try runs the standard test against the Worker at pl.base, then each command, in the project's root with env; it
// fails at the first that fails.
func try(p *project.Project, commands []string, pl places, env map[string]string) error {
	if err := standard.Run(pl.base, func(line string) { say("%s", line) }); err != nil {
		return err
	}
	for _, line := range commands {
		words, err := pl.command(line)
		if err != nil {
			return err
		}
		if err := run.Run(p.Root, env, words[0], words[1:]...); err != nil {
			return err
		}
	}
	return nil
}

// LiveTest runs charter live-test: the standard test, then [test] live, against the deployed Worker (url). Behind
// Cloudflare Access, its service token comes from the environment (fnox exec -- charter live-test).
func LiveTest(args []string) {
	fs := flag.NewFlagSet("charter live-test", flag.ExitOnError)
	fs.Parse(args)
	p := load()
	if p.URL == "" {
		check(errors.New("charter.toml has no url: where the Worker is deployed"))
	}
	if len(p.Test.Live) == 0 {
		check(errors.New("charter.toml has no [test] live: the tests against the deployed Worker"))
	}
	// The deployed Worker's tokens are the environment's (fnox), never [local]'s throwaway ones.
	check(try(p, p.Test.Live, places{base: p.URL}, nil))
}
