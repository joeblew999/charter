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

// charter run delete takes the deployed Worker away: the Worker, its secrets and what it kept in its Durable
// Objects. Its data stays (the database, the bucket, the queue, the KV namespace), so go tool charter deploy brings
// the same app back, with its secrets from fnox. With --data those go too, and nothing of the app is left on
// Cloudflare.
//
// It deletes, so it is asked for twice: alone it says what there is and changes nothing; with the Worker's name it
// goes ahead.
//
//	charter run delete [env]                       what there is
//	charter run delete [env] my-worker             the Worker and its secrets
//	charter run delete [env] my-worker --data      and its database, bucket, queue and KV namespace
//
// A secret cannot be read back from Cloudflare. So while the Worker has one that fnox does not, delete stops: the
// value would be gone for good.
func deleteWorker(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	args = slices.Clone(args)
	data, lose := false, false
	if i := slices.Index(args, "--data"); i >= 0 {
		args, data = slices.Delete(args, i, i+1), true
	}
	if i := slices.Index(args, "--lose-secrets"); i >= 0 {
		args, lose = slices.Delete(args, i, i+1), true
	}
	name := p.Worker.Name
	if env != "" {
		name += "-" + env
	}
	if len(args) > 1 {
		check(errors.New("usage: charter run delete [env] [<worker>] [--data]"))
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
	resources, err := resourcesOf(p)
	check(err)
	if data {
		say("and its data:")
	} else if len(resources) > 0 {
		say("its data stays (--data deletes it too):")
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
		check(fmt.Errorf("nothing was changed. To delete it, name the Worker:\n  go tool charter run delete %s%s%s", envWord(env), name, map[bool]string{true: " --data"}[data]))
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

// resourcesOf are the resources wrangler.jsonc names: what charter deploy had wrangler make.
func resourcesOf(p *project.Project) ([]resource, error) {
	b, err := os.ReadFile(filepath.Join(p.Root, project.Wrangler))
	if err != nil {
		return nil, err
	}
	if b, err = hujson.Standardize(b); err != nil {
		return nil, err
	}
	var c struct {
		D1 []struct {
			Name string `json:"database_name"`
		} `json:"d1_databases"`
		R2 []struct {
			Name string `json:"bucket_name"`
		} `json:"r2_buckets"`
		KV []struct {
			Binding string `json:"binding"`
			ID      string `json:"id"`
		} `json:"kv_namespaces"`
		Queues struct {
			Producers []struct {
				Queue string `json:"queue"`
			} `json:"producers"`
		} `json:"queues"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	var out []resource
	for _, q := range c.Queues.Producers {
		out = append(out, resource{"queue", q.Queue, []string{"queues", "delete", q.Queue}})
	}
	for _, db := range c.D1 {
		out = append(out, resource{"database", db.Name, []string{"d1", "delete", db.Name, "--skip-confirmation"}})
	}
	for _, kv := range c.KV {
		// A namespace wrangler made for a binding with no id is called after the Worker and the binding.
		if kv.ID != "" {
			out = append(out, resource{"KV", kv.Binding, []string{"kv", "namespace", "delete", "--namespace-id", kv.ID}})
		} else {
			title := p.Worker.Name + "-" + strings.ToLower(strings.ReplaceAll(kv.Binding, "_", "-"))
			out = append(out, resource{"KV", title, []string{"kv", "namespace", "delete", title}})
		}
	}
	for _, bucket := range c.R2 {
		out = append(out, resource{"bucket", bucket.Name, []string{"r2", "bucket", "delete", bucket.Name}})
	}
	return out, nil
}
