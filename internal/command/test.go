package command

import (
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/internal/command/seo"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/standard"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Test runs charter test: the app's Go tests, then the Worker itself, built and run on wrangler dev on a port and
// with state of its own: the standard test of an API with a contract, charter.toml's [test] run, and for a site
// ([seo]) the audit a search engine would pass or fail it on.
//
//	charter test                  all of that
//	charter test e2e              one of charter.toml's [test.suites], against the Worker on wrangler dev
//	charter test --deployed       the standard test and [test] run against the deployed Worker (charter.toml's url)
//	charter test --env staging    the same against that environment of wrangler.jsonc ([urls])
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
	deployed := fs.Bool("deployed", false, "the deployed Worker, at charter.toml's url")
	fs.Parse(args)
	p := load()
	if *deployed {
		if *base != "" || *env != "" {
			check(errors.New("--deployed, --env and --base each name the Worker to test: give one"))
		}
		if *base = p.URL; *base == "" {
			check(errors.New(`charter.toml has no url, where the Worker is deployed: url = "https://…"`))
		}
	}
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
		// A deployed Worker is asked what the local one is: the standard test, the app's own, and the search-engine
		// audit, which a deployed site is what a search engine sees of. Not go test (it asks no Worker).
		commands := p.Test.Live
		if len(commands) == 0 {
			commands = p.Test.Run
			if p.Test.Deployed {
				commands = append(slices.Clone(commands), "go test -count=1 -timeout 20m ./test/deployed/...")
			}
		}
		t := newTimes("test of " + *base)
		began := time.Now()
		t.part("worker", func() error { return try(p, commands, *base) })
		if p.SEO != nil {
			t.part("audit", func() error { return seo.Audit(strings.TrimRight(*base, "/")) })
		}
		// And what no test sees: whether Cloudflare ended any request of the run instead of the Worker answering it
		// (over its memory, over its CPU time, an exception). Only for a Worker this app deploys.
		if deployedHere(p, *base) {
			t.part("health", func() error { return healthOf(p, envOf(p, *base), began) })
		}
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
			commands := p.Test.Run
			if p.Test.Local {
				commands = append(slices.Clone(commands), "go test -count=1 ./test/dev/...")
			}
			if err := try(p, commands, base); err != nil {
				return err
			}
			if p.SEO != nil {
				return seo.Audit(base)
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
	env := map[string]string{"BASE": base, "TARGET": target(base)}
	if target(base) == "deployed" {
		// The deployed Worker's secrets, which a test needs to be who may do what (apptest.Secret): from fnox.
		for name, value := range deployedSecrets(p, envOf(p, base)) {
			env[name] = value
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
		if err := run.Run(p.Root, env, words[0], words[1:]...); err != nil {
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
	return servingOn(p, "", func(base string, _ int) error { return body(base) })
}

// servingOn is serving on an address of the machine other than its own name: "0.0.0.0", for what asks from a
// container. body gets the port too.
func servingOn(p *project.Project, ip string, body func(base string, port int) error) error {
	return servingWith(p, served{ip: ip}, body)
}

// served is how a Worker is started for something to ask it.
type served struct {
	ip    string            // another address of the machine than its name
	env   map[string]string // more of the environment wrangler dev and its build get
	local bool              // on the app's own local state (its database as charter dev has it), not a new one
	// config is another file than wrangler.jsonc for wrangler dev to read, beside it; cloudflare says the Worker
	// talks to Cloudflare (a binding of that file is the deployed one), so wrangler gets the credentials.
	config     string
	cloudflare bool
}

func servingWith(p *project.Project, how served, body func(base string, port int) error) error {
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	ports, err := run.FreePorts(2)
	if err != nil {
		return err
	}
	quiet := map[string]string{"WRANGLER_SEND_METRICS": "false", "CI": "1"}
	for k, v := range how.env {
		quiet[k] = v
	}
	// By name, not by number: a passkey belongs to a host name, and a page with sign-in is tested at one.
	base := "http://localhost:" + strconv.Itoa(ports[0])
	command := []string{bin, "dev", "--port", strconv.Itoa(ports[0]), "--inspector-port", strconv.Itoa(ports[1]), "--show-interactive-dev-session=false", "--test-scheduled"}
	if how.config != "" {
		command = append(command, "--config", how.config)
	}
	// What talks to Cloudflare (a binding that is the deployed one) has its credentials, as charter deploy does.
	if how.cloudflare && os.Getenv("CLOUDFLARE_API_TOKEN") == "" {
		fnox, err := credentials(p)
		if err != nil {
			return err
		}
		command = append([]string{fnox, "exec", "--"}, command...)
	}
	if !how.local {
		// A database of its own, made new and thrown away: what is asked changes nothing a developer has.
		state, err := os.MkdirTemp("", "charter-state-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(state)
		if err := migrate(p, quiet, "--local", "--persist-to", state); err != nil {
			return err
		}
		command = append(command, "--persist-to", state)
		// And filled by the app's seed, so what is asked can count on what the seed names: the same people and
		// notes a developer's own database starts with.
		if exists(filepath.Join(p.Root, "seed.go")) {
			quiet[seedBuild] = "1"
			ask := body
			body = func(base string, port int) error {
				res, err := http.Post(base+app.SeedPath, "text/plain", nil)
				if err != nil {
					return err
				}
				said, _ := io.ReadAll(res.Body)
				res.Body.Close()
				if res.StatusCode != http.StatusOK {
					return fmt.Errorf("seed.go: %s", strings.TrimSpace(string(said)))
				}
				return ask(base, port)
			}
		}
	}
	if how.ip != "" {
		command = append(command, "--ip", how.ip)
	}
	server := run.Server{Command: command, Dir: p.Root, Env: quiet, Ready: base + p.HealthPath()}
	return run.With(server, func() error { return body(base, ports[0]) })
}

// deployedHere reports whether base is where this app's Worker, or an environment of it, is deployed.
func deployedHere(p *project.Project, base string) bool {
	base = strings.TrimRight(base, "/")
	if base == strings.TrimRight(p.URL, "/") && p.URL != "" {
		return true
	}
	for _, at := range p.URLs {
		if strings.TrimRight(at, "/") == base {
			return true
		}
	}
	return false
}
