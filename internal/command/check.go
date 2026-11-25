package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/command/seo"
	"github.com/joeblew999/charter/internal/dbcode"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/standard"
)

// Fmt runs charter fmt: Go and gsx files written as gofmt and gsx fmt have them. --check writes nothing, and fails
// where a file would change. gsxui's own vendored files (gsxui.json's ui folder) are gsxui's to format.
func Fmt(args []string) {
	fs := flag.NewFlagSet("charter fmt", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "write nothing; fail where a file would change")
	fs.Parse(args)
	check(format(load(), !*checkOnly))
}

// Lint runs charter lint: go.mod as go mod tidy writes it, go vet for the host and as the Worker is built
// (GOOS=js GOARCH=wasm), and structpages-lint when go.mod has it.
func Lint(args []string) {
	flag.NewFlagSet("charter lint", flag.ExitOnError).Parse(args)
	p := load()
	check(generate(p))
	check(lint(p))
}

// Check runs charter check: everything that can be said without running the app. It starts nothing, so it is safe
// beside a running charter dev. --fix writes what it can.
//
//	generate   the messages, the components' Go and the browser's files, which everything after reads
//	fmt        charter fmt --check (with --fix: charter fmt)
//	lint       charter lint (with --fix: go mod tidy)
//	contract   the committed specs, Fern's settings and the queries' Go are what the contract gives (with --fix: they
//	           are written), and each rate limit the contract declares is in wrangler.jsonc
//	languages  every language complete, plural forms, unused messages, gsxui's own text, right-to-left set up
func Check(args []string) {
	fs := flag.NewFlagSet("charter check", flag.ExitOnError)
	fix := fs.Bool("fix", false, "write what can be written: formatting, go.mod, the specs and the queries' Go")
	fs.Parse(args)
	p := load()
	t := newTimes("check")
	if p.Spec != "" && p.DB != "" && *fix {
		t.part("queries", func() error { return dbcode.Write(p.Root, p.DB, false) })
	}
	t.part("generate", func() error { return generate(p) })
	t.part("fmt", func() error { return format(p, *fix) })
	if *fix {
		t.part("tidy", func() error { return run.Run(p.Root, nil, "go", "mod", "tidy") })
	}
	t.part("lint", func() error { return lint(p) })
	if p.Spec != "" {
		t.part("contract", func() error {
			if err := spec(p, !*fix); err != nil {
				return err
			}
			return limitsDeclared(p)
		})
	}
	if hasIntl(p) {
		t.part("languages", func() error {
			if !intl.Check() {
				return errors.New("the languages have problems (above)")
			}
			return nil
		})
	}
	t.done()
}

// Test runs charter test: the app's Go tests, then the Worker itself, built and run on wrangler dev on a port and
// with state of its own: the standard test of an API with a contract, charter.toml's [test] run, and for a site
// ([seo]) the audit a search engine would pass or fail it on.
//
//	charter test                  all of that
//	charter test e2e              one of charter.toml's [test.suites], against the Worker on wrangler dev
//	charter test --base <url>     the standard test and [test] live against a Worker already deployed
//	charter test e2e --base <url> a suite against a deployed Worker
func Test(args []string) {
	suite := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		suite, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("charter test", flag.ExitOnError)
	base := fs.String("base", "", "the address of a Worker already running or deployed, instead of wrangler dev")
	env := fs.String("env", "", "an environment of "+project.Wrangler+", deployed: its address is charter.toml's [urls]")
	fs.Parse(args)
	p := load()
	if *env != "" {
		if *base != "" {
			check(errors.New("--env and --base both name the Worker to test: give one"))
		}
		if _, ok := p.Worker.Env[*env]; !ok {
			check(fmt.Errorf("%s has no environment %q", project.Wrangler, *env))
		}
		if *base = p.URLs[*env]; *base == "" {
			check(fmt.Errorf("charter.toml has no address for %s: under [urls], %s = \"https://…\" (go tool charter deploy %s prints it)", *env, *env, *env))
		}
	}
	*base = strings.TrimRight(*base, "/")
	if suite != "" {
		commands, ok := p.Test.Suites[suite]
		if !ok {
			check(fmt.Errorf("charter.toml's [test.suites] has no %s", suite))
		}
		only := &project.Project{Root: p.Root} // a suite is its own commands, without the standard test
		if *base != "" {
			say("testing %s", *base)
			check(try(only, commands, *base))
			return
		}
		check(serving(p, func(base string) error { return try(only, commands, base) }))
		return
	}
	if *base != "" {
		// A deployed Worker is asked what the local one is: the standard test and the app's own. Not go test (it
		// asks no Worker), and not the search-engine audit, which is of the site at url.
		commands := p.Test.Live
		if len(commands) == 0 {
			commands = p.Test.Run
		}
		t := newTimes("test of " + *base)
		t.part("worker", func() error { return try(p, commands, *base) })
		t.done()
		return
	}
	t := newTimes("test")
	t.part("go test", func() error {
		if err := generate(p); err != nil {
			return err
		}
		return run.Run(p.Root, nil, "go", "test", "./...")
	})
	// wrangler dev builds the Worker itself (wrangler.jsonc's build command).
	t.part("worker", func() error {
		return serving(p, func(base string) error {
			if err := try(p, p.Test.Run, base); err != nil {
				return err
			}
			if p.SEO != nil {
				seo.Main([]string{"audit", "--site", base})
			}
			return nil
		})
	})
	t.done()
}

// times says how long each part of a command took.
type times struct {
	name  string
	start time.Time
	parts []string
}

func newTimes(name string) *times { return &times{name: name, start: time.Now()} }

func (t *times) part(name string, f func() error) {
	start := time.Now()
	if err := f(); err != nil {
		check(fmt.Errorf("%s: %w", name, err))
	}
	t.parts = append(t.parts, name+" "+run.Seconds(time.Since(start)))
}

func (t *times) done() {
	say("✓ %s took %s: %s", t.name, run.Seconds(time.Since(t.start)), strings.Join(t.parts, ", "))
}

// format fails on a Go file gofmt would change or a .gsx file gsx fmt would; with write, it writes them instead.
func format(p *project.Project, write bool) error {
	dirs, err := run.Output(p.Root, nil, "go", "list", "-f", "{{.Dir}}", "./...")
	if err != nil {
		return err
	}
	var files []string
	for _, dir := range strings.Fields(dirs) {
		m, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		files = append(files, m...)
	}
	mode := "-l"
	if write {
		mode = "-w"
	}
	out, err := run.Output(p.Root, nil, "gofmt", append([]string{mode}, files...)...)
	if err != nil {
		return err
	}
	if s := strings.TrimSpace(out); s != "" {
		return fmt.Errorf("not formatted (charter fmt writes them as they should be):\n%s", s)
	}
	if !hasGSX(p) {
		return nil
	}
	// The app's own .gsx files: every folder that has some, but the one gsxui vendors its components into.
	var conf struct {
		UI string `json:"ui"`
	}
	if b, err := os.ReadFile(filepath.Join(p.Root, "gsxui.json")); err == nil {
		json.Unmarshal(b, &conf)
	}
	seen := map[string]bool{}
	var own []string
	filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(p.Root, path)
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "build" || d.Name() == "dist" || (conf.UI != "" && rel == filepath.FromSlash(conf.UI))) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".gsx") && !seen[filepath.Dir(rel)] {
			seen[filepath.Dir(rel)] = true
			own = append(own, filepath.Dir(rel))
		}
		return nil
	})
	if len(own) == 0 {
		return nil
	}
	// gsx fmt -l lists what it would change, and fails when there is any.
	cmd := exec.Command("go", append([]string{"tool", "gsx", "fmt", mode}, own...)...)
	cmd.Dir = p.Root
	if listed, err := cmd.CombinedOutput(); err != nil || (!write && len(bytes.TrimSpace(listed)) > 0) {
		return fmt.Errorf("not formatted (charter fmt writes them as they should be):\n%s", bytes.TrimSpace(listed))
	}
	return nil
}

// lint is go.mod as go mod tidy writes it, go vet for the host and for the Worker, and structpages' linter for an app
// that has it as a tool.
func lint(p *project.Project) error {
	if err := run.Run(p.Root, nil, "go", "mod", "tidy", "-diff"); err != nil {
		return fmt.Errorf("go.mod is not tidy (charter check --fix): %w", err)
	}
	return vet(p)
}

// limitsDeclared fails when the contract declares a rate limit (x-rate-limit on an operation of the spec) that
// wrangler.jsonc does not bind, or binds with another limit or period: the Worker would count against nothing.
func limitsDeclared(p *project.Project) error {
	b, err := os.ReadFile(filepath.Join(p.Root, p.Spec))
	if err != nil {
		return err
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("%s: %w", p.Spec, err)
	}
	for path, ops := range doc.Paths {
		for method, raw := range ops {
			var op struct {
				Limit *struct {
					Binding string `json:"binding"`
					Limit   int    `json:"limit"`
					Period  int    `json:"period"`
				} `json:"x-rate-limit"`
			}
			if json.Unmarshal(raw, &op) != nil || op.Limit == nil {
				continue
			}
			have, ok := p.RateLimit(op.Limit.Binding)
			want := fmt.Sprintf(`{ "name": %q, "namespace_id": "<a number of your own>", "simple": { "limit": %d, "period": %d } }`, op.Limit.Binding, op.Limit.Limit, op.Limit.Period)
			switch {
			case !ok:
				return fmt.Errorf("%s %s counts against the rate limit %s, which %s does not bind: add to its ratelimits %s", strings.ToUpper(method), path, op.Limit.Binding, project.Wrangler, want)
			case have.Simple.Limit != op.Limit.Limit || have.Simple.Period != op.Limit.Period:
				return fmt.Errorf("the rate limit %s is %d per %d s in the contract and %d per %d s in %s: make them the same", op.Limit.Binding, op.Limit.Limit, op.Limit.Period, have.Simple.Limit, have.Simple.Period, project.Wrangler)
			}
		}
	}
	return nil
}

// vet is go vet for the host and for the Worker, and structpages' linter for an app that has it as a tool.
func vet(p *project.Project) error {
	if err := run.Run(p.Root, nil, "go", "vet", "./..."); err != nil {
		return err
	}
	if err := run.Run(p.Root, map[string]string{"GOOS": "js", "GOARCH": "wasm"}, "go", "vet", "."); err != nil {
		return err
	}
	if b, _ := os.ReadFile(filepath.Join(p.Root, "go.mod")); strings.Contains(string(b), "/structpages-lint") {
		return run.Run(p.Root, nil, "go", "tool", "structpages-lint", "./...")
	}
	return nil
}

// try runs the standard test against the Worker at base, for an API with a contract, then each command with BASE and
// {base}; it fails at the first that fails.
func try(p *project.Project, commands []string, base string) error {
	if p.Spec != "" {
		if err := standard.Run(base, func(line string) { say("%s", line) }); err != nil {
			return err
		}
	}
	for _, line := range commands {
		words := strings.Fields(strings.ReplaceAll(line, "{base}", base))
		if len(words) == 0 {
			return errors.New("an empty command in charter.toml's [test]")
		}
		if words[0] == "charter" {
			words[0] = self()
		}
		if err := run.Run(p.Root, map[string]string{"BASE": base}, words[0], words[1:]...); err != nil {
			return err
		}
	}
	return nil
}
