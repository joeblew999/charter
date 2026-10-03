package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// A task is one line that every shell mise uses takes the same way: sh on macOS and Linux, cmd.exe
// on Windows. Both run `program arguments`, keep "double quotes" together and chain with &&. They
// differ in everything else (NAME=value before a program, $NAME, $(...), ./a/path, > /dev/null,
// patterns), so what a line needs of that is here: `exec` sets variables and finds the project's
// npm programs, `lint` does the gofmt test, and `with-server` takes its commands as plain words.
// A setting of mise.toml reaches a line through mise's own template, which no shell sees.

func init() {
	commands["exec"] = command{"[-env NAME=VALUE]... [-quiet] [-secrets] <program> [args]",
		"run a program with these variables set; one that an npm package of the project installed (cf, fern, tsc) is found in node_modules/.bin. -quiet: its output only if it fails. -secrets: with the secrets WORKER_SECRETS names, from fnox unless they are set already (as on GitHub)", execProgram}
	commands["lint"] = command{"[-vet <packages>] [-wasm <packages>] <file or folder>...",
		"fail if gofmt would change one of the files (a * pattern is matched here, not by the shell), then go vet the packages (default ./...), and those of -wasm for Wasm (GOOS=js GOARCH=wasm)", lint}
	anywhere["exec"], anywhere["lint"] = true, true
}

// words splits a command into its words at spaces. Quotes, single or double, keep the spaces
// between them in one word. Nothing else is special: no variables, no pipes, no &&.
func words(line string) ([]string, error) {
	var out []string
	var word strings.Builder
	var quote rune
	in := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, in = r, true
		case r == ' ' || r == '\t':
			if in {
				out = append(out, word.String())
				word.Reset()
				in = false
			}
		default:
			word.WriteRune(r)
			in = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("a quote is not closed: %s", line)
	}
	if in {
		out = append(out, word.String())
	}
	if len(out) == 0 {
		return nil, errors.New("an empty command")
	}
	return out, nil
}

// npmBin is a program that an npm package of the project installed (package.json pins it): the
// file npm wrote for it in node_modules/.bin, which on Windows is <name>.cmd.
func npmBin(name string) string {
	shim := filepath.Join("node_modules", ".bin", npmShim(name))
	if abs, err := filepath.Abs(shim); err == nil {
		return abs
	}
	return shim
}

// npmInstalls reports whether an npm package of the project installs a program of this name:
// package-lock.json, which is committed, lists what every package puts in node_modules/.bin.
func npmInstalls(name string) bool {
	var lock struct {
		Packages map[string]struct{ Bin map[string]string }
	}
	raw, err := os.ReadFile("package-lock.json")
	if err != nil || json.Unmarshal(raw, &lock) != nil {
		return false
	}
	for path, pkg := range lock.Packages {
		if _, ok := pkg.Bin[name]; ok && strings.Count(path, "node_modules/") == 1 {
			return true
		}
	}
	return false
}

// program is where a command's first word is: the project's own install of it when an npm package
// brings one (cf, fern, tsc, vitest), otherwise the one on the path (go, node, mise). A program the
// project pins is never taken from the path: another version of it may be there.
func program(name string) (string, error) {
	if !strings.ContainsAny(name, `/\`) {
		if shim := npmBin(name); exists(shim) {
			return shim, nil
		}
		if npmInstalls(name) {
			return "", fmt.Errorf("no %s in node_modules/.bin: mise run setup installs the npm packages", name)
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("no %s on the path, and no npm package of the project installs one", name)
	}
	return path, nil
}

// settings checks -env values: each is NAME=VALUE.
func settings(env []string) error {
	for _, setting := range env {
		if name, _, ok := strings.Cut(setting, "="); !ok || name == "" {
			return fmt.Errorf("-env %s: want NAME=VALUE", setting)
		}
	}
	return nil
}

// execProgram is `NAME=VALUE program args` for every shell.
func execProgram(args []string) error {
	var env list
	var hide, secrets bool
	rest := flags("exec", args, func(f *flag.FlagSet) {
		f.Var(&env, "env", "a variable for the program, NAME=VALUE (repeat)")
		f.BoolVar(&hide, "quiet", false, "show the program's output only if it fails")
		f.BoolVar(&secrets, "secrets", false, "with the secrets WORKER_SECRETS names: from fnox (its fnox.toml) unless they are all set already")
	})
	if len(rest) == 0 {
		return errors.New("exec needs <program> [args]")
	}
	if secrets {
		for _, name := range strings.Fields(os.Getenv("WORKER_SECRETS")) {
			if os.Getenv(name) == "" {
				rest = append([]string{"fnox", "exec", "--"}, rest...)
				break
			}
		}
	}
	if err := settings(env); err != nil {
		return err
	}
	path, err := program(rest[0])
	if err != nil {
		return err
	}
	if hide {
		return quiet(".", env, path, rest[1:]...)
	}
	return become(path, rest[1:], environ(env))
}

// environ is this process's environment with these variables set: each name once, the last value
// given. (A name set twice reaches a program as its first value on some systems, its last on others.)
func environ(set []string) []string {
	last := map[string]string{}
	for _, setting := range set {
		name, _, _ := strings.Cut(setting, "=")
		last[name] = setting
	}
	var out []string
	for _, variable := range os.Environ() {
		if name, _, _ := strings.Cut(variable, "="); last[name] == "" {
			out = append(out, variable)
		}
	}
	for _, setting := range set {
		if name, _, _ := strings.Cut(setting, "="); last[name] == setting {
			out = append(out, setting)
			delete(last, name)
		}
	}
	return out
}

// lint is the Go lint of a project, or of any folder of Go code: formatted, and vetted for the
// host and, where the code also builds for Workers, for Wasm.
func lint(args []string) error {
	vet, wasm := "./...", ""
	rest := flags("lint", args, func(f *flag.FlagSet) {
		f.StringVar(&vet, "vet", vet, "the packages to go vet, comma-separated")
		f.StringVar(&wasm, "wasm", wasm, "the packages to also go vet for Wasm (GOOS=js GOARCH=wasm), comma-separated")
	})
	if len(rest) == 0 {
		return errors.New("lint needs the files and folders gofmt checks, e.g. lint *.go api cmd")
	}
	var paths []string
	for _, arg := range rest {
		if !strings.ContainsAny(arg, "*?[") {
			paths = append(paths, arg)
			continue
		}
		found, err := filepath.Glob(arg)
		if err != nil || len(found) == 0 {
			return fmt.Errorf("lint: no file matches %s", arg)
		}
		paths = append(paths, found...)
	}
	unformatted, err := output(".", "gofmt", append([]string{"-l"}, paths...)...)
	if err != nil {
		return err
	}
	if unformatted != "" {
		return fmt.Errorf("not formatted (gofmt -w writes them as they should be):\n%s", unformatted)
	}
	if err := sh(".", "go", append([]string{"vet"}, strings.Split(vet, ",")...)...); err != nil {
		return err
	}
	if wasm == "" {
		return nil
	}
	cmd := exec.Command("go", append([]string{"vet"}, strings.Split(wasm, ",")...)...)
	cmd.Env, cmd.Stdout, cmd.Stderr = append(os.Environ(), "GOOS=js", "GOARCH=wasm"), os.Stdout, os.Stderr
	return cmd.Run()
}
