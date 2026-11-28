package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/joeblew999/charter/internal/project"
)

// A Worker's resources: its database, KV namespace, bucket and queue. What a resource is, is its id (a database, a
// KV namespace) or its name (a bucket and a queue, which Cloudflare knows by name and cannot rename). Both are
// written down in wrangler.jsonc, the one file that says what the Worker is bound to:
//
//   - charter deploy records the id of each resource the deploy made, so the Worker is bound to exactly that one
//     from then on, whatever the Worker or the resource is later called;
//   - charter deploy stops when a bucket's or a queue's name has changed, since the new name is a new, empty one;
//   - charter run delete takes all of it away, and the ids out of the file;
//   - charter run resources says what the file names and what the deployed Worker is bound to.

// bound is one binding of the deployed Worker as Cloudflare has it: its kind (d1, kv_namespace, r2_bucket, queue),
// the binding's name, and what it is bound to (an id, or for a bucket and a queue the name).
type bound struct{ Kind, Binding, To string }

// deployedRaw are the bindings of a Worker's current deployment as Cloudflare lists them, ok false when it is not
// deployed. flags say which Worker (an environment, or the Worker in front's configuration): the Go Worker's
// without them.
func deployedRaw(p *project.Project, flags ...string) (bindings []map[string]any, ok bool) {
	out, err := cloudflareOutput(p, append([]string{"deployments", "status", "--json"}, flags...)...)
	start := strings.Index(out, "{")
	if err != nil || start < 0 {
		return nil, false
	}
	var status struct {
		Versions []struct {
			ID string `json:"version_id"`
		} `json:"versions"`
	}
	if json.Unmarshal([]byte(out[start:]), &status) != nil || len(status.Versions) == 0 {
		return nil, false
	}
	out, err = cloudflareOutput(p, append([]string{"versions", "view", status.Versions[0].ID, "--json"}, flags...)...)
	if start = strings.Index(out, "{"); err != nil || start < 0 {
		return nil, false
	}
	var version struct {
		Resources struct {
			Bindings []map[string]any `json:"bindings"`
		} `json:"resources"`
	}
	if json.Unmarshal([]byte(out[start:]), &version) != nil {
		return nil, false
	}
	return version.Resources.Bindings, true
}

// deployedBindings are the bindings of the Go Worker's current deployment, ok false when it is not deployed.
func deployedBindings(p *project.Project, env string) (bindings []bound, ok bool) {
	flags := []string{}
	if env != "" {
		flags = []string{"--env", env}
	}
	raw, ok := deployedRaw(p, flags...)
	if !ok {
		return nil, false
	}
	for _, b := range raw {
		kind, _ := b["type"].(string)
		name, _ := b["name"].(string)
		for _, key := range []string{"id", "database_id", "namespace_id", "bucket_name", "queue_name"} {
			if to, _ := b[key].(string); to != "" {
				bindings = append(bindings, bound{kind, name, to})
				break
			}
		}
	}
	return bindings, true
}

// declaredResource is one resource wrangler.jsonc names: its kind as Cloudflare calls it, its binding, where in the
// file it is (a JSON pointer), the key its id or name is under, and what the file says there ("" when nothing).
type declaredResource struct {
	Kind, Binding, Pointer, Key, Has string
	// ByName: Cloudflare knows it by its name, which cannot change (a bucket, a queue).
	ByName bool
	// Label is what a person calls it.
	Label string
}

// declaredResources are the resources of wrangler.jsonc, for the Worker itself or one of its environments.
func declaredResources(p *project.Project, env string) ([]declaredResource, error) {
	b, err := os.ReadFile(filepath.Join(p.Root, project.Wrangler))
	if err != nil {
		return nil, err
	}
	if b, err = hujson.Standardize(b); err != nil {
		return nil, err
	}
	var config map[string]any
	if err := json.Unmarshal(b, &config); err != nil {
		return nil, err
	}
	at := ""
	if env != "" {
		envs, _ := config["env"].(map[string]any)
		config, _ = envs[env].(map[string]any)
		at = "/env/" + env
	}
	var out []declaredResource
	list := func(key string) []any { l, _ := config[key].([]any); return l }
	text := func(m any, key string) string { s, _ := m.(map[string]any)[key].(string); return s }
	for i, db := range list("d1_databases") {
		out = append(out, declaredResource{"d1", text(db, "binding"), fmt.Sprintf("%s/d1_databases/%d", at, i), "database_id", text(db, "database_id"), false, "database " + text(db, "database_name")})
	}
	for i, ns := range list("kv_namespaces") {
		out = append(out, declaredResource{"kv_namespace", text(ns, "binding"), fmt.Sprintf("%s/kv_namespaces/%d", at, i), "id", text(ns, "id"), false, "KV namespace " + text(ns, "binding")})
	}
	for i, bucket := range list("r2_buckets") {
		out = append(out, declaredResource{"r2_bucket", text(bucket, "binding"), fmt.Sprintf("%s/r2_buckets/%d", at, i), "bucket_name", text(bucket, "bucket_name"), true, "bucket"})
	}
	queues, _ := config["queues"].(map[string]any)
	producers, _ := queues["producers"].([]any)
	for i, q := range producers {
		out = append(out, declaredResource{"queue", text(q, "binding"), fmt.Sprintf("%s/queues/producers/%d", at, i), "queue", text(q, "queue"), true, "queue"})
	}
	return out, nil
}

// patch changes wrangler.jsonc by JSON Patch operations, leaving its comments and its layout as they are.
func patch(p *project.Project, operations []map[string]any) error {
	if len(operations) == 0 {
		return nil
	}
	path := filepath.Join(p.Root, project.Wrangler)
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	v, err := hujson.Parse(b)
	if err != nil {
		return err
	}
	ops, err := json.Marshal(operations)
	if err != nil {
		return err
	}
	if err := v.Patch(ops); err != nil {
		return err
	}
	// Written as the rest of the file is: a space after each comma and colon.
	out := addedKey.ReplaceAll(v.Pack(), []byte(`, "$1": "$2"`))
	return os.WriteFile(path, out, 0o644)
}

// addedKey is a key as a JSON Patch adds one: with no space around it.
var addedKey = regexp.MustCompile(`,"(database_id|id)":"([^"]*)"`)

// recordResources writes into wrangler.jsonc the id of each resource the deployed Worker is bound to that the file
// leaves unsaid: what wrangler made on this deploy. From then on the Worker is bound to that one, by its id.
func recordResources(p *project.Project, env string) error {
	declared, err := declaredResources(p, env)
	if err != nil {
		return err
	}
	deployed, ok := deployedBindings(p, env)
	if !ok {
		return nil
	}
	var operations []map[string]any
	for _, d := range declared {
		if d.Has != "" {
			continue
		}
		for _, b := range deployed {
			if b.Kind == d.Kind && b.Binding == d.Binding {
				operations = append(operations, map[string]any{"op": "add", "path": d.Pointer + "/" + d.Key, "value": b.To})
				say("%s: the %s is %s, now in %s: the Worker is bound to that one from here on", project.Wrangler, d.Label, b.To, project.Wrangler)
			}
		}
	}
	return patch(p, operations)
}

// renamed are the resources Cloudflare knows by name whose name in wrangler.jsonc is no longer the one the deployed
// Worker is bound to: a deploy would make a new, empty one and leave the old one, with its contents, behind.
func renamed(p *project.Project, env string) ([]string, error) {
	declared, err := declaredResources(p, env)
	if err != nil {
		return nil, err
	}
	deployed, ok := deployedBindings(p, env)
	if !ok {
		return nil, nil
	}
	var out []string
	for _, d := range declared {
		for _, b := range deployed {
			if d.ByName && b.Kind == d.Kind && b.Binding == d.Binding && d.Has != "" && b.To != d.Has {
				out = append(out, fmt.Sprintf("the %s %s is %s on Cloudflare and %s in %s", d.Label, d.Binding, b.To, d.Has, project.Wrangler))
			}
		}
	}
	return out, nil
}

// resources is charter run resources: what wrangler.jsonc names, and what the deployed Worker is bound to.
func resources(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	if len(args) > 0 {
		check(errors.New("usage: charter run resources [env]"))
	}
	declared, err := declaredResources(p, env)
	check(err)
	deployed, ok := deployedBindings(p, env)
	if !ok {
		say("the Worker is not deployed: go tool charter deploy makes what %s names", project.Wrangler)
	}
	differ := false
	for _, d := range declared {
		is := ""
		for _, b := range deployed {
			if b.Kind == d.Kind && b.Binding == d.Binding {
				is = b.To
			}
		}
		switch {
		case !ok:
			say("· %-10s %-28s %s", d.Binding, d.Label, orNone(d.Has, "made on the first deploy"))
		case is == "":
			say("✗ %-10s %-28s in %s, and the deployed Worker has no such binding: go tool charter deploy", d.Binding, d.Label, project.Wrangler)
			differ = true
		case d.Has == "":
			say("· %-10s %-28s %s on Cloudflare, not yet in %s: go tool charter deploy records it", d.Binding, d.Label, is, project.Wrangler)
		case d.Has != is:
			say("✗ %-10s %-28s %s on Cloudflare, %s in %s", d.Binding, d.Label, is, d.Has, project.Wrangler)
			differ = true
		default:
			say("✓ %-10s %-28s %s", d.Binding, d.Label, is)
		}
	}
	if differ {
		check(errors.New("the file and the deployed Worker differ"))
	}
}

func orNone(s, none string) string {
	if s == "" {
		return none
	}
	return s
}
