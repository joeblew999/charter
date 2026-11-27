package command

import (
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/version"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const repository = "https://github.com/joeblew999/charter"

// Create runs charter create <dir>: a new app, copied from charter's example (the one app that uses all of charter) at
// this charter's release, under the folder's name: its Go module, its Worker, its npm package, installed, tidied and
// formatted, its specs written. Then go tool charter dev.
func Create(args []string) {
	log.SetFlags(0)
	flags := flag.NewFlagSet("charter create", flag.ExitOnError)
	module := flags.String("module", "", "the Go module's path (default: the folder's name)")
	usage := errors.New("usage: charter create <dir> [-module example.com/my-app]")
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
	src := filepath.Join(tmp, "example")
	if _, err := os.Stat(filepath.Join(src, "wrangler.jsonc")); err != nil {
		check(errors.New("this charter has no example/ to start from"))
	}
	oldModule, oldName := "github.com/joeblew999/charter/example", "charter-example"
	check(filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		to := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		// The example's secrets are the example's: where its own are kept is nothing a new app starts with (charter
		// run secrets:put writes the app's fnox.toml).
		if base := filepath.Base(rel); base == "fnox.toml" || strings.HasPrefix(base, "fnox.") && strings.HasSuffix(base, ".toml") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(b), oldModule, *module)
		text = strings.ReplaceAll(text, oldName, name)
		// The template's own address on Cloudflare is its owner's: the new app gets a place to write its own.
		text = templateURL.ReplaceAllString(text, "https://"+name+".example.workers.dev")
		switch rel {
		case "go.mod":
			// The example builds against the checkout beside it; an app, against the release.
			text = regexp.MustCompile(`(?m)^replace github\.com/joeblew999/charter => .*\n`).ReplaceAllString(text, "")
			if version.Released() {
				text = regexp.MustCompile(`(github\.com/joeblew999/charter) v\S+`).ReplaceAllString(text, "$1 "+version.String())
			}
		case "main.go":
			// The docs' links to the repository and to each page's file: the app's own repository, which a Go module's
			// path names, with the app at its top.
			text = strings.ReplaceAll(text, `"https://github.com/joeblew999/charter"`, `"https://`+*module+`"`)
			text = regexp.MustCompile(`(?m)^\s*Dir:\s*"example",\n`).ReplaceAllString(text, "")
		case "wrangler.jsonc":
			// The example's database and KV namespace are the example's: a new app's first deploy makes its own,
			// and writes their ids here.
			text = regexp.MustCompile(`,?\s*"(database_id|id)":\s*"[0-9a-f-]{32,36}"`).ReplaceAllString(text, "")
			text = strings.ReplaceAll(text, `"../../node_modules/wrangler/config-schema.json"`, `"node_modules/wrangler/config-schema.json"`)
		}
		return os.WriteFile(to, []byte(text), 0o644)
	}))
	// The example's README points into charter's repo: the app gets its own.
	readme := fmt.Sprintf("# %s\n\nA Go app on Cloudflare Workers, made with [charter](%s) from its example.\n\n```sh\nnpm install\ngo tool charter dev                              # work on it; leave it running\ngo tool charter check && go tool charter test    # before a push\ngo tool charter deploy                           # ship it\n```\n\n`go tool charter` lists every command.\n", name, repository)
	check(os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644))
	// The new module path sorts differently among the imports: written as gofmt and gsx fmt have them.
	// -e: the packages a build writes (the messages, the docs) are not there until the check below has built them, and
	// tidied again.
	// A charter that is no release (a developer's own build) copied the example as it is now: the app builds
	// against that same commit, not against whatever release the example's go.mod last named.
	if !version.Released() {
		if head, err := exec.Command("git", "-C", tmp, "rev-parse", "HEAD").Output(); err == nil {
			check(run.Run(dir, nil, "go", "get", "github.com/joeblew999/charter@"+strings.TrimSpace(string(head))))
		}
	}
	check(run.Run(dir, nil, "go", "mod", "tidy", "-e"))
	check(run.Run(dir, nil, "gofmt", "-w", "."))
	if exists(filepath.Join(dir, "gsx.toml")) {
		check(run.Run(dir, nil, "go", "tool", "gsx", "fmt", "-w", "."))
	}
	// As Vite+'s create does: what the app pins is installed, and what is written from its own name (the specs, which
	// name the app and where it is deployed) is written, so the first check passes.
	if exists(filepath.Join(dir, "package.json")) {
		check(run.Run(dir, nil, "npm", "install"))
	}
	check(run.Run(dir, nil, "go", "tool", "charter", "check", "--fix"))
	say("✓ %s: charter's example as %s (module %s)\n\nNext:\n  cd %s\n  go tool charter dev", dir, name, *module, dir)
}

// templateURL is where a template is deployed, under its owner's workers.dev subdomain.
var templateURL = regexp.MustCompile(`https://[a-z0-9-]+\.[a-z0-9-]+\.workers\.dev`)

// workerName is a folder's name as a Worker's: lower case letters, digits and hyphens.
func workerName(s string) string {
	s = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}
