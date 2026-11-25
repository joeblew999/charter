// Package command is charter's commands for an app: a Go Worker on Cloudflare, laid out as its upstreams document.
// Each drives the tool that owns the job instead of copying it:
//
//	gsx        the components (go tool gsx generate) and the dev loop (go tool gsx dev)      https://gsxhq.dev
//	Vite       the browser's files (vite build, vite.config.ts), as gsx init sets it up
//	wrangler   the Worker: wrangler dev runs it, wrangler deploy ships it, wrangler.jsonc     https://developers.cloudflare.com/workers/wrangler/
//	           says what it is (workers-go's template)
//	Go         the Wasm (GOOS=js GOARCH=wasm), with charter's glue beside it (worker/*.mjs)
//
// Go and Node are all a machine needs: go.mod's toolchain line and package-lock.json pin the rest, on macOS, Linux
// and Windows.
package command

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
)

// load is the app of the working directory, which the command moves to the root of.
func load() *project.Project {
	log.SetFlags(0)
	p, err := project.Load(".")
	check(err)
	check(os.Chdir(p.Root))
	return p
}

func check(err error) {
	if err != nil {
		log.Fatal("✗ ", err)
	}
}

// self is this charter, which a command named charter runs as.
func self() string {
	bin, err := os.Executable()
	if err != nil {
		return "charter"
	}
	return bin
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// say prints a line as every command does.
func say(format string, args ...any) { fmt.Printf(format+"\n", args...) }

// goModule is the app's Go module (go.mod), "" without one.
func goModule(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	if m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

// npm is a program npm installed for the app (node_modules/.bin), or an error that says how to get it.
func npm(p *project.Project, program string) (string, error) {
	bin := run.Bin(program, p.Root)
	if bin == program {
		return "", fmt.Errorf("%s isn't installed here: npm install (package.json pins it)", program)
	}
	return bin, nil
}

// wrangler runs wrangler in the app's root.
func wrangler(p *project.Project, env map[string]string, args ...string) error {
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	return run.Run(p.Root, env, bin, args...)
}

// hasGSX reports whether the app has gsx components; hasVite whether Vite builds browser files for it; hasIntl whether
// it has languages (charter.toml's [intl]).
func hasGSX(p *project.Project) bool  { return exists(filepath.Join(p.Root, "gsx.toml")) }
func hasIntl(p *project.Project) bool { return p.Intl != nil }
func hasVite(p *project.Project) bool {
	m, _ := filepath.Glob(filepath.Join(p.Root, "vite.config.*"))
	return len(m) > 0
}
