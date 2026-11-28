package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/joeblew999/charter/internal/project"
)

// A deployed app is two Workers, both written by charter deploy from the one wrangler.jsonc, which stays what
// charter dev runs:
//
//   - the Go Worker, <name>-go: the app as wrangler.jsonc has it, with every binding, its cron, its queue and its
//     secrets. It has no address of its own.
//   - the Worker in front, <name>: a few KB of JavaScript (app/worker/front.mjs) at the app's address, with the
//     browser's files. It answers a page that is the same for every reader from KV, and hands everything else to
//     the Go Worker through a service binding.
//
// Why two: a Worker that is not running where a request arrives is loaded there first, and that takes as long as
// it is large. Measured on 2026-11-28, a one-line answer from the 18 MiB Worker took 210 to 630 ms where it was not
// running, whether or not Go was started and whether or not Go was even loaded; where it was running, 25 ms.
// Cloudflare's cache for the Worker spares that only in a data centre that has the page already, and each fills
// its own. The Worker in front is small enough to be loaded in a few milliseconds anywhere, and KV is one copy for
// every data centre: so a reader anywhere is given a kept page without the Go being loaded. This is how Void's
// ISR is built (its packages/isr): the edge's cache first, KV behind it, both keyed by the deploy.
const (
	goConfig    = "wrangler.go.built.json"
	frontConfig = "wrangler.front.built.json"
	// The Worker in front's two bindings: the Go Worker, and the namespace the kept pages are in.
	frontGo, frontPages = "CHARTER_GO", "CHARTER_PAGES"
)

// deployedName is the Go Worker's name on Cloudflare, for the app itself or an environment of it: where the app
// runs, so what its health, its logs and its secrets are of.
func deployedName(p *project.Project, env string) string {
	if env == "" {
		return p.Worker.Name + "-go"
	}
	return p.Worker.Name + "-go-" + env
}

// frontName is the name of the Worker in front, which is the app's own: its address on workers.dev is of it.
func frontName(p *project.Project, env string) string {
	if env == "" {
		return p.Worker.Name
	}
	return p.Worker.Name + "-" + env
}

// declaredConfig is wrangler.jsonc as it is written, without its comments.
func declaredConfig(p *project.Project) (map[string]any, error) {
	b, err := os.ReadFile(filepath.Join(p.Root, project.Wrangler))
	if err != nil {
		return nil, err
	}
	if b, err = hujson.Standardize(b); err != nil {
		return nil, err
	}
	var c map[string]any
	return c, json.Unmarshal(b, &c)
}

// atTheAddress is what of a Worker's configuration is about its address and what is served there before it runs:
// the Worker in front's, and not the Go Worker's.
var atTheAddress = []string{"assets", "cache", "routes", "route", "workers_dev", "preview_urls"}

// writeGo writes the Go Worker's configuration: wrangler.jsonc under the Go Worker's name, without an address.
func writeGo(p *project.Project) error {
	c, err := declaredConfig(p)
	if err != nil {
		return err
	}
	c["name"] = p.Worker.Name + "-go"
	strip := func(of map[string]any) {
		for _, key := range atTheAddress {
			delete(of, key)
		}
	}
	strip(c)
	c["workers_dev"], c["preview_urls"] = false, false
	if envs, _ := c["env"].(map[string]any); envs != nil {
		for _, of := range envs {
			if of, _ := of.(map[string]any); of != nil {
				strip(of)
				delete(of, "name") // wrangler names it <name>-go-<env>
			}
		}
	}
	return writeConfig(p, goConfig, c)
}

// writeFront writes the configuration of the Worker in front, for the app itself or one environment of it.
// gone are the Durable Object classes the Worker of that name still has from when it was the whole app, which it
// gives up (they are the Go Worker's now).
func writeFront(p *project.Project, env string, gone []string) error {
	c, err := declaredConfig(p)
	if err != nil {
		return err
	}
	// An environment's own settings, where it has them.
	of := func(key string) any {
		if envs, _ := c["env"].(map[string]any); envs != nil && env != "" {
			if own, _ := envs[env].(map[string]any); own != nil && own[key] != nil {
				return own[key]
			}
		}
		return c[key]
	}
	front := map[string]any{
		"name":          frontName(p, env),
		"main":          "build/front-worker.mjs",
		"services":      []any{map[string]any{"binding": frontGo, "service": deployedName(p, env)}},
		"kv_namespaces": []any{map[string]any{"binding": frontPages}},
		// It runs at no time of its own: a cron the Worker of this name had is the Go Worker's.
		"triggers": map[string]any{"crons": []any{}},
	}
	for _, key := range append([]string{"compatibility_date", "compatibility_flags", "observability"}, atTheAddress...) {
		if v := of(key); v != nil {
			front[key] = v
		}
	}
	if len(gone) > 0 {
		migrations, _ := c["migrations"].([]any)
		front["migrations"] = append(slices.Clone(migrations), map[string]any{"tag": "charter-front", "deleted_classes": gone})
	}
	return writeConfig(p, frontConfig, front)
}

func writeConfig(p *project.Project, name string, c map[string]any) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.Root, name), append(b, '\n'), 0o644)
}

// ofTheWorker are wrangler's commands that are about a deployed Worker by its name: charter means the Go Worker
// by each, unless it says the Worker in front.
var ofTheWorker = []string{"deploy", "secret", "tail", "deployments", "versions", "delete"}

// aimed gives wrangler's words the Go Worker's configuration, for a command that is about a deployed Worker.
func aimed(p *project.Project, words []string) []string {
	if len(words) == 0 || !slices.Contains(ofTheWorker, words[0]) || slices.Contains(words, "--config") {
		return words
	}
	check(writeGo(p))
	return append(slices.Clone(words), "--config", goConfig)
}

// frontState is what the Worker of the app's name has on Cloudflare that the Worker in front does not: what is
// left from when it was the whole app. whole: it is that still (or has never been deployed as the front).
// classes are its Durable Object classes, secrets its secrets, pages the id of the namespace its kept pages are in.
func frontState(p *project.Project, env string) (whole bool, classes, secrets []string, pages string) {
	if writeFront(p, env, nil) != nil {
		return false, nil, nil, ""
	}
	bindings, ok := deployedRaw(p, "--config", frontConfig)
	if !ok {
		return false, nil, nil, ""
	}
	for _, b := range bindings {
		kind, _ := b["type"].(string)
		name, _ := b["name"].(string)
		switch {
		case name == frontGo, kind == "assets":
		case name == frontPages:
			pages, _ = b["namespace_id"].(string)
		case kind == "durable_object_namespace":
			if class, _ := b["class_name"].(string); class != "" && !slices.Contains(classes, class) {
				classes = append(classes, class)
			}
			whole = true
		case kind == "secret_text":
			secrets = append(secrets, name)
			whole = true
		default:
			whole = true
		}
	}
	return whole, classes, secrets, pages
}

// handOver makes the Worker of the app's name ready to be the Worker in front, before the Go Worker is deployed:
// a queue has one consumer, so the old one stops consuming first.
func handOver(p *project.Project, env string) {
	queues, err := consumedBy(p)
	check(err)
	for _, queue := range queues {
		if err := cloudflareQuietly(p, "queues", "consumer", "remove", queue, frontName(p, env)); err == nil {
			say("✓ %s no longer consumes the queue %s: %s does", frontName(p, env), queue, deployedName(p, env))
		}
	}
}

// cleared takes from the Worker in front the secrets it had as the whole app, once the Go Worker has them (from
// fnox). One the Go Worker does not have stays where it is: Cloudflare never gives a secret back, so it is not
// deleted while it is the only copy, and the app is without it until it is put again.
func cleared(p *project.Project, env string, secrets []string) {
	if len(secrets) == 0 {
		return
	}
	has, ok := onWorker(p, env)
	if !ok {
		return
	}
	for _, name := range secrets {
		if !slices.Contains(has, name) {
			say("✗ the Go Worker has no %s, and fnox does not have it to give: what uses it does not work until it is put again.\n  echo <value> | go tool charter run secrets:put %s%s\n  (the Worker in front still has the old one, which no one can read: it is left there)", name, envWord(env), name)
			continue
		}
		if err := cloudflareQuietly(p, "secret", "delete", name, "--config", frontConfig); err != nil {
			say("· the Worker in front still has the secret %s, which it does not use: %v", name, firstLine(err.Error()))
			continue
		}
		say("✓ the Worker in front no longer has the secret %s: the Go Worker does", name)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
