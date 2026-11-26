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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/tools"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// times says how long each part of a command took, and which took much longer than is usual here (times.go).
type times struct {
	name  string
	start time.Time
	parts []string
	usual usual
	slow  []string
}

func newTimes(name string) *times { return &times{name: name, start: time.Now(), usual: readUsual()} }

func (t *times) part(name string, f func() error) {
	start := time.Now()
	// For a command run with --json (asJSON): where each part's output starts.
	if os.Getenv("CHARTER_PARTS") != "" {
		fmt.Printf("%s%s\n", partsMark, name)
		os.Stderr.Sync()
	}
	if err := f(); err != nil {
		check(fmt.Errorf("%s: %w", name, err))
	}
	d := time.Since(start)
	said := name + " " + run.Seconds(d)
	if median, slow := t.usual.took(t.name+"/"+name, d); slow {
		said += " (usually " + run.Seconds(median) + ")"
		t.slow = append(t.slow, name)
	}
	t.parts = append(t.parts, said)
}

func (t *times) done() {
	d := time.Since(t.start)
	median, slow := t.usual.took(t.name, d)
	t.usual.save()
	reported = true
	say("✓ %s took %s: %s", t.name, run.Seconds(d), strings.Join(t.parts, ", "))
	switch {
	case len(t.slow) > 0:
		say("  slower than usual here: %s", strings.Join(t.slow, ", "))
	case slow:
		say("  slower than usual here (%s)", run.Seconds(median))
	}
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
// CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID. In the environment already (CI's secrets), they are used as they
// are. Otherwise wrangler runs under fnox exec (https://fnox.jdx.dev: secrets from the keychain or a password
// manager, never a file), with the fnox charter brings (package tools): a developer keeps the two in fnox once
// (fnox set CLOUDFLARE_API_TOKEN) and types nothing more.
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
		fnox, err := credentials(p)
		if err != nil {
			return err
		}
		_, err = run.Output(p.Root, nil, fnox, append([]string{"exec", "--", bin}, words...)...)
		return err
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
		// charter's own fnox (package tools), brought the first time: the token comes from where fnox keeps it.
		fnox, err := credentials(p)
		if err != nil {
			return err
		}
		return run.Tee(copy, p.Root, nil, fnox, append([]string{"exec", "--", bin}, args...)...)
	}
	return run.Tee(copy, p.Root, nil, bin, args...)
}

// load is the app of the working directory, which the command moves to the root of.
func load() *project.Project {
	log.SetFlags(0)
	p, err := project.Load(".")
	check(err)
	check(os.Chdir(p.Root))
	localVars(p)
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

// localVars gives a checkout its .dev.vars, the file of local secrets and values wrangler dev reads, which is no one
// else's and so not in git: made from .dev.vars.example, the throwaway values the app's tests are written against.
func localVars(p *project.Project) {
	vars, example := filepath.Join(p.Root, project.DevVars), filepath.Join(p.Root, project.DevVars+".example")
	wanted, err := os.ReadFile(example)
	if err != nil {
		return
	}
	have, err := os.ReadFile(vars)
	if err != nil {
		if os.WriteFile(vars, wanted, 0o600) == nil {
			say("%s: made from %s.example (local values for wrangler dev; not in git)", project.DevVars, project.DevVars)
		}
		return
	}
	// A name the example has since gained (a pull brought a new secret) is added, with the example's throwaway value:
	// what a developer set themselves stays.
	names := func(b []byte) map[string]string {
		out := map[string]string{}
		for _, line := range strings.Split(string(b), "\n") {
			if name, _, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(name, "#") {
				out[strings.TrimSpace(name)] = line
			}
		}
		return out
	}
	mine, added := names(have), []string{}
	for _, line := range strings.Split(string(wanted), "\n") {
		if name, _, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(name, "#") && mine[strings.TrimSpace(name)] == "" {
			have = append(bytes.TrimRight(have, "\n"), ("\n" + line + "\n")...)
			added = append(added, strings.TrimSpace(name))
		}
	}
	if len(added) > 0 && os.WriteFile(vars, have, 0o600) == nil {
		say("%s: %s added from %s.example", project.DevVars, strings.Join(added, ", "), project.DevVars)
	}
}

// partsMark starts a line that says which part of a command what follows is from, when the command runs for asJSON.
const partsMark = "CHARTER_PART="

// Report is a command's result for a program to read (charter check --json): whether it passed, and each part it
// ran, in order, with what the part said. The part that failed is the last, and its output says why.
type Report struct {
	Command string       `json:"command"`
	OK      bool         `json:"ok"`
	Seconds float64      `json:"seconds"`
	Parts   []ReportPart `json:"parts"`
}

// ReportPart is one part of a command: generate, fmt, lint, …
type ReportPart struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Output string `json:"output"`
}

// asJSON runs this command again without --json, as its own program, and prints its result as one JSON document:
// whatever the command does (a part that ends the program, a tool that prints as it likes), the document is whole.
// It exits as the command did.
func asJSON(command string, args []string) {
	start := time.Now()
	cmd := exec.Command(self(), append([]string{command}, args...)...)
	cmd.Env = append(os.Environ(), "CHARTER_PARTS=1", "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	report := Report{Command: command, OK: err == nil, Seconds: time.Since(start).Seconds()}
	for _, line := range strings.SplitAfter(string(out), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), partsMark); ok && strings.HasPrefix(line, partsMark) {
			report.Parts = append(report.Parts, ReportPart{Name: name, OK: true})
			continue
		}
		if len(report.Parts) == 0 {
			report.Parts = append(report.Parts, ReportPart{Name: "start", OK: true})
		}
		report.Parts[len(report.Parts)-1].Output += line
	}
	if err != nil && len(report.Parts) > 0 {
		report.Parts[len(report.Parts)-1].OK = false
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(b))
	if err != nil {
		os.Exit(1)
	}
}

// credentials is charter's fnox, once it is known to hold Cloudflare's two: the token and the account's id. When one
// is not there it says what to do, in the three steps a developer new to this takes once; the values go to fnox's
// store (the keychain), typed at its own prompt, and are never shown.
func credentials(p *project.Project) (fnox string, err error) {
	fnox, err = tools.Fnox()
	if err != nil {
		return "", err
	}
	var missing []string
	for _, name := range []string{"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID"} {
		// Asked for, not shown: what matters is whether fnox has it.
		if exec.Command(fnox, "get", name).Run() != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return fnox, nil
	}
	steps := "Cloudflare's credentials are not in fnox yet (" + strings.Join(missing, ", ") + "). Once, on this machine:\n" +
		"  1. Make a token at https://dash.cloudflare.com/profile/api-tokens: the template \"Edit Cloudflare Workers\", with D1: Edit added.\n" +
		"  2. " + fnox + " set --global CLOUDFLARE_API_TOKEN     it asks for the token, and keeps it\n" +
		"  3. " + fnox + " set --global CLOUDFLARE_ACCOUNT_ID    the Account ID, on the right of https://dash.cloudflare.com → Workers & Pages\n" +
		"Then run this again. (In CI, the two are the environment's, and fnox is not asked.)"
	return "", errors.New(steps)
}
