package command

import (
	"bytes"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Deploy runs charter deploy [environment]: the D1 migrations, then wrangler deploy. An environment is one of
// wrangler.jsonc's env (production, staging). More arguments are wrangler deploy's. Credentials are wrangler's
// (cloudflare, in verbs.go, says where they come from).
func Deploy(args []string) {
	// A deployed Worker never has the app's seed in it, whatever the environment this was started in says.
	os.Unsetenv(seedBuild)
	p := load()
	var env []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		if _, ok := p.Worker.Env[args[0]]; !ok {
			check(fmt.Errorf("%s has no environment %q", project.Wrangler, args[0]))
		}
		env, args = []string{"--env", args[0]}, args[1:]
	}
	migrations := func() {
		for _, db := range p.Worker.D1 {
			check(cloudflare(p, append([]string{"d1", "migrations", "apply", db.Binding, "--remote"}, env...), nil))
		}
	}
	// The migrations go first, so the new code never meets the old schema. Except on the very first deploy of a
	// Worker whose database wrangler provisions itself (a binding with no database_id): there is no database until
	// wrangler deploy has made it, so there the Worker goes first.
	first := len(env) == 0 && slices.ContainsFunc(p.Worker.D1, func(db project.D1) bool {
		return db.ID == "" && (db.Name == "" || cloudflareQuietly(p, "d1", "info", db.Name) != nil)
	})
	if !first {
		migrations()
	}
	// A bucket and a queue are what they are by name, and Cloudflare cannot rename one: a changed name is a new,
	// empty one, and the old one is left behind with what it holds. That is never done by a slip.
	envName := ""
	if len(env) == 2 {
		envName = env[1]
	}
	force := slices.Contains(args, "--new-resources")
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--new-resources" })
	if changed, err := renamed(p, envName); err != nil {
		check(err)
	} else if len(changed) > 0 && !force {
		check(fmt.Errorf("%s\nA bucket and a queue cannot be renamed: deploying makes a new, empty one and leaves the old one behind, with what it holds. Put the name back, or to start a new one on purpose:\n  go tool charter deploy %s--new-resources", strings.Join(changed, "\n"), envWord(envName)))
	}
	// How the version that is going did, the last day: what the new one is to be compared with, and the moment a
	// fault nobody saw is seen. It never stops a deploy, which may be its mending.
	if outcomes, cpu50, cpu99, err := health(p, deployedName(p, envName), time.Now().Add(-24*time.Hour), time.Now()); err == nil && len(outcomes) > 0 {
		report(deployedName(p, envName), "the last 24 hours, before this deploy", outcomes, cpu50, cpu99)
	}
	var said bytes.Buffer
	check(cloudflareTo(&said, p, append([]string{"deploy"}, env...), args))
	if first {
		migrations()
	}
	startedIn(said.String())
	// What the deploy made is written down: the Worker is bound to that database and that namespace by their ids.
	check(recordResources(p, envName))
	// Its databases keep a copy near every reader, from here on (replicas.go).
	replicated(p, envName)
	// Its secrets, where fnox has them and the Worker does not: a first deploy, or one after charter run delete.
	pushSecrets(p, envName)
	// What next: the same tests that ask the local Worker ask the deployed one. An environment's address is kept in
	// charter.toml's [urls] the first time wrangler says it, so charter test --env needs nothing typed.
	// A new app's url is a placeholder until it has been deployed (charter create writes <name>.example.workers.dev):
	// the first deploy puts the real address there, which the specs then name (charter check --fix).
	if address := deployedURL.FindString(said.String()); len(env) == 0 && address != "" && (p.URL == "" || strings.Contains(p.URL, ".example.workers.dev")) {
		check(project.SetTopURL(p.Root, address))
		p.URL = address
		say("charter.toml: url = %q. charter check --fix writes it into the specs", address)
	}
	switch {
	case len(env) == 0 && p.URL != "":
		say("deployed. To test it: go tool charter test --deployed   (%s)", p.URL)
	case len(env) == 0:
	case p.URLs[env[1]] != "":
		say("deployed. To test it: go tool charter test --env %s", env[1])
	default:
		address := deployedURL.FindString(said.String())
		if address == "" {
			say("deployed. To test it: go tool charter test --base <its address>")
			return
		}
		check(project.SetURL(p.Root, env[1], address))
		say("deployed to %s, now in charter.toml's [urls]. To test it: go tool charter test --env %s", address, env[1])
	}
}

// deployedURL is an address in what wrangler deploy prints: the Worker's on workers.dev, or its custom domain.
var deployedURL = regexp.MustCompile(`https://[a-z0-9.-]+\.[a-z]{2,}`)

// migrate applies every D1 database's migrations with wrangler (its own record of which ran).
func migrate(p *project.Project, env map[string]string, where ...string) error {
	for _, db := range p.Worker.D1 {
		if err := wrangler(p, with(env, "CI", "1"), append([]string{"d1", "migrations", "apply", db.Binding}, where...)...); err != nil {
			return err
		}
	}
	return nil
}
