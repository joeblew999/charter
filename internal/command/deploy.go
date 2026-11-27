package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/tailscale/hujson"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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
	var said bytes.Buffer
	// A Worker that is not there (a first deploy, or one after charter run delete) has nothing wrangler can find
	// its resources by: one that is there already under the name wrangler would give it is named to it, by its id.
	deploy := []string{"deploy"}
	if len(env) == 0 {
		config, err := adopted(p)
		check(err)
		if config != "" {
			defer os.Remove(filepath.Join(p.Root, config))
			deploy = append(deploy, "--config", config)
		}
	}
	err := cloudflareTo(&said, p, append(deploy, env...), args)
	if len(deploy) > 1 {
		os.Remove(filepath.Join(p.Root, deploy[2]))
	}
	check(err)
	if first {
		migrations()
	}
	// Its secrets, where fnox has them and the Worker does not: a first deploy, or one after charter run delete.
	if len(env) == 0 {
		pushSecrets(p, "")
	} else {
		pushSecrets(p, env[1])
	}
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

// adopted is wrangler.jsonc written again beside it for one deploy, with the id of each KV namespace that is there
// already and that the file leaves wrangler to make: "" when there is nothing to adopt. wrangler finds the
// resources of a Worker that is deployed by the Worker's own bindings, so it needs this only while there is no
// Worker: after charter run delete, whose data stayed.
func adopted(p *project.Project) (string, error) {
	if _, deployed := onWorker(p, ""); deployed {
		return "", nil
	}
	b, err := os.ReadFile(filepath.Join(p.Root, project.Wrangler))
	if err != nil {
		return "", err
	}
	if b, err = hujson.Standardize(b); err != nil {
		return "", err
	}
	var config map[string]any
	if err := json.Unmarshal(b, &config); err != nil {
		return "", err
	}
	namespaces, _ := config["kv_namespaces"].([]any)
	var there []struct{ ID, Title string }
	found := false
	for _, ns := range namespaces {
		ns, ok := ns.(map[string]any)
		if id, _ := ns["id"].(string); !ok || id != "" {
			continue
		}
		if there == nil {
			out, err := cloudflareOutput(p, "kv", "namespace", "list")
			start := strings.Index(out, "[")
			if err != nil || start < 0 || json.Unmarshal([]byte(out[start:]), &there) != nil {
				return "", nil // wrangler makes what it makes
			}
			there = append(there, struct{ ID, Title string }{}) // asked, whatever it has
		}
		binding, _ := ns["binding"].(string)
		title := p.Worker.Name + "-" + strings.ToLower(strings.ReplaceAll(binding, "_", "-"))
		for _, has := range there {
			if has.Title == title {
				ns["id"], found = has.ID, true
				say("the KV namespace %s is there already: the Worker gets it", title)
			}
		}
	}
	if !found {
		return "", nil
	}
	if b, err = json.MarshalIndent(config, "", "  "); err != nil {
		return "", err
	}
	const name = "wrangler.charter-deploy.json"
	return name, os.WriteFile(filepath.Join(p.Root, name), b, 0o644)
}
