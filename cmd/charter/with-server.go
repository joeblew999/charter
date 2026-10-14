package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

// charter with-server: start a server with these variables, wait for <url>, run the commands, stop the server and what it started. A command is a program and its arguments, not a shell line. {port} in any of them is a free port, {port2} another. A command's output is shown only if it fails; -show streams it
// Run by: mise run test:native, mise run test:workerd (the examples' mise.toml)
func init() {
	commands["with-server"] = command{"-url <url> [-env NAME=VALUE]... -start <cmd> [-show] -run <cmd>...",
		"start a server with these variables, wait for <url>, run the commands, stop the server and what it started. A command is a program and its arguments, not a shell line. {port} in any of them is a free port, {port2} another. A command's output is shown only if it fails; -show streams it", withServer}
	anywhere["with-server"] = true
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

func withServer(args []string) error {
	var url, start string
	var runs, env list
	var show bool
	flags("with-server", args, func(f *flag.FlagSet) {
		f.StringVar(&url, "url", "", "what must answer before the commands run")
		f.StringVar(&start, "start", "", "the server's command: a program and its arguments (no shell)")
		f.Var(&env, "env", "a variable for the server, NAME=VALUE (repeat)")
		f.Var(&runs, "run", "a command to run while the server is up: a program and its arguments (no shell; repeat)")
		f.BoolVar(&show, "show", false, "stream the commands' output (default: shown only when one fails)")
	})
	if url == "" || start == "" || len(runs) == 0 {
		return errors.New("with-server needs -url, -start and at least one -run")
	}
	// {port} is a free port, the same one everywhere: several checks can then run at once (two
	// worktrees, two agents) without agreeing on port numbers. {port2} is a second one, for a test
	// that listens itself (a webhook receiver the server is told about).
	ports, err := freePorts(2)
	if err != nil {
		return err
	}
	fill := strings.NewReplacer("{port}", ports[0], "{port2}", ports[1])
	url, start = fill.Replace(url), fill.Replace(start)
	for i := range env {
		env[i] = fill.Replace(env[i])
	}
	if err := settings(env); err != nil {
		return err
	}
	// The commands are words, not shell lines, so they are the same on every system. Every one is
	// read before the server starts: a mistake in the last is found now, not after the others ran.
	serve, err := words(start)
	if err != nil {
		return err
	}
	cmds := make([][]string, len(runs))
	for i := range runs {
		runs[i] = fill.Replace(runs[i])
		if cmds[i], err = words(runs[i]); err != nil {
			return err
		}
		if cmds[i][0], err = program(cmds[i][0]); err != nil {
			return err
		}
	}
	stop, err := server(serve, env, url)
	if err != nil {
		return err
	}
	defer stop()
	for i, run := range cmds {
		do := quiet
		if show {
			do = func(dir string, _ []string, name string, args ...string) error { return sh(dir, name, args...) }
		}
		if err := do(".", nil, run[0], run[1:]...); err != nil {
			stop()
			return fmt.Errorf("failed: %s", runs[i])
		}
	}
	return nil
}
