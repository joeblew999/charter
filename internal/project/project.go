// Package project reads what charter knows about an app: the Worker from wrangler.jsonc, which wrangler runs and
// deploys (https://developers.cloudflare.com/workers/wrangler/configuration/), and charter's own settings from an
// optional charter.toml beside it.
//
//	# charter.toml: only what wrangler has no place for
//	health = "/healthz"                # a path that answers once the Worker is up (default "/")
//	url = "https://app.example.com"    # where it is deployed: the server the specs name, the site search engines read
//	spec = "fern/openapi.json"         # an API with a contract: charter check --fix writes its specs, charter check tests them
//	db = "sqlc"                        # api/queries.sql: charter check --fix writes its Go (api/queries.go), with sqlc
//
//	[build]
//	limit = 10000000                   # fail the build over this many bytes of Wasm, gzipped (default: none)
//
//	[sdks]                             # the clients Fern makes from the specs (charter pack)
//	name = "Notes"
//
//	[test]                             # commands run against the Worker: {base} is its URL, BASE in their environment
//	run = ["go test ./test/..."]       # charter test: against wrangler dev, or a deployed Worker (--env, --base)
//	live = ["go test ./test/..."]      # only when a deployed Worker is asked something else than run
//	[test.suites]                      # more, by name: charter test e2e [--env preview | --base https://…]
//	e2e = ["go test -C e2e ."]
//
//	[urls]                             # where each of wrangler.jsonc's environments is deployed: charter test --env preview
//	preview = "https://app-preview.example.workers.dev"
//
//	[run.tasks.seed]                   # the app's own tasks: charter run seed
//	about = "Fill the local database"
//	command = "go run ./cmd/seed {base}"
//	worker = true                      # with the Worker on wrangler dev, BASE its URL
//
//	[intl]                             # the app's languages (charter run languages; go doc …/intl/config)
//	locales = ["en", "de"]
//	catalogs = "locales"
//
//	[seo]                              # how it answers search engines (charter run search; go doc …/seo/config)
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tailscale/hujson"
)

// File is charter's own settings; Wrangler is the Worker's.
const (
	File     = "charter.toml"
	Wrangler = "wrangler.jsonc"
)

// Project is an app: its Worker (wrangler.jsonc) and charter's settings for it (charter.toml).
type Project struct {
	Health     string `toml:"health"`
	URL        string `toml:"url"`
	Spec       string `toml:"spec"`
	SpecServer string `toml:"spec_server"`
	DB         string `toml:"db"`
	Build      Build  `toml:"build"`
	SDKs       SDKs   `toml:"sdks"`
	Test       Test   `toml:"test"`
	// URLs is where each of wrangler.jsonc's environments is deployed, by its name: what charter test --env asks.
	URLs map[string]string `toml:"urls"`
	Run  Run               `toml:"run"`
	// The app's languages and how it answers search engines: intl/config and seo/config read them.
	Intl map[string]any `toml:"intl"`
	SEO  map[string]any `toml:"seo"`

	Root   string `toml:"-"` // the app's folder
	Worker Worker `toml:"-"` // wrangler.jsonc
}

// Worker is what charter reads of wrangler.jsonc.
type Worker struct {
	Name   string `json:"name"`
	Main   string `json:"main"`
	Assets struct {
		Directory string `json:"directory"`
	} `json:"assets"`
	// Vars are the Worker's variables, as wrangler dev gives them: the top-level vars.
	Vars       map[string]any             `json:"vars"`
	D1         []D1                       `json:"d1_databases"`
	RateLimits []RateLimit                `json:"ratelimits"`
	Env        map[string]json.RawMessage `json:"env"`
}

// DevVars is the file of local values wrangler dev reads beside wrangler.jsonc (KEY=value lines): a secret's
// throwaway value, or a variable's local one. It wins over vars.
const DevVars = ".dev.vars"

// Var is the Worker's variable called name as wrangler dev would give it: .dev.vars's, else wrangler.jsonc's
// top-level vars. ok is false when neither has it.
func (p *Project) Var(name string) (value string, ok bool) {
	if b, err := os.ReadFile(filepath.Join(p.Root, DevVars)); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			k, v, found := strings.Cut(strings.TrimSpace(line), "=")
			if found && !strings.HasPrefix(k, "#") && strings.TrimSpace(k) == name {
				return strings.Trim(strings.TrimSpace(v), `"'`), true
			}
		}
	}
	switch v := p.Worker.Vars[name].(type) {
	case nil:
		return "", false
	case string:
		return v, true
	default: // a number, a boolean, an object: as JSON, which is how a Worker's code would print it
		b, _ := json.Marshal(v)
		return string(b), true
	}
}

// D1 is a D1 database binding.
type D1 struct {
	Binding    string `json:"binding"`
	Name       string `json:"database_name"`
	Migrations string `json:"migrations_dir"`
}

// MigrationsDir is the folder of the database's *.sql migrations: migrations_dir, else wrangler's default.
func (d D1) MigrationsDir() string {
	if d.Migrations == "" {
		return "migrations"
	}
	return d.Migrations
}

// RateLimit is a Workers Rate Limiting binding: Limit calls per Period seconds (10 or 60) per key.
type RateLimit struct {
	Name   string `json:"name"`
	Simple struct {
		Limit  int `json:"limit"`
		Period int `json:"period"`
	} `json:"simple"`
}

// RateLimit is the rate limit bound as name.
func (p *Project) RateLimit(name string) (RateLimit, bool) {
	for _, r := range p.Worker.RateLimits {
		if r.Name == name {
			return r, true
		}
	}
	return RateLimit{}, false
}

// Build is charter build's settings.
type Build struct {
	Limit int `toml:"limit"`
}

// Run is charter run's settings: the app's own tasks, beside the built-in ones.
type Run struct {
	Tasks map[string]Task `toml:"tasks"`
}

// Task is one command of the app's own, run by name (charter run <name>): its words, run as they are (no shell).
// With Worker, the app runs on wrangler dev for as long as the command does, and BASE in its environment (and {base}
// in its words) is the Worker's URL.
type Task struct {
	Command string `toml:"command"`
	About   string `toml:"about"`
	Worker  bool   `toml:"worker"`
}

// SDKs are the clients Fern makes from the specs: charter check --fix writes fern/generators.yml from them and the spec.
type SDKs struct {
	Name      string   `toml:"name"`      // the API's name in its SDKs (Notes); lower case, the Go package and the CLI's command
	More      []string `toml:"more"`      // more than Go: typescript, python, and cli (a command-line client in Rust)
	Audiences []string `toml:"audiences"` // with typescript: an SDK each, of only what the contract tags for that audience
	Go        bool     `toml:"go"`        // the Go SDK is committed in sdk/go (charter pack -publish): check fails when the specs moved on
}

// Test is how charter test tries the Worker, on wrangler dev or deployed: commands, each its words, run as they are
// (no shell). {base} is the Worker's URL, also BASE in their environment; a command that is charter runs as this
// charter. The same commands ask a local Worker and a deployed one (Run), unless Live names others for a deployed one.
type Test struct {
	Run    []string            `toml:"run"`
	Live   []string            `toml:"live"`
	Suites map[string][]string `toml:"suites"`
}

// HealthPath is the path that answers once the Worker is up.
func (p *Project) HealthPath() string {
	if p.Health == "" {
		return "/"
	}
	return p.Health
}

// Load is the app at or above dir: the folder with a wrangler.jsonc.
func Load(dir string) (*Project, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for root := dir; ; root = filepath.Dir(root) {
		if _, err := os.Stat(filepath.Join(root, Wrangler)); err == nil {
			return read(root)
		}
		if filepath.Dir(root) == root {
			return nil, errors.New("no " + Wrangler + " at or above " + dir + ": the Worker's configuration (charter create makes an app)")
		}
	}
}

func read(root string) (*Project, error) {
	p := &Project{Root: root}
	path := filepath.Join(root, File)
	if md, err := toml.DecodeFile(path, p); err == nil {
		// [intl] and [seo] are read by their own packages.
		extra := slices.DeleteFunc(md.Undecoded(), func(k toml.Key) bool { return k[0] == "intl" || k[0] == "seo" })
		if len(extra) > 0 {
			return nil, fmt.Errorf("%s: unknown keys %v (the Worker's own settings are %s's)", path, extra, Wrangler)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	path = filepath.Join(root, Wrangler)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// JSON with comments and trailing commas, as wrangler reads it.
	if b, err = hujson.Standardize(b); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := json.Unmarshal(b, &p.Worker); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if p.Worker.Name == "" {
		return nil, fmt.Errorf("%s: no name", path)
	}
	return p, nil
}
