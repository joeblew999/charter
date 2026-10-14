package main

import (
	"errors"
	"flag"
	"os"
	"strings"
)

// charter exec: run a program with these variables set; one that an npm package of the project installed (cf, fern, tsc) is found in node_modules/.bin. -quiet: its output only if it fails. -secrets: with the secrets WORKER_SECRETS names, from fnox unless they are set already (as on GitHub)
// Run by: mise run dev (tasks/repo/tasks.toml); mise run dev (tasks/repo/tasks.toml); mise run run (tasks/repo/tasks.toml); mise run sdk:check-spec (tasks/repo/tasks.toml); mise run sdk:docs (tasks/repo/tasks.toml); mise run ts:check (mise.toml); mise run deploy, mise run lint, mise run live-test, mise run soak (the examples' mise.toml)
func init() {
	commands["exec"] = command{"[-env NAME=VALUE]... [-quiet] [-secrets] <program> [args]",
		"run a program with these variables set; one that an npm package of the project installed (cf, fern, tsc) is found in node_modules/.bin. -quiet: its output only if it fails. -secrets: with the secrets WORKER_SECRETS names, from fnox unless they are set already (as on GitHub)", execProgram}
	anywhere["exec"] = true
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
