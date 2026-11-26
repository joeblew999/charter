package command

import (
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
	"strings"
)

// Lint runs charter lint: go.mod as go mod tidy writes it, go vet for the host and as the Worker is built
// (GOOS=js GOARCH=wasm), and structpages-lint for both.
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

// vet is go vet for the host and for the Worker, and structpages' own linter for both: every link, id and target
// checked against the page tree, in the files that build only for a Worker too.
func vet(p *project.Project) error {
	if err := run.Run(p.Root, nil, "go", "vet", "./..."); err != nil {
		return err
	}
	worker := map[string]string{"GOOS": "js", "GOARCH": "wasm"}
	if err := run.Run(p.Root, worker, "go", "vet", "."); err != nil {
		return err
	}
	if !exists(filepath.Join(p.Root, "pages")) {
		return nil
	}
	b, _ := os.ReadFile(filepath.Join(p.Root, "go.mod"))
	if !strings.Contains(string(b), linter) {
		return errors.New("the app has pages and go.mod has no structpages-lint, which checks every link, id and target against the page tree: go get -tool " + linter + " (charter check --fix does it)")
	}
	if err := run.Run(p.Root, nil, "go", "tool", "structpages-lint", "./..."); err != nil {
		return err
	}
	// The same for the Worker's build. The linter is a program of this machine, so it is found first (go tool -n
	// builds it for the host) and then run with the Worker's GOOS and GOARCH, which only say what it reads.
	bin, err := run.Output(p.Root, nil, "go", "tool", "-n", "structpages-lint")
	if err != nil {
		return err
	}
	return run.Run(p.Root, worker, strings.TrimSpace(bin), "./...")
}

// linter is structpages' linter, as go.mod's tool line names it.
const linter = "github.com/jackielii/structpages/tools/lint/cmd/structpages-lint"
