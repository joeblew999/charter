//go:build !unix

package local

import (
	"errors"
	"os"
	"os/exec"
)

// becomeProcess runs bin (argv, env) in the foreground where a process cannot replace itself (Windows;
// js/wasm, where only go vet sees this): workerd shares the terminal, and its exit code is this
// command's.
func becomeProcess(bin string, argv, env []string) error {
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = env, os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if exit := (*exec.ExitError)(nil); errors.As(err, &exit) {
		os.Exit(exit.ExitCode())
	}
	return err
}
