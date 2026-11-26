package command

import (
	"bytes"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"regexp"
	"slices"
	"strings"
)

// Deploy runs charter deploy [environment]: the D1 migrations, then wrangler deploy. An environment is one of
// wrangler.jsonc's env (production, staging). More arguments are wrangler deploy's. Credentials are wrangler's
// (cloudflare, in verbs.go, says where they come from).
func Deploy(args []string) {
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
	var said bytes.Buffer
	check(cloudflareTo(&said, p, append([]string{"deploy"}, env...), args))
	if first {
		migrations()
	}
	// What next: the same tests that ask the local Worker ask the deployed one. An environment's address is kept in
	// charter.toml's [urls] the first time wrangler says it, so charter test --env needs nothing typed.
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
