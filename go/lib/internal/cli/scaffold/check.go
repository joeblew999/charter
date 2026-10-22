package scaffold

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/go/lib/project"
)

// Check runs go tool charter check: every problem with charter.toml and the files charter writes into the app.
func Check(args []string) {
	fs := flag.NewFlagSet("charter check", flag.ExitOnError)
	fs.Parse(args)
	log.SetFlags(0)
	p, err := project.Load(".")
	if err != nil {
		log.Fatal("✗ ", err)
	}
	check(os.Chdir(p.Root))
	problems := Problems(p)
	for _, s := range problems {
		fmt.Println("✗", s)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
	fmt.Printf("✓ %s and charter's files in this app\n", project.File)
}

// Problems is every problem with p's app, run from its root.
func Problems(p *project.Project) []string {
	var out []string
	for _, f := range owned() {
		b, err := os.ReadFile(f.path)
		switch {
		case os.IsNotExist(err) && strings.HasPrefix(f.path, ".github/"):
			// No CI of charter's: the app runs its own.
		case err != nil:
			out = append(out, fmt.Sprintf("%s: missing: %s", f.path, f.how))
		case !bytes.Equal(b, f.content):
			out = append(out, fmt.Sprintf("%s isn't this charter's: %s", f.path, f.how))
		case f.mode&0o111 != 0:
			if info, err := os.Stat(f.path); err == nil && info.Mode()&0o111 == 0 {
				out = append(out, fmt.Sprintf("%s isn't executable: %s", f.path, f.how))
			}
		}
	}
	if b, err := os.ReadFile(".gitignore"); err != nil || !strings.Contains("\n"+string(b), "\n/build/") {
		out = append(out, ".gitignore doesn't ignore /build/: go tool charter init")
	}
	for _, m := range append([]string{p.Main}, p.Modules...) {
		if m == "" {
			continue
		}
		if _, err := os.Stat(m); err != nil {
			out = append(out, fmt.Sprintf("%s: module %s: %v", project.File, m, err))
		}
	}
	for _, d := range p.D1 {
		if d.Migrations == "" {
			continue
		}
		if files, _ := filepath.Glob(filepath.Join(d.Migrations, "*.sql")); len(files) == 0 {
			out = append(out, fmt.Sprintf("%s: [[d1]] %s: no *.sql in %s", project.File, d.Binding, d.Migrations))
		}
	}
	if date := workerdDate(); date != "" && p.CompatibilityDate > date {
		out = append(out, fmt.Sprintf("%s: compatibility_date %s is newer than the workerd on PATH (%s), which runs it locally", project.File, p.CompatibilityDate, date))
	}
	if p.CI.Build != "" || p.CI.Test != "" {
		if p.Repo == "" {
			out = append(out, fmt.Sprintf("%s: [ci] publishes each commit's build to repo, which isn't set", project.File))
		}
	}
	return out
}
