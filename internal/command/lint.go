package command

import (
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"os/exec"
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
	// go.mod as go mod tidy writes it, but for one thing: whether a module is marked // indirect. The components' Go
	// (*.x.go) is not in git, so on a clone that has not been built nothing imports gsx yet and tidy there marks it
	// indirect; that is how Dependabot's pull requests arrive, and they are right in everything else.
	diff := exec.Command("go", "mod", "tidy", "-diff")
	diff.Dir = p.Root
	out, _ := diff.CombinedOutput()
	if changed := tidyChanges(string(out)); len(changed) > 0 {
		return fmt.Errorf("go.mod is not tidy (charter check --fix):\n%s", strings.Join(changed, "\n"))
	}
	// What is installed is what package.json asks for: a lock file that carries another version (a merge, an update
	// that half applied) builds the browser's files with a tool nobody chose.
	if exists(filepath.Join(p.Root, "package.json")) {
		ls := exec.Command("npm", "ls", "--depth=0")
		ls.Dir = p.Root
		if out, err := ls.CombinedOutput(); err != nil {
			return fmt.Errorf("the npm packages installed are not what package.json asks for (npm install, then commit package-lock.json):\n%s", out)
		}
	}
	return vet(p)
}

// tidyChanges are the lines of go mod tidy -diff that change more than a module's // indirect mark.
func tidyChanges(diff string) []string {
	bare := func(line string) string {
		return strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimSpace(line[1:]), "// indirect")), " ")
	}
	removed, added := map[string]int{}, map[string]int{}
	var lines []string
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
		case strings.HasPrefix(line, "-"):
			removed[bare(line)]++
			lines = append(lines, line)
		case strings.HasPrefix(line, "+"):
			added[bare(line)]++
			lines = append(lines, line)
		}
	}
	var changed []string
	for _, line := range lines {
		if key := bare(line); removed[key] != added[key] {
			changed = append(changed, line)
		}
	}
	return changed
}

// vet is go vet for the host and for the Worker, and structpages' own linter for both: every link, id and target
// checked against the page tree, in the files that build only for a Worker too.
func vet(p *project.Project) error {
	if err := run.Run(p.Root, nil, "go", "vet", "./..."); err != nil {
		return err
	}
	// The seed is Go like the rest (db/seed.go, built only for charter run db:seed): a seed that no longer fits
	// the schema or the queries fails here, not the day someone needs it.
	if exists(filepath.Join(p.Root, "db", "seed.go")) {
		if err := run.Run(p.Root, nil, "go", "vet", "-tags", "seed", "./db/..."); err != nil {
			return err
		}
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
