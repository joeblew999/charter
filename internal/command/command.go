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
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

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

func with(env map[string]string, key, value string) map[string]string {
	out := map[string]string{key: value}
	for k, v := range env {
		out[k] = v
	}
	return out
}

// passthrough runs a program with the terminal, until it ends.
func passthrough(p *project.Project, program string, args ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = p.Root, os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// flagsFirst is args with its flags (-x) before the rest, which Go's flag package stops at: charter pack group -if-stale
// is charter pack -if-stale group. sdk's flags take no values, so none is taken from its place.
func flagsFirst(args []string) []string {
	var flags, rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// envArgs takes a leading environment name (one of wrangler.jsonc's env) off args, as wrangler's --env.
func envArgs(p *project.Project, args *[]string) []string {
	if len(*args) > 0 {
		if _, ok := p.Worker.Env[(*args)[0]]; ok {
			env := (*args)[0]
			*args = (*args)[1:]
			return []string{"--env", env}
		}
	}
	return nil
}

// cloudflare runs wrangler with words then rest, for what talks to Cloudflare. Its credentials are wrangler's:
// CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID, or wrangler login. When the environment has no token and fnox is
// installed (https://fnox.jdx.dev: secrets from the keychain, never a file), wrangler runs under fnox exec, so a
// developer who keeps them there types nothing more.
func cloudflare(p *project.Project, words, rest []string) error {
	return cloudflareTo(nil, p, words, rest)
}

// cloudflareQuietly is cloudflare for a question: nothing is shown, and the error says whether wrangler answered.
func cloudflareQuietly(p *project.Project, words ...string) error {
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	if os.Getenv("CLOUDFLARE_API_TOKEN") == "" {
		if fnox, err := exec.LookPath("fnox"); err == nil && exec.Command(fnox, "--version").Run() == nil {
			_, err := run.Output(p.Root, nil, fnox, append([]string{"exec", "--", bin}, words...)...)
			return err
		}
	}
	_, err = run.Output(p.Root, nil, bin, words...)
	return err
}

// cloudflareTo is cloudflare, with wrangler's output also written to copy.
func cloudflareTo(copy io.Writer, p *project.Project, words, rest []string) error {
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	args := append(words, rest...)
	if os.Getenv("CLOUDFLARE_API_TOKEN") == "" {
		// A fnox that answers: a version manager's shim can be on the PATH with no fnox behind it.
		if fnox, err := exec.LookPath("fnox"); err == nil && exec.Command(fnox, "--version").Run() == nil {
			return run.Tee(copy, p.Root, nil, fnox, append([]string{"exec", "--", bin}, args...)...)
		}
	}
	return run.Tee(copy, p.Root, nil, bin, args...)
}

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
func hasGSX(p *project.Project) bool { return exists(filepath.Join(p.Root, "gsx.toml")) }

func hasIntl(p *project.Project) bool { return p.Intl != nil }

func hasVite(p *project.Project) bool {
	m, _ := filepath.Glob(filepath.Join(p.Root, "vite.config.*"))
	return len(m) > 0
}
