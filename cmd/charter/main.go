// Command charter is what the mise tasks of a project run: every task in a project's mise.toml is
// one line, and anything that needs more than one line is a command here.
//
//	charter <command> [flags] [args]      (mise run <task> does this for you)
//	charter help
//
// It works on the project it is run in. A project is a folder with a mise.toml beside a fern/
// folder: the nearest one, from where the command is started upwards. `charter new` makes one.
//
// It only uses the standard library and the pinned tools (node, cf, fern, docker, cargo, gh).
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type command struct {
	usage string // after the name
	help  string
	run   func(args []string) error
}

var commands = map[string]command{}

// anywhere names the commands that also run outside a project: in a repo that holds projects (this
// one), or in any folder. The others need a project.
var anywhere = map[string]bool{}

// started is the directory the command was started in (main moves to the project when there is one).
var started string

const whatAProjectIs = "a project is a folder with a mise.toml beside a fern/ folder (charter new makes one)"

func main() {
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" {
		names := make([]string, 0, len(commands))
		for name := range commands {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Println("charter <command> [flags] [args]")
		fmt.Println("Commands work on the project they are run in: " + whatAProjectIs + ".")
		fmt.Println("Those marked * also run outside one.")
		for _, name := range names {
			mark := " "
			if anywhere[name] {
				mark = "*"
			}
			fmt.Printf("%s %-34s %s\n", mark, strings.TrimSpace(name+" "+commands[name].usage), commands[name].help)
		}
		return
	}
	cmd, ok := commands[os.Args[1]]
	if !ok {
		fail(fmt.Errorf("no command %q: charter help lists them", os.Args[1]))
	}
	var err error
	if started, err = os.Getwd(); err != nil {
		fail(err)
	}
	if dir, ok := project(started); ok {
		if err := os.Chdir(dir); err != nil {
			fail(err)
		}
	} else if !anywhere[os.Args[1]] {
		fail(errors.New("not in a project: " + whatAProjectIs))
	}
	if err := cmd.run(os.Args[2:]); err != nil {
		fail(err)
	}
}

func fail(err error) {
	var exit *exec.ExitError
	if !errors.As(err, &exit) { // a failed child has already said why
		fmt.Fprintln(os.Stderr, "charter:", err)
	}
	os.Exit(1)
}

// project is the project that dir is in: dir itself or the nearest folder above it that is one.
func project(dir string) (string, bool) {
	for ; ; dir = filepath.Dir(dir) {
		if isProject(dir) {
			return dir, true
		}
		if dir == filepath.Dir(dir) {
			return "", false
		}
	}
}

// isProject reports whether dir is a project: it has a mise.toml (its tasks) beside a fern/ folder
// (its specs and Fern's settings).
func isProject(dir string) bool {
	return exists(filepath.Join(dir, "mise.toml")) && exists(filepath.Join(dir, "fern"))
}

// flags parses a command's flags; what is left are its arguments.
func flags(name string, args []string, define func(*flag.FlagSet)) []string {
	set := flag.NewFlagSet(name, flag.ExitOnError)
	if define != nil {
		define(set)
	}
	set.Parse(args)
	return set.Args()
}

// sh runs a program in dir (relative to the project), with the terminal's output.
func sh(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr, cmd.Stdin = dir, os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

// quiet runs a program and shows its output only when it fails.
func quiet(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
	}
	return err
}

// output is a program's standard output, trimmed.
func output(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Stderr = dir, os.Stderr
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// env is an environment variable that the project's mise.toml sets ([env]).
func env(name string) (string, error) {
	if value := os.Getenv(name); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("%s is not set: run this through mise (mise run ...), whose mise.toml sets it", name)
}

// deployed is where the project's Worker is deployed (API_URL in its mise.toml), and the Worker's
// name: the first label of that URL's host, which is how workers.dev names a Worker's URL.
func deployed() (address *url.URL, worker string, err error) {
	value, err := env("API_URL")
	if err != nil {
		return nil, "", err
	}
	address, err = url.Parse(value)
	if err != nil || address.Host == "" {
		return nil, "", fmt.Errorf("API_URL (%q) is not a URL", value)
	}
	worker, _, _ = strings.Cut(address.Host, ".")
	return address, worker, nil
}

// The TypeScript a generated SDK is checked with, where the project's own is another (sdk-check):
// its program, which node runs.
const tscForSDKs = "node_modules/typescript-sdk/bin/tsc"
