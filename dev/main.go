// Command dev is what the mise tasks run: every task in mise.toml is one line, and anything that
// needs more than one line is a command here. Run from anywhere in the repo:
//
//	go run ./dev <command> [flags] [args]      (mise run <task> does this for you)
//	go run ./dev help
//
// It only uses the standard library and the pinned tools (node, cf, fern, docker, cargo, gh).
package main

import (
	"errors"
	"flag"
	"fmt"
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

func main() {
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" {
		names := make([]string, 0, len(commands))
		for name := range commands {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Println("go run ./dev <command> [flags] [args]")
		for _, name := range names {
			fmt.Printf("  %-34s %s\n", strings.TrimSpace(name+" "+commands[name].usage), commands[name].help)
		}
		return
	}
	cmd, ok := commands[os.Args[1]]
	if !ok {
		fail(fmt.Errorf("no command %q: go run ./dev help", os.Args[1]))
	}
	if err := os.Chdir(root()); err != nil {
		fail(err)
	}
	if err := cmd.run(os.Args[2:]); err != nil {
		fail(err)
	}
}

func fail(err error) {
	var exit *exec.ExitError
	if !errors.As(err, &exit) { // a failed child has already said why
		fmt.Fprintln(os.Stderr, "dev:", err)
	}
	os.Exit(1)
}

// root is the repo: the directory with go.work, found from the working directory upwards.
func root() string {
	dir, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fail(errors.New("not inside the repo (no go.work above here)"))
		}
		dir = parent
	}
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

// sh runs a program in dir (relative to the repo), with the terminal's output.
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

// env is an environment variable that mise.toml sets ([env]).
func env(name string) (string, error) {
	if value := os.Getenv(name); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("%s is not set: run this through mise (mise run ...)", name)
}

const (
	cf   = "./node_modules/.bin/cf" // in api/, api-go/ and sdk/harness/
	fern = "./node_modules/.bin/fern"
)
