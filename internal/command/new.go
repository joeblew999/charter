package command

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joeblew999/charter/internal/version"
)

const repository = "https://github.com/joeblew999/charter"

// New runs charter new <dir> [-from example]: a new app, copied from one of charter's examples (api, site, notes) at
// this charter's release, under the folder's name: its Go module, its Worker, its npm package. Then npm install and
// go tool charter dev.
func New(args []string) {
	log.SetFlags(0)
	flags := flag.NewFlagSet("charter new", flag.ExitOnError)
	from := flags.String("from", "api", "the example to start from: api, site or notes")
	module := flags.String("module", "", "the Go module's path (default: the folder's name)")
	usage := errors.New("usage: charter new <dir> [-from api|site|notes] [-module example.com/my-app]")
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		check(usage)
	}
	dir := args[0]
	flags.Parse(args[1:])
	if flags.NArg() != 0 {
		check(usage)
	}
	name := workerName(filepath.Base(dir))
	if *module == "" {
		*module = name
	}
	if _, err := os.Stat(dir); err == nil {
		check(fmt.Errorf("%s is there already", dir))
	}
	tmp, err := os.MkdirTemp("", "charter-new-")
	check(err)
	defer os.RemoveAll(tmp)
	clone := []string{"clone", "-q", "--depth", "1"}
	if version.Released() {
		clone = append(clone, "--branch", version.String())
	}
	if out, err := exec.Command("git", append(clone, repository, tmp)...).CombinedOutput(); err != nil {
		check(fmt.Errorf("git clone %s: %v\n%s", repository, err, out))
	}
	src := filepath.Join(tmp, "examples", *from)
	if _, err := os.Stat(filepath.Join(src, "wrangler.jsonc")); err != nil {
		check(fmt.Errorf("charter has no example %q: api, site or notes", *from))
	}
	oldModule, oldName := "github.com/joeblew999/charter/examples/"+*from, "charter-"+*from
	check(filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		to := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(b), oldModule, *module)
		text = strings.ReplaceAll(text, oldName, name)
		switch rel {
		case "go.mod":
			// The example builds against the checkout beside it; an app, against the release.
			text = regexp.MustCompile(`(?m)^replace github\.com/joeblew999/charter => .*\n`).ReplaceAllString(text, "")
			if version.Released() {
				text = regexp.MustCompile(`(github\.com/joeblew999/charter) v\S+`).ReplaceAllString(text, "$1 "+version.String())
			}
		case "wrangler.jsonc":
			text = strings.ReplaceAll(text, `"../../node_modules/wrangler/config-schema.json"`, `"node_modules/wrangler/config-schema.json"`)
		}
		return os.WriteFile(to, []byte(text), 0o644)
	}))
	// The agent skill, for Claude Code and others that read .claude/skills.
	if b, err := os.ReadFile(filepath.Join(tmp, ".claude", "skills", "charter", "SKILL.md")); err == nil {
		skill := filepath.Join(dir, ".claude", "skills", "charter", "SKILL.md")
		check(os.MkdirAll(filepath.Dir(skill), 0o755))
		check(os.WriteFile(skill, b, 0o644))
	}
	say("✓ %s: the %s example as %s (module %s)\n\nNext:\n  cd %s\n  go mod tidy\n  npm install\n  go tool charter dev", dir, *from, name, *module, dir)
}

// workerName is a folder's name as a Worker's: lower case letters, digits and hyphens.
func workerName(s string) string {
	s = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}
