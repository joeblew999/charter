// Package project reads charter.toml, everything about a Go Worker on Cloudflare in one file at the app's root: its
// name, compatibility date and modules, its Static Assets, its bindings (D1, Durable Objects, rate limits) and what
// each environment (production, staging, previews, local workerd) gets. charter reads it to run the Worker
// locally (charter local, serve, with) and to deploy it (charter deploy production|staging|preview), and an app's
// tests read it so that a fact kept anywhere else (a limit a test counts to) can be checked against it.
//
//	# charter.toml
//	name = "my-app"                    # the Worker; staging is my-app-staging, previews are Worker Previews of it
//	compatibility_date = "2026-09-11"  # workerd's locally, Cloudflare's in production
//	main = "worker/index.mjs"          # the entry module, importing ./build/worker.mjs ("": build/worker.mjs is it)
//	modules = ["worker/room.mjs"]      # the app's other modules, beside main
//	assets = "dist/site"               # Static Assets ("": none)
//	health = "/healthz"                # a path that touches no binding: a local server is up when it answers
//	port = 8787                        # local workerd's port (PORT overrides it)
//	repo = "you/my-app"                # its GitHub repository: CI publishes each commit's build there for Workers Builds
//	subdomain = "you"                  # the account's workers.dev subdomain: a preview is <branch>-my-app.you.workers.dev
//
//	[[d1]]                             # databases my-app, my-app-staging and my-app-preview
//	binding = "DB"
//	migrations = "migrations"          # *.sql, applied in name order, each once
//
//	[[durable_objects]]
//	binding = "ROOM"
//	class = "Room"                     # exported by main
//	migration = "v1"                   # the Durable Object migration that created it (SQLite storage)
//
//	[[rate_limits]]                    # for an app without a contract: one with one declares its limits there (spec)
//	binding = "WRITES"
//	limit = 60                         # per period seconds (10 or 60), per key
//	period = 10
//	# namespace_id = 1001              # its counter, unique in the account (default: named from the Worker and binding)
//
//	spec = "fern/openapi.json"         # the contract's spec (charter spec): each x-rate-limit on an operation is a limit
//
//	[secrets]                          # values from the deploy's environment (fnox exec -- charter deploy …), never here
//	required = ["WRITE_TOKEN"]         # a deploy fails without one
//	optional = ["OIDC_ISSUER"]         # set where the environment has it
//
//	[production]
//	domain = "app.example.com"         # a Workers Custom Domain ("": workers.dev only)
//	vars = { APP_ENV = "production" }
//	[staging]                          # optional: charter deploy staging
//	vars = { APP_ENV = "staging" }
//	[preview]                          # optional: charter deploy preview <name>; {preview} is the name
//	vars = { APP_ENV = "preview {preview}" }
//	note = "Shares staging's D1."      # a sentence for the pull request's preview comment
//	[local]
//	vars = { APP_ENV = "local", WRITE_TOKEN = "local-write-token" }   # a secret's local value is a throwaway var
//
//	[build]                            # optional
//	command = "mise run build"         # what builds the Worker and its assets (default: charter build)
//
//	[ci]                               # optional: charter ci run, on GitHub (.github/workflows/charter.yml)
//	tools = ["go", "tinygo", "workerd"]  # what CI installs, of mise.toml's (default: all of them)
//	build = "mise run build ::: test"  # the build, before it is published for a preview (default: [build] command);
//	                                   # checks that may run beside it go here too
//	test = "mise run e2e"              # the checks after it (default: none)
//	release_hook = "ci/tag-modules.sh" # run when a release is tagged, before production moves (default: none)
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// File is the file's name.
const File = "charter.toml"

// Project is charter.toml.
type Project struct {
	Name               string          `toml:"name"`
	CompatibilityDate  string          `toml:"compatibility_date"`
	CompatibilityFlags []string        `toml:"compatibility_flags"`
	Main               string          `toml:"main"`
	Modules            []string        `toml:"modules"`
	Assets             string          `toml:"assets"`
	Health             string          `toml:"health"`
	Port               int             `toml:"port"`
	Repo               string          `toml:"repo"`
	Subdomain          string          `toml:"subdomain"`
	D1                 []D1            `toml:"d1"`
	DurableObjects     []DurableObject `toml:"durable_objects"`
	RateLimits         []RateLimit     `toml:"rate_limits"`
	Spec               string          `toml:"spec"`
	Secrets            Secrets         `toml:"secrets"`
	URL                string          `toml:"url"`
	SpecServer         string          `toml:"spec_server"`
	DB                 string          `toml:"db"`
	SDKs               SDKs            `toml:"sdks"`
	Go                 Go              `toml:"go"`
	Test               Test            `toml:"test"`
	Production         Environment     `toml:"production"`
	Staging            *Environment    `toml:"staging"`
	Preview            *Environment    `toml:"preview"`
	Local              Environment     `toml:"local"`
	Build              Build           `toml:"build"`
	CI                 CI              `toml:"ci"`

	Root string `toml:"-"` // the directory charter.toml is in
}

// Build is how the app builds its Worker into build/ and its Static Assets.
type Build struct {
	Command string `toml:"command"`
	// Entry is a TypeScript (or JavaScript) Worker's entry module (worker.ts): charter build bundles it into
	// build/worker.mjs (esbuild), the Worker's one module. Without it, charter build builds a Go Worker (TinyGo).
	Entry string `toml:"entry"`
}

// BuildCommand is the command that builds the Worker: [build] command, else charter build.
func (p *Project) BuildCommand() string {
	if p.Build.Command != "" {
		return p.Build.Command
	}
	return "charter build"
}

// CI is what charter ci run does on GitHub.
type CI struct {
	Tools       []string `toml:"tools"`
	Build       string   `toml:"build"`
	Test        string   `toml:"test"`
	ReleaseHook string   `toml:"release_hook"`
}

// D1 is a D1 database binding. Its database is Database (by default the Worker's name), with -staging or -preview in
// those environments.
type D1 struct {
	Binding    string `toml:"binding"`
	Migrations string `toml:"migrations"`
	Database   string `toml:"database"` // the production database's name, e.g. one cf deploy named <worker>-db
}

// DatabaseOf is the name of the database bound as d in the deploy dep: Database, else the Worker's name, with the
// environment's -staging or -preview.
func (p *Project) DatabaseOf(d D1, dep Deploy) string {
	name := d.Database
	if name == "" {
		name = p.Name
	}
	// Another Worker of the same code (also) has its own: its name in place of the Worker's.
	if dep.As != "" {
		name = strings.Replace(name, p.Name, dep.As, 1)
	}
	return name + strings.TrimPrefix(dep.Database, p.Name)
}

// DurableObject is a Durable Object namespace binding, with SQLite storage. Its class is declared in each upload's
// exports, as cf deploy declares one (live while it is declared, no migration tags), unless it names the migration
// that created it: then the deploy sends that migration where the Worker has not had it.
type DurableObject struct {
	Binding   string `toml:"binding"`
	Class     string `toml:"class"`
	Migration string `toml:"migration"`
}

// RateLimit is a Workers Rate Limiting binding: Limit calls per Period seconds per key. NamespaceID is its counter;
// none (0) is Namespace's, named from the Worker and the binding.
type RateLimit struct {
	Binding     string `toml:"binding"`
	NamespaceID int    `toml:"namespace_id"`
	Limit       int    `toml:"limit"`
	Period      int    `toml:"period"`
}

// Namespace is a rate limit's counter when charter.toml names none: a positive integer, unique in the account as
// Cloudflare needs, named from the Worker and the binding (FNV-1a), so no app numbers its own, two Workers never share
// a counter by mistake, and a Worker keeps its counter across deploys.
func Namespace(worker, binding string) int {
	h := fnv.New32a()
	h.Write([]byte(worker + "/" + binding))
	return int(h.Sum32()%1_000_000_000) + 1
}

// Limits is every rate limit the Worker binds: charter.toml's [[rate_limits]], and each the contract declares on an
// operation of its spec (x-rate-limit, the spec charter spec writes), which the app's code counts against.
func (p *Project) Limits() ([]RateLimit, error) {
	out := slices.Clone(p.RateLimits)
	if p.Spec == "" {
		return out, nil
	}
	b, err := os.ReadFile(filepath.Join(p.Root, p.Spec))
	if err != nil {
		return nil, fmt.Errorf("spec: %w (charter spec writes it)", err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("%s: %w", p.Spec, err)
	}
	for _, path := range slices.Sorted(maps.Keys(spec.Paths)) {
		for _, method := range slices.Sorted(maps.Keys(spec.Paths[path])) {
			var op struct {
				Limit *RateLimit `json:"x-rate-limit"`
			}
			if json.Unmarshal(spec.Paths[path][method], &op) != nil || op.Limit == nil {
				continue
			}
			l := *op.Limit
			if i := slices.IndexFunc(out, func(r RateLimit) bool { return r.Binding == l.Binding }); i >= 0 {
				if i < len(p.RateLimits) {
					return nil, fmt.Errorf("rate limit %s: in [[rate_limits]] and in %s (%s %s): say it in one", l.Binding, p.Spec, method, path)
				}
				if out[i].Limit != l.Limit || out[i].Period != l.Period {
					return nil, fmt.Errorf("rate limit %s: %s declares it twice, differently", l.Binding, p.Spec)
				}
				continue
			}
			out = append(out, l)
		}
	}
	return out, nil
}

// Go is a Go project's own checks, beside gofmt, go vet and go test of its root package.
type Go struct {
	Lint []string `toml:"lint"` // the folders gofmt checks beside the root's Go files (default ["api"])
	GSX  bool     `toml:"gsx"`  // its pages' *.x.go are gsx's, committed: check fails when they are stale
}

// Test is how charter check and charter live-test try the Worker: commands, each run against it. A command is its
// words, run as they are (no shell): {base} is the Worker's URL, {port1}…{port3} free ports for what the tests serve,
// and a command that is charter runs as this charter.
type Test struct {
	Run    []string          `toml:"run"`    // check: against the Worker on local workerd, after the standard test
	Native bool              `toml:"native"` // check: the same against the app run natively (go run .), first
	Live   []string          `toml:"live"`   // live-test: against the deployed Worker (url), after the standard test
	Env    map[string]string `toml:"env"`    // more environment for the Worker under test and its commands
	Serve  []Helper          `toml:"serve"`  // what the tests serve beside the Worker for its whole run
}

// Helper is a server the tests start before the Worker and stop after it (a webhook's receiver).
type Helper struct {
	Command string `toml:"command"`
	Ready   string `toml:"ready"` // a URL that answers once it is up
}

// IsProject reports whether the app is a charter project: a contract with specs and SDKs, tested the standard way,
// whose check (charter check) builds and tries it too; an app that only runs on Workers with charter is not.
func (p *Project) IsProject() bool {
	return p.SDKs.Name != "" || len(p.Test.Run) > 0 || len(p.Test.Live) > 0
}

// SDKs are the clients Fern makes from the specs: charter spec writes fern/generators.yml from them and the spec.
type SDKs struct {
	Name      string   `toml:"name"`      // the TypeScript SDK's namespace (Notes); lower case, the Go package and the CLI's command
	More      []string `toml:"more"`      // more languages: python, and cli (a command-line client in Rust, Fern's early access)
	Audiences []string `toml:"audiences"` // a TypeScript SDK each, of only what the contract tags for that audience
	Go        bool     `toml:"go"`        // the Go SDK is committed in sdk/go (charter sdk -publish): check fails when the specs moved on
	// InWorker is a folder the compiled TypeScript SDK (typescript-dist) is copied into before the Worker is built,
	// for a Worker that runs the SDK itself; made first when the specs are newer (Docker).
	InWorker string `toml:"in_worker"`
}

// Secrets are the Worker's secret_text bindings. Their values come from the environment a deploy runs in (fnox exec --
// charter deploy …), never from charter.toml; [local] vars has the throwaway values a local Worker runs with.
type Secrets struct {
	Required []string `toml:"required"` // a deploy sets each, and fails without one
	Optional []string `toml:"optional"` // set where the environment has it; without it the Worker does without
}

// Environment is what one environment gets beyond the bindings every one has.
type Environment struct {
	Domain string            `toml:"domain"`
	Vars   map[string]string `toml:"vars"`
	Note   string            `toml:"note"` // preview: a sentence for the pull request's comment
	// Also are more Workers the same code deploys as (production): each with databases of its own (its name in
	// place of the Worker's in theirs) and its own rate limit counters, as cf deploy --mode deployed one.
	Also []string `toml:"also"`
}

// As is the deploy d of another Worker, worker: the same code under its name, with its own databases (DatabaseOf).
func (p *Project) As(d Deploy, worker string) Deploy {
	d.Worker, d.As, d.Domain = worker, worker, ""
	return d
}

// The branches that deploy something other than their own preview (charter builds deploy): production
// deploys production (CI moves it there on a release, charter ci release), and main deploys staging.
const (
	ProductionBranch = "production"
	StagingBranch    = "main"
)

// BranchEnvironment is what branch deploys: "production", "staging", "preview", or "" for a release request
// (release/v*: CI releases it, nothing deploys).
func BranchEnvironment(branch string) string {
	switch {
	case branch == ProductionBranch:
		return "production"
	case branch == StagingBranch:
		return "staging"
	case strings.HasPrefix(branch, "release/"):
		return ""
	}
	return "preview"
}

// PreviewURL is where the Worker Preview called slug answers, "" without a subdomain.
func (p *Project) PreviewURL(slug string) string {
	if p.Subdomain == "" {
		return ""
	}
	return "https://" + slug + "-" + p.Name + "." + p.Subdomain + ".workers.dev"
}

// Load reads charter.toml in dir, or the nearest directory above it.
func Load(dir string) (*Project, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		path := filepath.Join(dir, File)
		if _, err := os.Stat(path); err == nil {
			p := &Project{Root: dir}
			md, err := toml.DecodeFile(path, p)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			if undecoded := md.Undecoded(); len(undecoded) > 0 {
				return nil, fmt.Errorf("%s: unknown keys %v", path, undecoded)
			}
			if err := p.check(); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("no " + File + " at or above the working directory")
		}
		dir = parent
	}
}

func (p *Project) check() error {
	var problems []string
	if p.Name == "" {
		problems = append(problems, "name is required")
	}
	if p.CompatibilityDate == "" {
		problems = append(problems, "compatibility_date is required")
	}
	for _, d := range p.D1 {
		if d.Binding == "" {
			problems = append(problems, "every [[d1]] needs a binding")
		}
	}
	tagged := 0
	for _, d := range p.DurableObjects {
		if d.Binding == "" || d.Class == "" {
			problems = append(problems, "every [[durable_objects]] needs a binding and a class")
		}
		if d.Migration != "" {
			tagged++
		}
	}
	if tagged > 0 && tagged < len(p.DurableObjects) {
		problems = append(problems, "[[durable_objects]]: each names its migration, or none does (declared in exports, as cf deploy does)")
	}
	for _, r := range p.RateLimits {
		if r.Binding == "" || r.NamespaceID < 0 || r.Limit < 1 || (r.Period != 10 && r.Period != 60) {
			problems = append(problems, fmt.Sprintf("rate limit %q: want a binding, limit ≥ 1, period 10 or 60, and a namespace_id ≥ 1 or none", r.Binding))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// LocalPort is the port local workerd listens on: PORT, else port, else 8787 (wrangler's).
func (p *Project) LocalPort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	if p.Port != 0 {
		return fmt.Sprint(p.Port)
	}
	return "8787"
}

// RateLimit is the rate limit bound as binding, false if there is none.
func (p *Project) RateLimit(binding string) (RateLimit, bool) {
	limits, err := p.Limits()
	if err != nil {
		return RateLimit{}, false
	}
	i := slices.IndexFunc(limits, func(r RateLimit) bool { return r.Binding == binding })
	if i < 0 {
		return RateLimit{}, false
	}
	return limits[i], true
}

// Deploy is one deploy of the Worker: the Worker and database names and the variables and domain of an environment.
type Deploy struct {
	Worker   string            // the script: name, or name-staging
	Preview  string            // the Worker Preview's name, for a preview
	Database string            // every D1 binding's database (DatabaseOf names one with its own)
	Vars     map[string]string // the environment's, with {preview} filled in
	Domain   string
	As       string // another Worker of the same code (Environment.Also), whose name its databases have
}

// Deploy is the deploy to env: "production", "staging" or "preview" (preview names the Worker Preview).
func (p *Project) Deploy(env, preview string) (Deploy, error) {
	var e *Environment
	d := Deploy{Worker: p.Name, Database: p.Name}
	switch env {
	case "production":
		e = &p.Production
		d.Domain = e.Domain
	case "staging":
		e = p.Staging
		d.Worker, d.Database = p.Name+"-staging", p.Name+"-staging"
	case "preview":
		if preview == "" {
			return Deploy{}, errors.New("a preview needs a name, e.g. pr-7")
		}
		e = p.Preview
		d.Preview, d.Database = preview, p.Name+"-preview"
	default:
		return Deploy{}, fmt.Errorf("%q: deploy to production, staging or preview <name>", env)
	}
	if e == nil {
		return Deploy{}, fmt.Errorf("%s has no [%s]", File, env)
	}
	d.Vars = map[string]string{}
	for k, v := range e.Vars {
		d.Vars[k] = strings.ReplaceAll(v, "{preview}", preview)
	}
	return d, nil
}

// LastMigration is the Durable Object migration the Worker is at, and the classes it created: the one named last.
func (p *Project) LastMigration() (tag string, classes []string) {
	for _, d := range p.DurableObjects {
		if d.Migration == "" {
			continue
		}
		if d.Migration != tag {
			tag, classes = d.Migration, nil
		}
		classes = append(classes, d.Class)
	}
	return tag, classes
}
