// Package scaffold is go tool charter init, tasks, skill, workflow and check: a Go app set up to run on Cloudflare
// Workers with charter, the way intl and seo set an app up: a config file, its mise tasks, an agent skill, and a check
// that says when any of them is stale.
//
//	go tool charter init [-name n] [-compat date]   charter.toml, build/ in .gitignore, and everything below
//	go tool charter tasks                           the mise tasks (.mise/tasks/charter.toml) and Workers Builds'
//	                                                launcher (.mise/tasks/charter-builds)
//	go tool charter skill                           the agent skill (.claude/skills/charter)
//	go tool charter workflow                        the CI workflow (.github/workflows/charter.yml)
//	go tool charter check                           every problem: charter.toml, and each file above as charter
//	                                                writes it now (after upgrading charter: write them again)
//
// What init leaves to the app: a main package that serves its handler with transport.Run (the Worker, and off
// Workers a plain HTTP server on $PORT), and its bindings in charter.toml.
package scaffold

import (
	_ "embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joeblew999/charter/go/lib/project"
	"github.com/joeblew999/charter/go/lib/skills"

	"github.com/joeblew999/charter/go/lib/internal/pins"
)

var (
	//go:embed tasks.toml
	tasks []byte
	//go:embed charter-builds
	launcher []byte
	//go:embed charter.yml
	workflow []byte
)

// TasksFile is where tasks writes, under the app's root: mise finds it there with no configuration (Task discovery).
const TasksFile = ".mise/tasks/charter.toml"

// file is one file charter writes into an app, under its root.
type file struct {
	path    string
	content []byte
	mode    os.FileMode
	how     string // the command that writes it
}

// owned are the files charter writes into an app, but charter.toml.
func owned() []file {
	b, err := fs.ReadFile(skills.FS, "charter/SKILL.md")
	check(err)
	return []file{
		{TasksFile, tasks, 0o644, "go tool charter tasks"},
		{".mise/tasks/charter-builds", launcher, 0o755, "go tool charter tasks"},
		{pins.ConfFile, pins.File, 0o644, "go tool charter tasks"},
		{".claude/skills/charter/SKILL.md", b, 0o644, "go tool charter skill"},
		{".github/workflows/charter.yml", workflow, 0o644, "go tool charter workflow"},
	}
}

// write writes the owned files that how writes.
func write(how string) {
	for _, f := range owned() {
		if f.how != how {
			continue
		}
		check(os.MkdirAll(filepath.Dir(f.path), 0o755))
		check(os.WriteFile(f.path, f.content, f.mode))
		check(os.Chmod(f.path, f.mode))
		log.Printf("✓ %s", f.path)
	}
}

// Tasks runs go tool charter tasks.
func Tasks(args []string) { writer("tasks", "go tool charter tasks", args) }

// Skill runs go tool charter skill.
func Skill(args []string) { writer("skill", "go tool charter skill", args) }

// Workflow runs go tool charter workflow.
func Workflow(args []string) { writer("workflow", "go tool charter workflow", args) }

func writer(name, how string, args []string) {
	fs := flag.NewFlagSet("charter "+name, flag.ExitOnError)
	fs.Parse(args)
	log.SetFlags(0)
	root()
	write(how)
}

// root moves to the app's root: charter.toml's folder, else go.mod's.
func root() {
	if p, err := project.Load("."); err == nil {
		check(os.Chdir(p.Root))
	}
}

// Init runs go tool charter init, from the app's root (its go.mod's directory).
func Init(args []string) {
	fs := flag.NewFlagSet("charter init", flag.ExitOnError)
	name := fs.String("name", "", "the Worker's name (default: the module path's last element)")
	compat := fs.String("compat", "", "the compatibility date (default: the date of the workerd on PATH, which runs it locally)")
	fs.Parse(args)
	log.SetFlags(0)
	if _, err := os.Stat("go.mod"); err != nil {
		log.Fatal("✗ run go tool charter init in the app's root, beside its go.mod")
	}
	if _, err := os.Stat(project.File); err == nil {
		log.Printf("· %s is there already: left as it is", project.File)
	} else {
		if *name == "" {
			*name = workerName(modulePath())
		}
		if *compat == "" {
			*compat = workerdDate()
		}
		if *compat == "" {
			log.Fatal("✗ no workerd on PATH to take the compatibility date from: give one, -compat 2026-09-25")
		}
		check(os.WriteFile(project.File, []byte(starter(*name, *compat)), 0o644))
		log.Printf("✓ %s: the Worker %s, compatibility date %s", project.File, *name, *compat)
	}
	check(ignore(".gitignore", "/build/"))
	for _, how := range []string{"go tool charter tasks", "go tool charter skill", "go tool charter workflow"} {
		write(how)
	}
	log.Print(`
Next:
  main.go     func main() { transport.Run(handler) }   // github.com/joeblew999/charter/go/lib/transport
  bindings    D1, Durable Objects, rate limits, vars: charter.toml (go doc github.com/joeblew999/charter/go/lib/project)
  mise run charter:build && mise run charter:serve      // or go tool charter build, serve
  mise run charter:deploy production                     // Cloudflare credentials in the environment, or fnox
  charter.toml [ci], then mise run charter:builds-configure   // CI and Workers Builds: a commit compiled once`)
}

func starter(name, compat string) string {
	return `# charter.toml: ` + name + ` on Cloudflare (github.com/joeblew999/charter; every key: go doc
# github.com/joeblew999/charter/go/lib/project). go tool charter reads it to run the Worker locally and to deploy it.
name = "` + name + `"
# The date workerd runs the Worker with locally, and Cloudflare in production: not newer than workerd's.
compatibility_date = "` + compat + `"
# A path that answers without touching a binding: a local server is up when it does.
health = "/"
# assets = "static"          # Static Assets, served before the Worker
# repo = "you/` + name + `"     # GitHub: CI publishes each commit's build there for Workers Builds
# subdomain = "you"          # the account's workers.dev subdomain, for preview URLs

# [[d1]]
# binding = "DB"
# migrations = "migrations"

[production]
vars = { APP_ENV = "production" }

[local]
vars = { APP_ENV = "local" }
`
}

// modulePath is go.mod's module path.
func modulePath() string {
	b, err := os.ReadFile("go.mod")
	check(err)
	m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(b)
	if m == nil {
		log.Fatal("✗ go.mod has no module line")
	}
	return strings.Trim(string(m[1]), `"`)
}

// workerName is a module path's last element as a Worker's name: lowercase letters, digits and hyphens.
func workerName(module string) string {
	n := strings.ToLower(path.Base(module))
	n = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(n, "-")
	return strings.Trim(n, "-")
}

// workerdDate is the date of the workerd on PATH ("workerd 2026-10-19"), "" without one.
func workerdDate() string {
	out, err := exec.Command("workerd", "--version").Output()
	if err != nil {
		return ""
	}
	return regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).FindString(string(out))
}

// ignore adds line to the .gitignore at path unless it has it.
func ignore(path, line string) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return os.WriteFile(path, append(b, []byte(fmt.Sprintf("# The Worker's build (go tool charter build)\n%s\n", line))...), 0o644)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
