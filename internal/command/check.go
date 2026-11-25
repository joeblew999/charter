package command

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/standard"
)

// Check runs charter check: everything before a push, needing no Cloudflare account. Each part says how long it took.
//
//	tidy       go.mod and go.sum as go mod tidy writes them
//	generate   the messages and the components' Go (charter build -only generate)
//	format     gofmt, and gsx fmt for .gsx files
//	vet        go vet, for the host and as the Worker is built (GOOS=js GOARCH=wasm); structpages-lint when go.mod has it
//	specs      the committed specs are what the contract gives (charter.toml's spec)
//	test       go test ./...
//	languages  every language complete (an charter.toml's [intl])
//	worker     the build, then on wrangler dev: the standard test of an API with a contract, and [test] run
func Check(args []string) {
	fs := flag.NewFlagSet("charter check", flag.ExitOnError)
	fs.Parse(args)
	p := load()
	var times []string
	start := time.Now()
	part := func(name string, f func() error) {
		t := time.Now()
		if err := f(); err != nil {
			check(fmt.Errorf("%s: %w", name, err))
		}
		times = append(times, name+" "+run.Seconds(time.Since(t)))
	}
	part("tidy", func() error { return run.Run(p.Root, nil, "go", "mod", "tidy", "-diff") })
	part("generate", func() error { return generate(p) })
	part("format", func() error { return format(p) })
	part("vet", func() error { return vet(p) })
	if p.Spec != "" {
		part("specs", func() error { return spec(p, true) })
	}
	part("test", func() error { return run.Run(p.Root, nil, "go", "test", "./...") })
	if hasIntl(p) {
		part("languages", func() error { intl.Main([]string{"status"}); return nil })
	}
	// wrangler dev builds the Worker itself (wrangler.jsonc's build command).
	part("worker", func() error {
		return serving(p, func(base string) error { return try(p, p.Test.Run, base) })
	})
	say("✓ check took %s: %s", run.Seconds(time.Since(start)), strings.Join(times, ", "))
}

// format fails on a Go file gofmt would change, or a .gsx file gsx fmt would.
func format(p *project.Project) error {
	dirs, err := run.Output(p.Root, nil, "go", "list", "-f", "{{.Dir}}", "./...")
	if err != nil {
		return err
	}
	var files []string
	for _, dir := range strings.Fields(dirs) {
		m, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		files = append(files, m...)
	}
	out, err := run.Output(p.Root, nil, "gofmt", append([]string{"-l"}, files...)...)
	if err != nil {
		return err
	}
	if s := strings.TrimSpace(out); s != "" {
		return fmt.Errorf("not formatted (gofmt -w writes them as they should be):\n%s", s)
	}
	if !hasGSX(p) {
		return nil
	}
	// gsx fmt -l lists what it would change, and fails when there is any.
	cmd := exec.Command("go", "tool", "gsx", "fmt", "-l", ".")
	cmd.Dir = p.Root
	if listed, err := cmd.CombinedOutput(); err != nil || len(bytes.TrimSpace(listed)) > 0 {
		return fmt.Errorf("not formatted (go tool gsx fmt -w . writes them as they should be):\n%s", bytes.TrimSpace(listed))
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

// Test runs charter test <suite> [-base url]: one of charter.toml's [test.suites], against the Worker built and run
// on wrangler dev, or with -base against one already running or deployed.
func Test(args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		check(errors.New("usage: charter test <suite> [-base https://…]   (charter.toml's [test.suites])"))
	}
	fs := flag.NewFlagSet("charter test", flag.ExitOnError)
	base := fs.String("base", "", "a Worker already running or deployed, instead of wrangler dev")
	fs.Parse(args[1:])
	p := load()
	commands, ok := p.Test.Suites[args[0]]
	if !ok {
		check(fmt.Errorf("charter.toml's [test.suites] has no %s", args[0]))
	}
	if *base != "" {
		check(try(&project.Project{Root: p.Root}, commands, strings.TrimRight(*base, "/")))
		return
	}
	check(serving(p, func(base string) error { return try(&project.Project{Root: p.Root}, commands, base) }))
}

// LiveTest runs charter live-test [-base url]: the standard test and [test] live against the deployed Worker
// (charter.toml's url).
func LiveTest(args []string) {
	fs := flag.NewFlagSet("charter live-test", flag.ExitOnError)
	base := fs.String("base", "", "the deployed Worker's address (default: charter.toml's url)")
	fs.Parse(args)
	p := load()
	if *base == "" {
		*base = p.URL
	}
	if *base == "" {
		check(errors.New("charter.toml has no url: where the Worker is deployed (or -base)"))
	}
	check(try(p, p.Test.Live, strings.TrimRight(*base, "/")))
}
