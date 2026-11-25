package command

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/command/seo"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
)

// task is a built-in task of charter run.
type task struct {
	name, about string
	run         func(p *project.Project, args []string)
}

// tasks are what charter run has for every app: what is no verb of its own. An app adds its own in charter.toml's
// [run.tasks].
var tasks = []task{
	{"translate", "Claude Code translates what is missing or stale, a worker per language (charter run translate de)", func(_ *project.Project, a []string) { intl.Main(append([]string{"translate"}, a...)) }},
	{"languages", "per language: translated, inherited, stale, missing; with languages named (de fr), their catalogs checked", func(_ *project.Project, a []string) {
		if len(a) > 0 {
			intl.Main(append([]string{"check"}, a...))
		}
		intl.Main([]string{"status"})
	}},
	{"todo", "what is left to translate, with what a translator needs (--json)", func(_ *project.Project, a []string) { intl.Main(append([]string{"todo"}, a...)) }},
	{"confirm", "stale translations that are still right, kept (de, or de:home.title)", func(_ *project.Project, a []string) { intl.Main(append([]string{"confirm"}, a...)) }},
	{"export", "XLIFF for translators (--all, --out dir)", func(_ *project.Project, a []string) { intl.Main(append([]string{"export"}, a...)) }},
	{"import", "translators' XLIFF read back", func(_ *project.Project, a []string) { intl.Main(append([]string{"import"}, a...)) }},
	{"prune", "messages the source no longer has, removed from every catalog", func(_ *project.Project, a []string) { intl.Main(append([]string{"prune"}, a...)) }},
	{"audit", "the live site as a search engine reads it (--site: another address)", func(_ *project.Project, a []string) { seo.Main(append([]string{"audit"}, a...)) }},
	{"lighthouse", "Lighthouse on the live site, on Google's servers", func(_ *project.Project, a []string) { seo.Main(append([]string{"lighthouse"}, a...)) }},
	{"badges", "Lighthouse and the audit, then the README's badges and scores table", func(_ *project.Project, a []string) { seo.Main(append([]string{"badges"}, a...)) }},
	{"search", "Google Search Console: setup, sites, submit, status, todo, inspect (charter run search status)", func(_ *project.Project, a []string) {
		if len(a) == 0 {
			a = []string{"status"}
		}
		seo.Main(a)
	}},
	{"logs", "the deployed Worker's live logs, until Ctrl-C (wrangler tail; an environment first: charter run logs staging)", func(p *project.Project, a []string) {
		check(cloudflare(p, append([]string{"tail"}, envArgs(p, &a)...), a))
	}},
	{"bench", "how long the Worker takes to answer, on wrangler dev: charter run bench /  /api/hello  (-n requests each)", bench},
}

// Run runs charter run <task> [args]: a built-in task, or one of charter.toml's [run.tasks]. Alone, it lists them.
func Run(args []string) {
	p := load()
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("charter run <task> [arguments]\n\nBuilt in:")
		for _, t := range tasks {
			fmt.Printf("  %-11s %s\n", t.name, t.about)
		}
		if len(p.Run.Tasks) > 0 {
			fmt.Println("\nThis app's (charter.toml's [run.tasks]):")
			names := make([]string, 0, len(p.Run.Tasks))
			for name := range p.Run.Tasks {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf("  %-11s %s\n", name, cmpOr(p.Run.Tasks[name].About, p.Run.Tasks[name].Command))
			}
		}
		return
	}
	name, rest := args[0], args[1:]
	if own, ok := p.Run.Tasks[name]; ok {
		do := func(base string) error {
			words := strings.Fields(strings.ReplaceAll(own.Command, "{base}", base))
			if len(words) == 0 {
				return fmt.Errorf("charter.toml's [run.tasks.%s] has no command", name)
			}
			if words[0] == "charter" {
				words[0] = self()
			}
			env := map[string]string{}
			if base != "" {
				env["BASE"] = base
			}
			return run.Run(p.Root, env, words[0], append(words[1:], rest...)...)
		}
		if own.Worker {
			check(serving(p, do))
		} else {
			check(do(""))
		}
		return
	}
	for _, t := range tasks {
		if t.name == name {
			t.run(p, rest)
			return
		}
	}
	check(fmt.Errorf("no task %s: charter run lists them", name))
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// envArgs takes a leading environment name (one of wrangler.jsonc's env) off args, as wrangler's --env.
func envArgs(p *project.Project, args *[]string) []string {
	if len(*args) > 0 {
		if _, ok := p.Worker.Env[(*args)[0]]; ok {
			env := (*args)[0]
			*args = (*args)[1:]
			return []string{"--env", env}
		}
	}
	return nil
}

// cloudflare runs wrangler with words then rest, for what talks to Cloudflare. Its credentials are wrangler's:
// CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID, or wrangler login. When the environment has no token and fnox is
// installed (https://fnox.jdx.dev: secrets from the keychain, never a file), wrangler runs under fnox exec, so a
// developer who keeps them there types nothing more.
func cloudflare(p *project.Project, words, rest []string) error {
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	args := append(words, rest...)
	if os.Getenv("CLOUDFLARE_API_TOKEN") == "" {
		// A fnox that answers: a version manager's shim can be on the PATH with no fnox behind it.
		if fnox, err := exec.LookPath("fnox"); err == nil && exec.Command(fnox, "--version").Run() == nil {
			return run.Run(p.Root, nil, fnox, append([]string{"exec", "--", bin}, args...)...)
		}
	}
	return run.Run(p.Root, nil, bin, args...)
}

// bench is charter run bench: the Worker on wrangler dev, each path asked for n times, one at a time, and how long an
// answer took: the median and the slowest tenth. It is the time a request waits on this machine, for comparing one
// build with another; it is not what Cloudflare bills.
func bench(p *project.Project, args []string) {
	fs := flag.NewFlagSet("charter run bench", flag.ExitOnError)
	n := fs.Int("n", 100, "requests per path")
	fs.Parse(args)
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{p.HealthPath()}
	}
	check(serving(p, func(base string) error {
		fmt.Printf("\n%-40s %10s %10s\n", "path ("+fmt.Sprint(*n)+" requests)", "median", "p90")
		for _, path := range paths {
			took := make([]time.Duration, 0, *n)
			for range *n {
				start := time.Now()
				if err := get(base + path); err != nil {
					return err
				}
				took = append(took, time.Since(start))
			}
			slices.Sort(took)
			fmt.Printf("%-40s %8.1f ms %8.1f ms\n", path, ms(took[len(took)/2]), ms(took[len(took)*9/10]))
		}
		return nil
	}))
}

// get asks for url and reads the whole answer; a status of 500 or more is an error.
func get(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return nil
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// Exec runs charter exec <tool> [args]: one of the app's own tools, as the app pins it: an npm one (wrangler, vite,
// fern: node_modules/.bin) or a Go one (gsx, gsxui: go.mod's tool lines).
func Exec(args []string) {
	if len(args) == 0 {
		check(errors.New("usage: charter exec <tool> [arguments]   (wrangler, vite, gsx, gsxui, …)"))
	}
	p := load()
	if bin := run.Bin(args[0], p.Root); bin != args[0] {
		check(passthrough(p, bin, args[1:]...))
		return
	}
	check(passthrough(p, "go", append([]string{"tool"}, args...)...))
}

// Env runs charter env: what this machine has of what an app needs, and what is missing. Go and Node are all it must
// have; the rest is for one job each.
func Env(args []string) {
	log.SetFlags(0)
	missing := false
	show := func(name, why string, required bool, cmd string, a ...string) {
		out, err := exec.Command(cmd, a...).Output()
		line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
		switch {
		case err == nil:
			fmt.Printf("✓ %-10s %s\n", name, line)
		case required:
			missing = true
			fmt.Printf("✗ %-10s not found: %s\n", name, why)
		default:
			fmt.Printf("· %-10s not found: %s\n", name, why)
		}
	}
	show("go", "https://go.dev/dl/", true, "go", "version")
	show("node", "https://nodejs.org", true, "node", "--version")
	show("npm", "it comes with Node", true, "npm", "--version")
	show("git", "charter create copies a template with it", false, "git", "--version")
	show("docker", "only charter pack needs it (Fern generates SDKs in containers)", false, "docker", "--version")
	show("claude", "only charter run translate needs it (Claude Code)", false, "claude", "--version")
	show("fnox", "optional: Cloudflare credentials from the keychain for charter deploy", false, "fnox", "--version")
	if p, err := project.Load("."); err == nil {
		for _, tool := range []string{"wrangler", "vite"} {
			if tool == "vite" && !hasVite(p) {
				continue
			}
			if bin := run.Bin(tool, p.Root); bin != tool {
				show(tool, "", true, bin, "--version")
			} else {
				missing = true
				fmt.Printf("✗ %-10s not installed here: charter install\n", tool)
			}
		}
		if os.Getenv("CLOUDFLARE_API_TOKEN") != "" {
			fmt.Println("✓ cloudflare  CLOUDFLARE_API_TOKEN is set")
		} else {
			fmt.Println("· cloudflare  no CLOUDFLARE_API_TOKEN in the environment: charter deploy uses fnox when it is installed, else wrangler login")
		}
	}
	if missing {
		os.Exit(1)
	}
}

// generated are what a build writes, in an app: charter cache clean removes them.
var generated = []string{"build", "dist", ".wrangler", filepath.Join("web", "intl"), filepath.Join("sdk", "out")}

// Cache runs charter cache clean: everything a build wrote is removed, so the next one starts from the sources.
func Cache(args []string) {
	if len(args) != 1 || args[0] != "clean" {
		check(errors.New("usage: charter cache clean"))
	}
	p := load()
	for _, dir := range generated {
		if exists(filepath.Join(p.Root, dir)) {
			check(os.RemoveAll(filepath.Join(p.Root, dir)))
			say("removed %s", dir)
		}
	}
	n := 0
	filepath.WalkDir(p.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		name := d.Name()
		if !d.IsDir() && (strings.HasSuffix(name, ".x.go") || name == "intl_gen.go" || name == "intl-messages.mjs") {
			if os.Remove(path) == nil {
				n++
			}
		}
		if d.IsDir() && name == "intl-workerd" {
			os.RemoveAll(path)
			return filepath.SkipDir
		}
		return nil
	})
	say("removed %d generated files", n)
}

// Install runs charter install: everything the app pins, installed: npm's packages as package-lock.json has them, Go's
// modules, and the tools go.mod names.
func Install(args []string) {
	p := load()
	if exists(filepath.Join(p.Root, "package-lock.json")) {
		check(run.Run(p.Root, nil, "npm", "ci"))
	} else if exists(filepath.Join(p.Root, "package.json")) {
		check(run.Run(p.Root, nil, "npm", "install"))
	}
	check(run.Run(p.Root, nil, "go", "mod", "download"))
}

// Add runs charter add <what>: something more for the app.
//
//	charter add locale:de            a language: charter.toml's [intl], and its catalog to translate
//	charter add ui:dialog            a gsxui component (gsxui add)
//	charter add screen:language-list one of intl's screens, as gsx to own
//	charter add d1:DB                a D1 database, created on Cloudflare and bound in wrangler.jsonc
//	charter add github.com/x/y       a Go module (go get)
//	charter add leaflet              an npm package (npm install)
func Add(args []string) {
	if len(args) == 0 {
		check(errors.New("usage: charter add locale:<id> | ui:<component> | screen:<name> | d1:<binding> | <Go module> | <npm package>"))
	}
	p := load()
	for _, what := range args {
		kind, name, typed := strings.Cut(what, ":")
		switch {
		case typed && kind == "locale":
			if !hasIntl(p) {
				intl.Main([]string{"init", "-locales", "en," + name})
			} else {
				intl.Main([]string{"add-locale", name})
			}
		case typed && kind == "ui":
			check(run.Run(p.Root, nil, "go", "tool", "gsxui", "add", name))
		case typed && kind == "screen":
			intl.Main([]string{"add", name})
		case typed && kind == "d1":
			check(cloudflare(p, []string{"d1", "create", p.Worker.Name + "-" + strings.ToLower(name), "--binding", name, "--update-config"}, nil))
		case typed:
			check(fmt.Errorf("charter add %s: locale:, ui:, screen: or d1:", what))
		case modulePath.MatchString(what):
			check(run.Run(p.Root, nil, "go", "get", what))
		default:
			check(run.Run(p.Root, nil, "npm", "install", what))
		}
	}
}

// modulePath is how a Go module path looks beside an npm package name: a host with a dot, then a path.
var modulePath = regexp.MustCompile(`^[a-z0-9.-]+\.[a-z]+/`)

// Remove runs charter remove <what>: a Go module or an npm package the app no longer uses.
func Remove(args []string) {
	if len(args) == 0 {
		check(errors.New("usage: charter remove <Go module> | <npm package>"))
	}
	p := load()
	for _, what := range args {
		if modulePath.MatchString(what) {
			check(run.Run(p.Root, nil, "go", "get", what+"@none"))
			check(run.Run(p.Root, nil, "go", "mod", "tidy"))
		} else {
			check(run.Run(p.Root, nil, "npm", "uninstall", what))
		}
	}
}

// Outdated runs charter outdated: every pin with a newer version, Go's and npm's.
func Outdated(args []string) {
	p := load()
	out, err := run.Output(p.Root, nil, "go", "list", "-m", "-u", "-f", "{{if and (not .Indirect) .Update}}{{.Path}} {{.Version}} → {{.Update.Version}}{{end}}", "all")
	check(err)
	if s := strings.TrimSpace(out); s != "" {
		fmt.Println("Go modules:\n" + s)
	} else {
		fmt.Println("Go modules: every one is the latest")
	}
	if exists(filepath.Join(p.Root, "package.json")) {
		fmt.Println("\nnpm packages:")
		cmd := exec.Command("npm", "outdated") // exits 1 when something is outdated
		cmd.Dir, cmd.Stdout, cmd.Stderr = p.Root, os.Stdout, os.Stderr
		if cmd.Run() == nil {
			fmt.Println("every one is the latest its range allows")
		}
	}
}

// Update runs charter update: every pin moved to its latest: Go's direct modules and tools, npm's packages within
// the ranges package.json gives. Then charter check and charter test say whether the app still works.
func Update(args []string) {
	p := load()
	out, err := run.Output(p.Root, nil, "go", "list", "-m", "-f", "{{if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}", "all")
	check(err)
	if mods := strings.Fields(out); len(mods) > 0 {
		check(run.Run(p.Root, nil, "go", append([]string{"get", "-u"}, mods...)...))
		check(run.Run(p.Root, nil, "go", "mod", "tidy"))
	}
	if exists(filepath.Join(p.Root, "package.json")) {
		check(run.Run(p.Root, nil, "npm", "update"))
	}
	say("\nNext: charter check && charter test")
}

// Upgrade runs charter upgrade: charter itself moved to its latest, the library and the command together, then
// charter migrate for what the new one names differently.
func Upgrade(args []string) {
	p := load()
	check(run.Run(p.Root, nil, "go", "get", library+"@latest"))
	check(run.Run(p.Root, nil, "go", "get", "-tool", library+"/cmd/charter@latest"))
	check(run.Run(p.Root, nil, "go", "mod", "tidy"))
	check(run.Run(p.Root, nil, "go", "tool", "charter", "migrate"))
}

// moved are charter's import paths as older releases had them, each with what it is now.
var moved = regexp.MustCompile(`github\.com/joeblew999/(?:charter/(?:cf|api|site)/|mise-intl/|mise-seo/)([a-z]+)`)

// Migrate runs charter migrate: an app written for an older charter, moved onto this one where that is mechanical:
// the import paths (charter/site/intl, charter/cf/d1, mise-intl/intl, … are charter/intl, charter/d1 now), and the
// commands package.json's scripts name. What is left for a person, it lists.
func Migrate(args []string) {
	p := load()
	changed := 0
	check(filepath.WalkDir(p.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "build" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".go" && ext != ".gsx" && d.Name() != "gsx.toml" && d.Name() != "package.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := moved.ReplaceAllString(string(b), "github.com/joeblew999/charter/$1")
		text = strings.ReplaceAll(text, "github.com/joeblew999/charter/site\"", "github.com/joeblew999/charter/gsxapp\"")
		if d.Name() == "package.json" {
			for old, now := range map[string]string{"charter new": "charter create", "charter sdk": "charter pack", "charter intl translate": "charter run translate", "charter live-test": "charter test --base"} {
				text = strings.ReplaceAll(text, old, now)
			}
		}
		if text != string(b) {
			changed++
			rel, _ := filepath.Rel(p.Root, path)
			say("moved %s", rel)
			return os.WriteFile(path, []byte(text), 0o644)
		}
		return nil
	}))
	say("✓ %d files moved", changed)
	for _, old := range []string{"intl.toml", "seo.toml", "mise.toml"} {
		if exists(filepath.Join(p.Root, old)) {
			say("· %s is still here: its settings are charter.toml's now ([intl], [seo]); mise is not needed", old)
		}
	}
	if changed > 0 {
		say("\nNext: charter check --fix && charter test")
	}
}

// Staged runs charter staged: the files staged for a commit written as charter fmt has them and staged again, then
// charter check. charter staged install makes it the repository's pre-commit hook.
func Staged(args []string) {
	p := load()
	if len(args) == 1 && args[0] == "install" {
		top, err := run.Output(p.Root, nil, "git", "rev-parse", "--git-path", "hooks")
		check(err)
		hook := filepath.Join(p.Root, strings.TrimSpace(top), "pre-commit")
		if !filepath.IsAbs(strings.TrimSpace(top)) {
			hook = filepath.Join(p.Root, strings.TrimSpace(top), "pre-commit")
		}
		check(os.MkdirAll(filepath.Dir(hook), 0o755))
		check(os.WriteFile(hook, []byte("#!/bin/sh\n# Written by charter staged install.\nexec go tool charter staged\n"), 0o755))
		say("✓ %s runs charter staged before every commit", hook)
		return
	}
	out, err := run.Output(p.Root, nil, "git", "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	check(err)
	var files []string
	for _, f := range strings.Fields(out) {
		if strings.HasSuffix(f, ".go") || strings.HasSuffix(f, ".gsx") {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return
	}
	check(format(p, true))
	check(run.Run(p.Root, nil, "git", append([]string{"add", "--"}, files...)...))
	Check(nil)
}
