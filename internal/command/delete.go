package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/tools"
)

// charter run delete takes the app away from Cloudflare: the Worker, its secrets, what it kept in its Durable
// Objects, and its database, bucket, queue and KV namespace. Nothing of it is left, and wrangler.jsonc no longer
// names the ids of what is gone. With --keep-data only the Worker and its secrets go: its data stays, wrangler.jsonc
// still names it by its ids, and go tool charter deploy brings the same app back, with its secrets from fnox.
//
// It deletes, so it is asked for twice: alone it says what there is and changes nothing; with the Worker's name it
// goes ahead.
//
//	charter run delete [env]                          what there is
//	charter run delete [env] my-worker                all of it
//	charter run delete [env] my-worker --keep-data    the Worker and its secrets; its data stays
//
// A secret cannot be read back from Cloudflare. So while the Worker has one that fnox does not, delete stops: the
// value would be gone for good.
func deleteWorker(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	args = slices.Clone(args)
	data, lose := true, false
	if i := slices.Index(args, "--keep-data"); i >= 0 {
		args, data = slices.Delete(args, i, i+1), false
	}
	if i := slices.Index(args, "--lose-secrets"); i >= 0 {
		args, lose = slices.Delete(args, i, i+1), true
	}
	name := p.Worker.Name
	if env != "" {
		name += "-" + env
	}
	if len(args) > 1 {
		check(errors.New("usage: charter run delete [env] [<worker>] [--keep-data]"))
	}
	secrets, deployed := onWorker(p, env)
	if !deployed {
		say("there is no deployed Worker %s", name)
	} else {
		say("the deployed Worker %s, with what its Durable Objects keep, and its secrets:", name)
		for _, s := range secrets {
			say("  %s", s)
		}
	}
	resources, err := resourcesOf(p, env)
	check(err)
	if data {
		say("and its data (--keep-data leaves it):")
	} else if len(resources) > 0 {
		say("its data stays:")
	}
	for _, r := range resources {
		say("  %-8s %s", r.kind, r.name)
	}
	// A secret only the Worker has would be gone for good.
	var only []string
	if fnox, err := tools.Fnox(); err == nil {
		for _, s := range secrets {
			if _, kept := keptSecret(p, fnox, env, s); !kept {
				only = append(only, s)
			}
		}
	}
	if len(only) > 0 && !lose {
		say("fnox does not have %s: Cloudflare never gives a secret back, so deleting the Worker loses %s.", strings.Join(only, ", "), map[bool]string{true: "it", false: "them"}[len(only) == 1])
		say("  echo <value> | go tool charter run secrets:put %s%s     keeps one first (--lose-secrets deletes regardless)", envWord(env), only[0])
	}
	if len(args) != 1 || args[0] != name {
		check(fmt.Errorf("nothing was changed. To delete it, name the Worker:\n  go tool charter run delete %s%s%s", envWord(env), name, map[bool]string{false: " --keep-data"}[data]))
	}
	if len(only) > 0 && !lose {
		check(errors.New("nothing was changed: the Worker has secrets fnox does not"))
	}
	if deployed {
		// Cloudflare keeps a Worker that a queue still hands messages to: it stops consuming first. (charter deploy
		// makes it the consumer again.)
		queues, err := consumedBy(p)
		check(err)
		for _, queue := range queues {
			if err := cloudflareQuietly(p, "queues", "consumer", "remove", queue, name); err == nil {
				say("✓ %s no longer consumes the queue %s", name, queue)
			}
		}
		words := []string{"delete", "--force"}
		if env != "" {
			words = append(words, "--env", env)
		}
		check(cloudflare(p, words, nil))
		say("✓ the Worker %s is deleted", name)
	}
	if !data {
		say("go tool charter deploy brings it back, with its data as it was and its secrets from fnox")
		return
	}
	failed := false
	for _, r := range resources {
		if err := cloudflare(p, r.remove, nil); err != nil {
			say("✗ %s %s was not deleted: %v", r.kind, r.name, err)
			failed = true
			continue
		}
		say("✓ %s %s is deleted", r.kind, r.name)
	}
	if failed {
		check(errors.New("not everything was deleted (a bucket must be empty first): what is left is named above"))
	}
	// What is gone is no longer named: the next deploy makes its own.
	declared, err := declaredResources(p, env)
	check(err)
	var forget []map[string]any
	for _, d := range declared {
		if !d.ByName && d.Has != "" {
			forget = append(forget, map[string]any{"op": "remove", "path": d.Pointer + "/" + d.Key})
		}
	}
	check(patch(p, forget))
	if len(forget) > 0 {
		say("%s no longer names the ids of what is gone", project.Wrangler)
	}
}

// consumedBy are the queues wrangler.jsonc makes the Worker a consumer of.
func consumedBy(p *project.Project) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(p.Root, project.Wrangler))
	if err != nil {
		return nil, err
	}
	if b, err = hujson.Standardize(b); err != nil {
		return nil, err
	}
	var c struct {
		Queues struct {
			Consumers []struct {
				Queue string `json:"queue"`
			} `json:"consumers"`
		} `json:"queues"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	var queues []string
	for _, consumer := range c.Queues.Consumers {
		queues = append(queues, consumer.Queue)
	}
	return queues, nil
}

// resource is something the Worker's configuration names that wrangler made: its kind, its name, and the wrangler
// command that deletes it.
type resource struct {
	kind, name string
	remove     []string
}

// resourcesOf are the resources of the app on Cloudflare, each with the wrangler command that deletes it: what
// wrangler.jsonc names, by the id the file has or the deployed Worker is bound to.
func resourcesOf(p *project.Project, env string) ([]resource, error) {
	declared, err := declaredResources(p, env)
	if err != nil {
		return nil, err
	}
	deployed, _ := deployedBindings(p, env)
	var queues, rest []resource
	for _, d := range declared {
		is := d.Has
		for _, b := range deployed {
			if b.Kind == d.Kind && b.Binding == d.Binding {
				is = b.To // what it is bound to now is what there is to delete
			}
		}
		if is == "" {
			continue // never made
		}
		switch d.Kind {
		case "queue":
			queues = append(queues, resource{"queue", is, []string{"queues", "delete", is}})
		case "d1":
			rest = append(rest, resource{"database", is, []string{"d1", "delete", is, "--skip-confirmation"}})
		case "kv_namespace":
			rest = append(rest, resource{"KV", is, []string{"kv", "namespace", "delete", "--namespace-id", is}})
		case "r2_bucket":
			rest = append(rest, resource{"bucket", is, []string{"r2", "bucket", "delete", is}})
		}
	}
	return append(queues, rest...), nil
}
