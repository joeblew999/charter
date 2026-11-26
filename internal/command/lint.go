package command

import (
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
	"strings"
)

// Lint runs charter lint: go.mod as go mod tidy writes it, go vet for the host and as the Worker is built
// (GOOS=js GOARCH=wasm), and structpages-lint when go.mod has it.
func Lint(args []string) {
	flag.NewFlagSet("charter lint", flag.ExitOnError).Parse(args)
	p := load()
	check(generate(p))
	check(lint(p))
}

// lint is go.mod as go mod tidy writes it, go vet for the host and for the Worker, and structpages' linter for an app
// that has it as a tool.
func lint(p *project.Project) error {
	if err := run.Run(p.Root, nil, "go", "mod", "tidy", "-diff"); err != nil {
		return fmt.Errorf("go.mod is not tidy (charter check --fix): %w", err)
	}
	return vet(p)
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
